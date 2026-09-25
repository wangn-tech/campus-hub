package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wangn-tech/campus-hub/internal/model"
	"github.com/wangn-tech/campus-hub/internal/platform/kafka"
	"github.com/wangn-tech/campus-hub/internal/platform/timestamp"
	"github.com/wangn-tech/campus-hub/internal/repository"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var (
	ErrChatGroupNotFound   = errors.New("chat group not found")
	ErrChatMessageNotFound = errors.New("chat message not found")
	ErrChatInvalidMessage  = errors.New("invalid chat message")
)

const (
	chatMessageDefaultLimit = 30
	chatMessageMaxLimit     = 100
	chatMessageMaxContent   = 2000
	// chatImageBizType marks uploads that may be attached to a chat message.
	chatImageBizType = "chat_image"
)

type ChatService struct {
	chats       *repository.ChatRepository
	activities  *repository.ActivityRepository
	users       *repository.UserRepository
	files       *repository.FileRepository
	attachments *FileService
	// publisher spreads persisted messages to every instance; a nil publisher
	// keeps the read and persistence paths usable on their own.
	publisher MessagePublisher
	logger    *zap.Logger
}

func NewChatService(chats *repository.ChatRepository, activities *repository.ActivityRepository, users *repository.UserRepository, files *repository.FileRepository, attachments *FileService, publisher MessagePublisher, logger *zap.Logger) *ChatService {
	return &ChatService{chats: chats, activities: activities, users: users, files: files, attachments: attachments, publisher: publisher, logger: logger}
}

type ChatGroupView struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	ActivityID  string           `json:"activity_id"`
	Status      uint8            `json:"status"`
	MemberCount int64            `json:"member_count"`
	CreatedAt   timestamp.Millis `json:"created_at"`
}

type ChatMemberView struct {
	ID        string `json:"id"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
	Role      uint8  `json:"role"`
	Muted     bool   `json:"muted"`
}

type ChatMessageView struct {
	ID        uint64           `json:"id"`
	UUID      string           `json:"uuid"`
	GroupID   string           `json:"group_id"`
	Sender    ChatMemberView   `json:"sender"`
	MsgType   uint8            `json:"msg_type"`
	Content   string           `json:"content"`
	ImageURL  string           `json:"image_url"`
	CreatedAt timestamp.Millis `json:"created_at"`
}

// MyGroups lists the groups the caller is an active member of.
func (s *ChatService) MyGroups(ctx context.Context, user *model.User) ([]ChatGroupView, error) {
	groups, err := s.chats.ListGroupsByUser(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	return s.groupViews(ctx, groups)
}

// Group returns one group to its members.
func (s *ChatService) Group(ctx context.Context, user *model.User, groupUUID string) (*ChatGroupView, error) {
	group, err := s.memberGroup(ctx, user, groupUUID)
	if err != nil {
		return nil, err
	}
	views, err := s.groupViews(ctx, []model.ChatGroup{*group})
	if err != nil {
		return nil, err
	}
	return &views[0], nil
}

func (s *ChatService) Members(ctx context.Context, user *model.User, groupUUID string) ([]ChatMemberView, error) {
	group, err := s.memberGroup(ctx, user, groupUUID)
	if err != nil {
		return nil, err
	}
	members, err := s.chats.ListMembers(ctx, group.ID)
	if err != nil {
		return nil, err
	}
	userIDs := make([]uint64, 0, len(members))
	for _, member := range members {
		userIDs = append(userIDs, member.UserID)
	}
	users, err := s.users.FindByIDs(ctx, uniqueUint64(userIDs))
	if err != nil {
		return nil, err
	}
	avatarURLs, err := presignAvatarURLs(ctx, s.files, s.attachments, users)
	if err != nil {
		return nil, err
	}
	byID := make(map[uint64]model.User, len(users))
	for _, item := range users {
		byID[item.ID] = item
	}
	views := make([]ChatMemberView, 0, len(members))
	for _, member := range members {
		view := ChatMemberView{Role: member.Role, Muted: member.Muted}
		if user, ok := byID[member.UserID]; ok {
			view.ID = user.UUID
			view.Nickname = user.Nickname
			view.AvatarURL = avatarURLs[user.ID]
		}
		views = append(views, view)
	}
	return views, nil
}

// Messages returns a group's history, newest first. after_id pages forward for
// clients that already hold older messages.
func (s *ChatService) Messages(ctx context.Context, user *model.User, groupUUID string, afterID uint64, limit int) ([]ChatMessageView, error) {
	group, err := s.memberGroup(ctx, user, groupUUID)
	if err != nil {
		return nil, err
	}
	messages, err := s.chats.ListMessages(ctx, repository.MessageFilter{
		GroupID: group.ID,
		AfterID: afterID,
		Limit:   clampMessageLimit(limit),
	})
	if err != nil {
		return nil, err
	}
	return s.messageViews(ctx, messages)
}

// Offline returns messages the caller may have missed across their groups.
func (s *ChatService) Offline(ctx context.Context, user *model.User, afterID uint64, limit int) ([]ChatMessageView, error) {
	groups, err := s.chats.ListGroupsByUser(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	groupIDs := make([]uint64, 0, len(groups))
	for _, group := range groups {
		groupIDs = append(groupIDs, group.ID)
	}
	messages, err := s.chats.ListMessagesForGroups(ctx, groupIDs, afterID, clampMessageLimit(limit))
	if err != nil {
		return nil, err
	}
	return s.messageViews(ctx, messages)
}

func (s *ChatService) groupViews(ctx context.Context, groups []model.ChatGroup) ([]ChatGroupView, error) {
	views := make([]ChatGroupView, 0, len(groups))
	if len(groups) == 0 {
		return views, nil
	}
	groupIDs := make([]uint64, 0, len(groups))
	activityIDs := make([]uint64, 0, len(groups))
	for _, group := range groups {
		groupIDs = append(groupIDs, group.ID)
		activityIDs = append(activityIDs, group.ActivityID)
	}
	counts, err := s.chats.CountMembers(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	activities, err := s.activities.FindByIDs(ctx, uniqueUint64(activityIDs))
	if err != nil {
		return nil, err
	}
	activityUUID := make(map[uint64]string, len(activities))
	for _, activity := range activities {
		activityUUID[activity.ID] = activity.UUID
	}
	for _, group := range groups {
		views = append(views, ChatGroupView{
			ID:          group.UUID,
			Name:        group.Name,
			ActivityID:  activityUUID[group.ActivityID],
			Status:      group.Status,
			MemberCount: counts[group.ID],
			CreatedAt:   timestamp.Millis(group.CreatedAt),
		})
	}
	return views, nil
}

func (s *ChatService) messageViews(ctx context.Context, messages []model.ChatMessage) ([]ChatMessageView, error) {
	views := make([]ChatMessageView, 0, len(messages))
	if len(messages) == 0 {
		return views, nil
	}
	senderIDs := make([]uint64, 0, len(messages))
	groupIDs := make([]uint64, 0, len(messages))
	imageIDs := make([]uint64, 0, len(messages))
	for _, message := range messages {
		senderIDs = append(senderIDs, message.SenderID)
		groupIDs = append(groupIDs, message.GroupID)
		if message.ImageFileID != nil {
			imageIDs = append(imageIDs, *message.ImageFileID)
		}
	}
	users, err := s.users.FindByIDs(ctx, uniqueUint64(senderIDs))
	if err != nil {
		return nil, err
	}
	avatarURLs, err := presignAvatarURLs(ctx, s.files, s.attachments, users)
	if err != nil {
		return nil, err
	}
	senderByID := make(map[uint64]ChatMemberView, len(users))
	for _, user := range users {
		senderByID[user.ID] = ChatMemberView{ID: user.UUID, Nickname: user.Nickname, AvatarURL: avatarURLs[user.ID]}
	}
	groups, err := s.chats.FindGroupsByIDs(ctx, uniqueUint64(groupIDs))
	if err != nil {
		return nil, err
	}
	groupUUID := make(map[uint64]string, len(groups))
	for _, group := range groups {
		groupUUID[group.ID] = group.UUID
	}
	imageURLs, err := presignFileURLs(ctx, s.files, s.attachments, imageIDs)
	if err != nil {
		return nil, err
	}
	for _, message := range messages {
		view := ChatMessageView{
			ID:        message.ID,
			UUID:      message.UUID,
			GroupID:   groupUUID[message.GroupID],
			Sender:    senderByID[message.SenderID],
			MsgType:   message.MsgType,
			CreatedAt: timestamp.Millis(message.CreatedAt),
		}
		if message.Content != nil {
			view.Content = *message.Content
		}
		if message.ImageFileID != nil {
			view.ImageURL = imageURLs[*message.ImageFileID]
		}
		views = append(views, view)
	}
	return views, nil
}

// memberGroup resolves a group and requires the caller to be an active member.
func (s *ChatService) memberGroup(ctx context.Context, user *model.User, groupUUID string) (*model.ChatGroup, error) {
	group, err := s.chats.FindGroupByUUID(ctx, strings.TrimSpace(groupUUID))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrChatGroupNotFound
		}
		return nil, err
	}
	member, err := s.chats.FindMember(ctx, group.ID, user.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrForbidden
		}
		return nil, err
	}
	if member.Status != model.ChatMemberStatusActive {
		return nil, ErrForbidden
	}
	return group, nil
}

func clampMessageLimit(limit int) int {
	if limit < 1 {
		return chatMessageDefaultLimit
	}
	if limit > chatMessageMaxLimit {
		return chatMessageMaxLimit
	}
	return limit
}

// SendMessageInput is one `send_message` frame. ClientMessageID is the client
// side idempotency key from the WebSocket design document section 13.4.
type SendMessageInput struct {
	GroupUUID       string
	ClientMessageID string
	MsgType         uint8
	Content         string
	ImageFileID     *uint64
	TraceID         string
}

// SendMessage persists a group message and publishes it for every instance to
// deliver. Reusing a ClientMessageID returns the original message instead of
// creating a duplicate, so a client retry after a lost ACK is safe.
func (s *ChatService) SendMessage(ctx context.Context, user *model.User, in SendMessageInput) (*ChatMessageView, error) {
	group, err := s.memberGroup(ctx, user, in.GroupUUID)
	if err != nil {
		return nil, err
	}
	clientMessageID := strings.TrimSpace(in.ClientMessageID)
	if clientMessageID == "" || len(clientMessageID) > 64 {
		return nil, ErrChatInvalidMessage
	}
	if existing, err := s.chats.FindMessageByClientID(ctx, user.ID, clientMessageID); err == nil {
		return s.singleMessageView(ctx, *existing)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	message := &model.ChatMessage{
		UUID:            uuid.NewString(),
		GroupID:         group.ID,
		SenderID:        user.ID,
		ClientMessageID: clientMessageID,
		MsgType:         in.MsgType,
		Status:          1,
		CreatedAt:       time.Now().UTC(),
	}
	if err := s.applyMessageBody(ctx, user, message, in); err != nil {
		return nil, err
	}
	if err := s.chats.CreateMessage(ctx, message); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			// A concurrent retry won the insert; answer with the stored row.
			existing, findErr := s.chats.FindMessageByClientID(ctx, user.ID, clientMessageID)
			if findErr != nil {
				return nil, findErr
			}
			return s.singleMessageView(ctx, *existing)
		}
		return nil, err
	}
	view, err := s.singleMessageView(ctx, *message)
	if err != nil {
		return nil, err
	}
	s.publishMessage(ctx, group, view, in.TraceID)
	return view, nil
}

// applyMessageBody validates the frame contents and fills the message body.
func (s *ChatService) applyMessageBody(ctx context.Context, user *model.User, message *model.ChatMessage, in SendMessageInput) error {
	switch in.MsgType {
	case model.ChatMessageText:
		content := strings.TrimSpace(in.Content)
		if content == "" || len([]rune(content)) > chatMessageMaxContent {
			return ErrChatInvalidMessage
		}
		message.Content = &content
	case model.ChatMessageImage:
		if in.ImageFileID == nil {
			return ErrChatInvalidMessage
		}
		file, err := s.files.FindByID(ctx, *in.ImageFileID)
		if err != nil || file.UploaderID != user.ID || file.BizType != chatImageBizType || file.Status != 1 {
			return ErrChatInvalidMessage
		}
		message.ImageFileID = in.ImageFileID
	default:
		return ErrChatInvalidMessage
	}
	return nil
}

// MarkRead moves the caller's read pointer in one group. Only members may do
// so, and the message has to belong to the same group.
func (s *ChatService) MarkRead(ctx context.Context, user *model.User, groupUUID, messageUUID string) error {
	group, err := s.memberGroup(ctx, user, groupUUID)
	if err != nil {
		return err
	}
	message, err := s.chats.FindMessageByUUID(ctx, strings.TrimSpace(messageUUID))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrChatMessageNotFound
		}
		return err
	}
	if message.GroupID != group.ID {
		return ErrForbidden
	}
	return s.chats.UpdateLastRead(ctx, group.ID, user.ID, message.ID)
}

func (s *ChatService) singleMessageView(ctx context.Context, message model.ChatMessage) (*ChatMessageView, error) {
	views, err := s.messageViews(ctx, []model.ChatMessage{message})
	if err != nil {
		return nil, err
	}
	return &views[0], nil
}

// publishMessage spreads the message to the other instances (and back to this
// one) through Kafka. A publish failure is logged instead of failing the send:
// the message is already durable and members can still pull it over HTTP.
func (s *ChatService) publishMessage(ctx context.Context, group *model.ChatGroup, view *ChatMessageView, traceID string) {
	if s.publisher == nil {
		return
	}
	payload, err := newChatMessageSentEvent(chatMessagePayload{
		MessageID:    view.UUID,
		GroupID:      group.UUID,
		SenderID:     view.Sender.ID,
		SenderName:   view.Sender.Nickname,
		SenderAvatar: view.Sender.AvatarURL,
		MsgType:      view.MsgType,
		Content:      view.Content,
		ImageURL:     view.ImageURL,
		CreatedAt:    timestamp.ToMillis(view.CreatedAt.Time()),
	}, traceID)
	if err != nil {
		s.warn("encode chat message event", zap.Error(err))
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if errs := s.publisher.Publish(ctx, []kafka.Message{{Topic: chatEventsTopic, Key: group.UUID, Value: payload}}); len(errs) > 0 && errs[0] != nil {
		s.warn("publish chat message event", zap.Error(errs[0]))
	}
}

func (s *ChatService) warn(message string, fields ...zap.Field) {
	if s.logger == nil {
		return
	}
	s.logger.Warn(message, fields...)
}
