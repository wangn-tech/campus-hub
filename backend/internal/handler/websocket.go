package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/wangn-tech/campus-hub/internal/config"
	"github.com/wangn-tech/campus-hub/internal/middleware"
	"github.com/wangn-tech/campus-hub/internal/model"
	"github.com/wangn-tech/campus-hub/internal/realtime"
	"github.com/wangn-tech/campus-hub/internal/service"
	"go.uber.org/zap"
)

// WebSocketHandler serves the realtime endpoint from the WebSocket protocol
// design document: the connection authenticates in band, then exchanges chat
// frames over the shared envelope.
type WebSocketHandler struct {
	chats  *service.ChatService
	users  *service.UserService
	hub    *realtime.Hub
	auth   middleware.Authenticator
	config config.WebSocketConfig
	logger *zap.Logger

	upgrader websocket.Upgrader
}

func NewWebSocketHandler(chats *service.ChatService, users *service.UserService, hub *realtime.Hub, auth middleware.Authenticator, cfg config.WebSocketConfig, logger *zap.Logger) *WebSocketHandler {
	return &WebSocketHandler{
		chats:  chats,
		users:  users,
		hub:    hub,
		auth:   auth,
		config: cfg,
		logger: logger,
		upgrader: websocket.Upgrader{
			ReadBufferSize:   cfg.ReadBufferSize,
			WriteBufferSize:  cfg.WriteBufferSize,
			HandshakeTimeout: cfg.HandshakeTimeout,
			CheckOrigin:      originChecker(cfg.AllowedOrigins),
		},
	}
}

// Handle upgrades the request and serves the connection until it closes.
func (h *WebSocketHandler) Handle(c *gin.Context) {
	socket, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		// Upgrade already wrote an HTTP error response.
		return
	}
	conn := realtime.NewConnection(socket)
	go conn.WriteLoop()
	defer conn.Close()
	conn.SetReadLimit(h.config.MaxMessageSize)

	ctx := c.Request.Context()
	user, ok := h.authenticate(ctx, conn)
	if !ok {
		return
	}
	h.hub.Register(ctx, conn)
	defer h.hub.Unregister(context.WithoutCancel(ctx), conn)

	h.subscribeRooms(ctx, conn, user)
	h.sendAuthSuccess(conn, user)
	h.logger.Info("websocket connected", zap.String("user_id", user.UUID), zap.String("connection_id", conn.ID))
	defer h.logger.Info("websocket disconnected", zap.String("user_id", user.UUID), zap.String("connection_id", conn.ID))

	readTimeout := 2 * h.config.HeartbeatInterval
	err = conn.ReadPayloads(readTimeout, func(payload []byte) error {
		return h.handleFrame(ctx, conn, user, payload)
	})
	if err != nil && !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
		h.logger.Debug("websocket read loop ended", zap.String("user_id", user.UUID), zap.Error(err))
	}
}

// authenticate consumes the first frame, which must be an `auth` event carrying
// a valid access token, within the configured auth timeout.
func (h *WebSocketHandler) authenticate(ctx context.Context, conn *realtime.Connection) (*model.User, bool) {
	_ = conn.SetReadDeadline(time.Now().Add(h.config.AuthTimeout))
	_, payload, err := conn.ReadMessage()
	if err != nil {
		return nil, false
	}
	var envelope realtime.Envelope
	if err := json.Unmarshal(payload, &envelope); err != nil || envelope.Type != realtime.EventAuth {
		h.authFailed(conn, 101400, "the first frame must be an auth event")
		return nil, false
	}
	var data struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(envelope.Data, &data); err != nil || strings.TrimSpace(data.Token) == "" {
		h.authFailed(conn, 101400, "auth token is required")
		return nil, false
	}
	userUUID, err := h.auth.Authenticate(ctx, data.Token)
	if err != nil {
		h.authFailed(conn, 101401, "invalid access token")
		return nil, false
	}
	user, err := h.users.Current(ctx, userUUID)
	if err != nil {
		h.authFailed(conn, 101401, "invalid access token")
		return nil, false
	}
	conn.UserUUID = user.UUID
	return user, true
}

// subscribeRooms restores the room subscriptions of the authenticated user so
// reconnecting clients immediately receive their group traffic.
func (h *WebSocketHandler) subscribeRooms(ctx context.Context, conn *realtime.Connection, user *model.User) {
	groups, err := h.chats.MyGroups(ctx, user)
	if err != nil {
		h.logger.Warn("load websocket rooms", zap.String("user_id", user.UUID), zap.Error(err))
		return
	}
	roomUUIDs := make([]string, 0, len(groups))
	for _, group := range groups {
		roomUUIDs = append(roomUUIDs, group.ID)
	}
	h.hub.JoinGroups(roomUUIDs, conn)
}

func (h *WebSocketHandler) sendAuthSuccess(conn *realtime.Connection, user *model.User) {
	h.send(conn, realtime.EventAuthSuccess, user.UUID, map[string]any{"user_id": user.UUID})
}

func (h *WebSocketHandler) authFailed(conn *realtime.Connection, code int, message string) {
	h.send(conn, realtime.EventAuthFailed, uuid.NewString(), map[string]any{"code": code, "message": message})
}

// handleFrame dispatches one client event. Errors are reported back as `error`
// frames so one rejected event does not drop the connection.
func (h *WebSocketHandler) handleFrame(ctx context.Context, conn *realtime.Connection, user *model.User, payload []byte) error {
	var envelope realtime.Envelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		h.sendError(conn, 104400, "malformed event envelope")
		return nil
	}
	switch envelope.Type {
	case realtime.EventPing:
		h.send(conn, realtime.EventPong, envelope.MessageID, nil)
	case realtime.EventSendMessage:
		h.sendMessage(ctx, conn, user, envelope)
	case realtime.EventMarkRead:
		h.markRead(ctx, conn, user, envelope)
	default:
		h.sendError(conn, 104400, "unsupported event type")
	}
	return nil
}

func (h *WebSocketHandler) sendMessage(ctx context.Context, conn *realtime.Connection, user *model.User, envelope realtime.Envelope) {
	var data struct {
		GroupID         string  `json:"group_id"`
		MsgType         uint8   `json:"msg_type"`
		Content         string  `json:"content"`
		ImageFileID     *uint64 `json:"image_file_id"`
		ClientMessageID string  `json:"client_message_id"`
	}
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		h.sendError(conn, 104400, "malformed send_message payload")
		return
	}
	// The client message id identifies the retry; fall back to the envelope id
	// for older clients that only send a message_id.
	clientMessageID := strings.TrimSpace(data.ClientMessageID)
	if clientMessageID == "" {
		clientMessageID = strings.TrimSpace(envelope.MessageID)
	}
	message, err := h.chats.SendMessage(ctx, user, service.SendMessageInput{
		GroupUUID:       data.GroupID,
		ClientMessageID: clientMessageID,
		MsgType:         data.MsgType,
		Content:         data.Content,
		ImageFileID:     data.ImageFileID,
		TraceID:         envelope.TraceID,
	})
	if err != nil {
		h.sendServiceError(conn, err)
		return
	}
	h.send(conn, realtime.EventAck, envelope.MessageID, map[string]any{
		"client_message_id": clientMessageID,
		"server_message_id": message.UUID,
	})
}

func (h *WebSocketHandler) markRead(ctx context.Context, conn *realtime.Connection, user *model.User, envelope realtime.Envelope) {
	var data struct {
		GroupID   string `json:"group_id"`
		MessageID string `json:"message_id"`
	}
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		h.sendError(conn, 104400, "malformed mark_read payload")
		return
	}
	if err := h.chats.MarkRead(ctx, user, data.GroupID, data.MessageID); err != nil {
		h.sendServiceError(conn, err)
	}
}

func (h *WebSocketHandler) sendServiceError(conn *realtime.Connection, err error) {
	switch {
	case errors.Is(err, service.ErrChatInvalidMessage):
		h.sendError(conn, 104400, "invalid chat message")
	case errors.Is(err, service.ErrForbidden):
		h.sendError(conn, 104403, "forbidden")
	case errors.Is(err, service.ErrChatGroupNotFound), errors.Is(err, service.ErrChatMessageNotFound):
		h.sendError(conn, 104404, "chat target not found")
	default:
		h.logger.Warn("websocket request failed", zap.Error(err))
		h.sendError(conn, 104500, "chat request failed")
	}
}

func (h *WebSocketHandler) sendError(conn *realtime.Connection, code int, message string) {
	h.send(conn, realtime.EventError, uuid.NewString(), map[string]any{"code": code, "message": message})
}

// send queues one envelope, closing the connection when the peer cannot keep up.
func (h *WebSocketHandler) send(conn *realtime.Connection, eventType, messageID string, data any) {
	envelope, err := realtime.NewEnvelope(eventType, messageID, time.Now().UTC().UnixMilli(), data)
	if err != nil {
		h.logger.Warn("encode websocket event", zap.String("event", eventType), zap.Error(err))
		return
	}
	if !conn.SendEnvelope(envelope) {
		h.logger.Warn("closing slow websocket peer", zap.String("connection_id", conn.ID))
		conn.Close()
	}
}

// originChecker rejects browser origins that are not in the allow list while
// still accepting non browser clients that send no Origin header at all.
func originChecker(allowedOrigins []string) func(*http.Request) bool {
	return func(r *http.Request) bool {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin == "" {
			return true
		}
		for _, allowed := range allowedOrigins {
			if strings.TrimSpace(allowed) == "*" || strings.EqualFold(strings.TrimSpace(allowed), origin) {
				return true
			}
		}
		return false
	}
}
