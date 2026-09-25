package realtime

import (
	"context"
	"sync"

	"go.uber.org/zap"
)

// Hub keeps the live connections of this instance and routes frames to the
// users and groups they belong to. Rooms are keyed by the external group UUID
// because that is what both clients and Kafka events carry.
type Hub struct {
	mu      sync.RWMutex
	conns   map[*Connection]struct{}
	byUser  map[string]map[*Connection]struct{}
	byGroup map[string]map[*Connection]struct{}

	presence Presence
	logger   *zap.Logger
}

func NewHub(presence Presence, logger *zap.Logger) *Hub {
	return &Hub{
		conns:    make(map[*Connection]struct{}),
		byUser:   make(map[string]map[*Connection]struct{}),
		byGroup:  make(map[string]map[*Connection]struct{}),
		presence: presence,
		logger:   logger,
	}
}

// Register adds an authenticated connection and marks the user online.
func (h *Hub) Register(ctx context.Context, conn *Connection) {
	h.mu.Lock()
	h.conns[conn] = struct{}{}
	if h.byUser[conn.UserUUID] == nil {
		h.byUser[conn.UserUUID] = make(map[*Connection]struct{})
	}
	h.byUser[conn.UserUUID][conn] = struct{}{}
	h.mu.Unlock()

	if h.presence != nil {
		if err := h.presence.Connected(ctx, conn.UserUUID, conn.ID); err != nil {
			h.warn("mark user online", conn, err)
		}
	}
}

// Unregister removes a connection from every room and clears the online marker.
func (h *Hub) Unregister(ctx context.Context, conn *Connection) {
	h.mu.Lock()
	delete(h.conns, conn)
	if peers := h.byUser[conn.UserUUID]; peers != nil {
		delete(peers, conn)
		if len(peers) == 0 {
			delete(h.byUser, conn.UserUUID)
		}
	}
	for groupUUID, members := range h.byGroup {
		delete(members, conn)
		if len(members) == 0 {
			delete(h.byGroup, groupUUID)
		}
	}
	h.mu.Unlock()

	if h.presence != nil {
		if err := h.presence.Disconnected(context.WithoutCancel(ctx), conn.UserUUID, conn.ID); err != nil {
			h.warn("mark user offline", conn, err)
		}
	}
}

// Join subscribes a connection to one group room.
func (h *Hub) Join(groupUUID string, conn *Connection) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.byGroup[groupUUID] == nil {
		h.byGroup[groupUUID] = make(map[*Connection]struct{})
	}
	h.byGroup[groupUUID][conn] = struct{}{}
}

// JoinGroups subscribes a connection to several rooms, used right after auth to
// restore the memberships of the reconnecting user.
func (h *Hub) JoinGroups(groupUUIDs []string, conn *Connection) {
	for _, groupUUID := range groupUUIDs {
		h.Join(groupUUID, conn)
	}
}

// DeliverToGroup queues a frame for every local member of the group and reports
// how many connections received it.
func (h *Hub) DeliverToGroup(groupUUID string, payload []byte) int {
	h.mu.RLock()
	targets := make([]*Connection, 0, len(h.byGroup[groupUUID]))
	for conn := range h.byGroup[groupUUID] {
		targets = append(targets, conn)
	}
	h.mu.RUnlock()
	return h.deliver(targets, payload)
}

// DeliverToUser queues a frame for every local connection of one user.
func (h *Hub) DeliverToUser(userUUID string, payload []byte) int {
	h.mu.RLock()
	targets := make([]*Connection, 0, len(h.byUser[userUUID]))
	for conn := range h.byUser[userUUID] {
		targets = append(targets, conn)
	}
	h.mu.RUnlock()
	return h.deliver(targets, payload)
}

// GroupMemberCount reports how many local connections joined the room.
func (h *Hub) GroupMemberCount(groupUUID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.byGroup[groupUUID])
}

// Shutdown closes every connection so a graceful stop sends a clean close frame
// instead of dropping sockets.
func (h *Hub) Shutdown() {
	h.mu.RLock()
	targets := make([]*Connection, 0, len(h.conns))
	for conn := range h.conns {
		targets = append(targets, conn)
	}
	h.mu.RUnlock()
	for _, conn := range targets {
		conn.Close()
	}
}

func (h *Hub) deliver(targets []*Connection, payload []byte) int {
	delivered := 0
	for _, conn := range targets {
		if conn.SendRaw(payload) {
			delivered++
			continue
		}
		// A full queue means the peer cannot keep up; drop it rather than
		// blocking the broadcaster.
		h.warn("closing slow websocket peer", conn, nil)
		conn.Close()
	}
	return delivered
}

func (h *Hub) warn(message string, conn *Connection, err error) {
	if h.logger == nil {
		return
	}
	fields := []zap.Field{zap.String("connection_id", conn.ID), zap.String("user_id", conn.UserUUID)}
	if err != nil {
		fields = append(fields, zap.Error(err))
	}
	h.logger.Warn(message, fields...)
}
