package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/wangn-tech/campus-hub/internal/model"
	"github.com/wangn-tech/campus-hub/internal/platform/kafka"
	"github.com/wangn-tech/campus-hub/internal/repository"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ChatMembershipConsumerGroup keeps activity group membership in sync with
// registration events (see the WebSocket design document section on joining).
const ChatMembershipConsumerGroup = "campushub.chat-membership"

var ChatMembershipTopics = []string{"campushub.registration.events.v1"}

// chatMembershipAction is the pure decision derived from an event type, kept
// separate so the join/leave mapping is testable without a database.
type chatMembershipAction int

const (
	chatMembershipIgnore chatMembershipAction = iota
	chatMembershipJoin
	chatMembershipLeave
)

func chatMembershipActionFor(eventType string) chatMembershipAction {
	switch eventType {
	case "registration.approved":
		return chatMembershipJoin
	case "registration.cancelled", "registration.rejected", "registration.expired":
		return chatMembershipLeave
	default:
		return chatMembershipIgnore
	}
}

type ChatMembershipConsumer struct {
	chats      *repository.ChatRepository
	activities *repository.ActivityRepository
	users      *repository.UserRepository
	logger     *zap.Logger
}

func NewChatMembershipConsumer(chats *repository.ChatRepository, activities *repository.ActivityRepository, users *repository.UserRepository, logger *zap.Logger) *ChatMembershipConsumer {
	return &ChatMembershipConsumer{chats: chats, activities: activities, users: users, logger: logger}
}

// HandleEvent joins or leaves the activity group based on the registration
// status. Unknown or malformed events are skipped instead of blocking the
// consumer.
func (c *ChatMembershipConsumer) HandleEvent(ctx context.Context, record kafka.Record) error {
	var event notificationEvent
	if err := json.Unmarshal(record.Value, &event); err != nil {
		c.logger.Warn("skipping malformed chat event", zap.String("topic", record.Topic), zap.Error(err))
		return nil
	}
	switch chatMembershipActionFor(event.EventType) {
	case chatMembershipJoin:
		return c.join(ctx, event)
	case chatMembershipLeave:
		return c.leave(ctx, event)
	default:
		return nil
	}
}

func (c *ChatMembershipConsumer) join(ctx context.Context, event notificationEvent) error {
	user, err := c.users.FindByUUID(ctx, event.Payload.UserID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	group, err := c.findOrCreateGroup(ctx, event.Payload.ActivityID)
	if err != nil || group == nil {
		return err
	}
	return c.chats.JoinGroup(ctx, group.ID, user.ID, model.ChatMemberRoleMember)
}

func (c *ChatMembershipConsumer) leave(ctx context.Context, event notificationEvent) error {
	user, err := c.users.FindByUUID(ctx, event.Payload.UserID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	activity, err := c.activities.FindByUUID(ctx, event.Payload.ActivityID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	group, err := c.chats.FindGroupByActivityID(ctx, activity.ID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return c.chats.LeaveGroup(ctx, group.ID, user.ID)
}

// findOrCreateGroup returns the activity group, creating it with its owner for
// activities that were published before groups existed.
func (c *ChatMembershipConsumer) findOrCreateGroup(ctx context.Context, activityUUID string) (*model.ChatGroup, error) {
	activity, err := c.activities.FindByUUID(ctx, activityUUID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	group, err := c.chats.FindGroupByActivityID(ctx, activity.ID)
	if err == nil {
		return group, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	now := time.Now().UTC()
	group = &model.ChatGroup{
		UUID:       uuid.NewString(),
		ActivityID: activity.ID,
		Name:       activity.Title,
		Status:     model.ChatGroupActive,
		OwnerID:    activity.OrganizerID,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	owner := &model.ChatGroupMember{
		UserID:    activity.OrganizerID,
		Role:      model.ChatMemberRoleOwner,
		Status:    model.ChatMemberStatusActive,
		JoinedAt:  now,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := c.chats.CreateGroupWithOwner(ctx, group, owner); err != nil {
		return nil, err
	}
	return group, nil
}
