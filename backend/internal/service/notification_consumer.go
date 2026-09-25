package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/wangn-tech/campus-hub/internal/model"
	"github.com/wangn-tech/campus-hub/internal/platform/kafka"
	"github.com/wangn-tech/campus-hub/internal/repository"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// notificationNamespace seeds the deterministic notification keys.
var notificationNamespace = uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")

// NotificationConsumerGroup and NotificationTopics follow the Kafka design
// document sections 8.1 and 4.1.
const NotificationConsumerGroup = "campushub.notification-worker"

var NotificationTopics = []string{
	"campushub.registration.events.v1",
	"campushub.ticket.events.v1",
}

// notificationEvent is the subset of the outbox envelope the consumer reads.
type notificationEvent struct {
	EventID   string `json:"event_id"`
	EventType string `json:"event_type"`
	TraceID   string `json:"trace_id"`
	Payload   struct {
		ActivityID     string `json:"activity_id"`
		RegistrationID string `json:"registration_id"`
		UserID         string `json:"user_id"`
		Status         int    `json:"status"`
	} `json:"payload"`
}

// NotificationConsumer turns registration events into notifications.
type NotificationConsumer struct {
	notifications *repository.NotificationRepository
	activities    *repository.ActivityRepository
	users         *repository.UserRepository
	logger        *zap.Logger
}

func NewNotificationConsumer(notifications *repository.NotificationRepository, activities *repository.ActivityRepository, users *repository.UserRepository, logger *zap.Logger) *NotificationConsumer {
	return &NotificationConsumer{notifications: notifications, activities: activities, users: users, logger: logger}
}

// HandleEvent processes one Kafka record. Unknown or malformed events are
// skipped instead of blocking the consumer; database failures are returned so
// the record is replayed from the last committed offset.
func (c *NotificationConsumer) HandleEvent(ctx context.Context, record kafka.Record) error {
	var event notificationEvent
	if err := json.Unmarshal(record.Value, &event); err != nil {
		c.logger.Warn("skipping malformed notification event", zap.String("topic", record.Topic), zap.Error(err))
		return nil
	}
	switch event.EventType {
	case "registration.created":
		return c.notifyOrganizer(ctx, event)
	case "registration.approved":
		return c.notifyRegistrant(ctx, event, "registration_approved", "报名已通过", "你报名的「%s」已通过审核，请查看票据")
	case "registration.rejected":
		return c.notifyRegistrant(ctx, event, "registration_rejected", "报名未通过", "你报名的「%s」未通过审核")
	case "registration.expired":
		return c.notifyRegistrant(ctx, event, "registration_expired", "报名已超时", "你报名的「%s」审批超时，已自动取消")
	default:
		return nil
	}
}

func (c *NotificationConsumer) notifyOrganizer(ctx context.Context, event notificationEvent) error {
	activity, err := c.activities.FindByUUID(ctx, event.Payload.ActivityID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	content := fmt.Sprintf("有新的报名：%s", activity.Title)
	if registrant, err := c.users.FindByUUID(ctx, event.Payload.UserID); err == nil {
		content = fmt.Sprintf("%s 报名了「%s」", registrant.Nickname, activity.Title)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	owners, err := c.users.FindByIDs(ctx, []uint64{activity.OrganizerID})
	if err != nil || len(owners) == 0 {
		return err
	}
	return c.create(ctx, event, &owners[0], "registration_created", "有新的活动报名", content)
}

func (c *NotificationConsumer) notifyRegistrant(ctx context.Context, event notificationEvent, kind, title, format string) error {
	user, err := c.users.FindByUUID(ctx, event.Payload.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	activityTitle := ""
	if activity, err := c.activities.FindByUUID(ctx, event.Payload.ActivityID); err == nil {
		activityTitle = activity.Title
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return c.create(ctx, event, user, kind, title, fmt.Sprintf(format, activityTitle))
}

func (c *NotificationConsumer) create(ctx context.Context, event notificationEvent, user *model.User, kind, title, content string) error {
	data, err := json.Marshal(map[string]string{
		"activity_id":     event.Payload.ActivityID,
		"registration_id": event.Payload.RegistrationID,
	})
	if err != nil {
		return err
	}
	encoded := string(data)
	notification := model.Notification{
		UUID:      notificationKey(event.EventID, user.ID),
		UserID:    user.ID,
		Type:      kind,
		Title:     title,
		Content:   content,
		Data:      &encoded,
		CreatedAt: time.Now().UTC(),
	}
	hint, err := newNotificationDeliveryEvent(user.UUID, notification, event.TraceID)
	if err != nil {
		return err
	}
	returnErr := error(nil)
	if _, err := c.notifications.Create(ctx, &notification, []*model.OutboxEvent{hint}); err != nil {
		returnErr = err
	}
	return returnErr
}

// notificationKey derives a stable id per (event, recipient), so a redelivered
// event cannot create a second notification.
func notificationKey(eventID string, userID uint64) string {
	return uuid.NewSHA1(notificationNamespace, []byte(fmt.Sprintf("%s:%d", eventID, userID))).String()
}
