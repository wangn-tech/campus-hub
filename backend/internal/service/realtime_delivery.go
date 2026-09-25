package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/wangn-tech/campus-hub/internal/model"
	"github.com/wangn-tech/campus-hub/internal/platform/kafka"
	"github.com/wangn-tech/campus-hub/internal/realtime"
	"github.com/wangn-tech/campus-hub/internal/repository"
	"go.uber.org/zap"
)

const realtimeEventsTopic = "campushub.realtime.events.v1"

// RealtimeTopics contains durable business-event topics and the lightweight
// notification-delivery hint topic. Every instance consumes all three.
func RealtimeTopics() []string {
	return []string{
		"campushub.registration.events.v1",
		"campushub.verification.events.v1",
		realtimeEventsTopic,
	}
}

func RealtimeDeliveryGroup(prefix, instanceID string) string { return prefix + "." + instanceID }

type UserDeliverer interface {
	DeliverToUser(userUUID string, payload []byte) int
}

type realtimeEvent struct {
	EventID   string          `json:"event_id"`
	EventType string          `json:"event_type"`
	TraceID   string          `json:"trace_id"`
	Payload   json.RawMessage `json:"payload"`
}

type notificationDeliveryPayload struct {
	UserID       string             `json:"user_id"`
	Notification model.Notification `json:"notification"`
}

// RealtimeDeliveryConsumer renders user-targeted events into WebSocket frames.
// It deliberately treats Kafka delivery as a hint: each underlying result is
// already persisted, and reconnecting clients can query it through HTTP.
type RealtimeDeliveryConsumer struct {
	deliverer     UserDeliverer
	registrations *repository.RegistrationRepository
	tickets       *repository.TicketRepository
	chats         *repository.ChatRepository
	logger        *zap.Logger
}

func NewRealtimeDeliveryConsumer(deliverer UserDeliverer, registrations *repository.RegistrationRepository, tickets *repository.TicketRepository, chats *repository.ChatRepository, logger *zap.Logger) *RealtimeDeliveryConsumer {
	return &RealtimeDeliveryConsumer{deliverer: deliverer, registrations: registrations, tickets: tickets, chats: chats, logger: logger}
}

func (c *RealtimeDeliveryConsumer) HandleEvent(ctx context.Context, record kafka.Record) error {
	var event realtimeEvent
	if err := json.Unmarshal(record.Value, &event); err != nil {
		c.logger.Warn("skipping malformed realtime event", zap.String("topic", record.Topic), zap.Error(err))
		return nil
	}
	switch {
	case event.EventType == "realtime.notification_created":
		return c.deliverNotification(event)
	case len(event.EventType) > len("registration.") && event.EventType[:len("registration.")] == "registration.":
		return c.deliverRegistration(ctx, event)
	case len(event.EventType) > len("verification.") && event.EventType[:len("verification.")] == "verification.":
		return c.deliverVerification(event)
	default:
		return nil
	}
}

func (c *RealtimeDeliveryConsumer) deliverNotification(event realtimeEvent) error {
	var payload notificationDeliveryPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil || payload.UserID == "" {
		return nil
	}
	data, err := json.Marshal(map[string]any{
		"notification_id":   payload.Notification.UUID,
		"notification_type": payload.Notification.Type,
		"title":             payload.Notification.Title,
		"content":           payload.Notification.Content,
		"created_at":        payload.Notification.CreatedAt.UTC().UnixMilli(),
	})
	if err != nil {
		return err
	}
	return c.deliver(payload.UserID, realtime.EventNotification, payload.Notification.UUID, event.TraceID, data)
}

func (c *RealtimeDeliveryConsumer) deliverRegistration(ctx context.Context, event realtimeEvent) error {
	var payload struct {
		ActivityID     string `json:"activity_id"`
		RegistrationID string `json:"registration_id"`
		UserID         string `json:"user_id"`
		FromStatus     *uint8 `json:"from_status"`
		ToStatus       uint8  `json:"to_status"`
		Status         uint8  `json:"status"`
		TicketID       string `json:"ticket_id"`
		GroupID        string `json:"group_id"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil || payload.UserID == "" || payload.RegistrationID == "" {
		return nil
	}
	toStatus := payload.ToStatus
	if toStatus == 0 && payload.Status != 0 {
		toStatus = payload.Status
	}
	var activityID uint64
	if c.registrations != nil {
		if registration, err := c.registrations.FindByUUID(ctx, payload.RegistrationID); err == nil {
			activityID = registration.ActivityID
			if c.tickets != nil {
				if ticket, err := c.tickets.FindByRegistrationID(ctx, registration.ID); err == nil {
					payload.TicketID = ticket.UUID
				}
			}
		}
	}
	if c.chats != nil && activityID != 0 {
		if group, err := c.chats.FindGroupByActivityID(ctx, activityID); err == nil {
			payload.GroupID = group.UUID
		}
	}
	dataMap := map[string]any{
		"registration_id": payload.RegistrationID,
		"activity_id":     payload.ActivityID,
		"to_status":       registrationStatusName(toStatus),
		"message":         registrationStatusMessage(toStatus),
	}
	if payload.FromStatus != nil {
		dataMap["from_status"] = registrationStatusName(*payload.FromStatus)
	}
	if payload.TicketID != "" {
		dataMap["ticket_id"] = payload.TicketID
	}
	if payload.GroupID != "" {
		dataMap["group_id"] = payload.GroupID
	}
	data, err := json.Marshal(dataMap)
	if err != nil {
		return err
	}
	return c.deliver(payload.UserID, realtime.EventRegistrationStatusChanged, payload.RegistrationID, event.TraceID, data)
}

func (c *RealtimeDeliveryConsumer) deliverVerification(event realtimeEvent) error {
	var payload struct {
		VerificationID string `json:"verification_id"`
		UserID         string `json:"user_id"`
		Status         uint8  `json:"status"`
		Message        string `json:"message"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil || payload.UserID == "" || payload.VerificationID == "" {
		return nil
	}
	data, err := json.Marshal(map[string]any{
		"verification_id": payload.VerificationID,
		"status":          verificationStatusName(payload.Status),
		"progress":        verificationProgress(payload.Status),
		"message":         payload.Message,
	})
	if err != nil {
		return err
	}
	return c.deliver(payload.UserID, realtime.EventVerifyProgress, payload.VerificationID, event.TraceID, data)
}

func (c *RealtimeDeliveryConsumer) deliver(userID, eventType, messageID, traceID string, data []byte) error {
	frame, err := json.Marshal(realtime.Envelope{Type: eventType, MessageID: messageID, Timestamp: time.Now().UTC().UnixMilli(), TraceID: traceID, Data: data})
	if err != nil {
		return fmt.Errorf("encode realtime frame: %w", err)
	}
	c.deliverer.DeliverToUser(userID, frame)
	return nil
}

func registrationStatusName(status uint8) string {
	switch model.RegistrationStatus(status) {
	case model.RegistrationPending:
		return "pending"
	case model.RegistrationApproved:
		return "approved"
	case model.RegistrationRejected:
		return "rejected"
	case model.RegistrationCancelled:
		return "cancelled"
	case model.RegistrationExpired:
		return "expired"
	default:
		return "failed"
	}
}

func registrationStatusMessage(status uint8) string {
	switch registrationStatusName(status) {
	case "approved":
		return "报名已通过"
	case "rejected":
		return "报名未通过"
	case "cancelled":
		return "报名已取消"
	case "expired":
		return "报名已超时"
	default:
		return "报名状态已更新"
	}
}

func verificationStatusName(status uint8) string {
	switch model.StudentVerificationStatus(status) {
	case model.VerificationPendingConfirm:
		return "pending_confirm"
	case model.VerificationManualReview:
		return "manual_review"
	case model.VerificationCancelled:
		return "cancelled"
	case model.VerificationApproved:
		return "approved"
	case model.VerificationRejected:
		return "rejected"
	default:
		return "initialized"
	}
}

func verificationProgress(status uint8) int {
	switch model.StudentVerificationStatus(status) {
	case model.VerificationPendingConfirm:
		return 60
	case model.VerificationManualReview:
		return 80
	case model.VerificationApproved, model.VerificationRejected, model.VerificationCancelled:
		return 100
	default:
		return 0
	}
}

func newNotificationDeliveryEvent(userID string, notification model.Notification, traceID string) (*model.OutboxEvent, error) {
	payload := notificationDeliveryPayload{UserID: userID, Notification: notification}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var fields map[string]any
	if err := json.Unmarshal(encoded, &fields); err != nil {
		return nil, err
	}
	return newOutboxEvent("realtime.notification_created", "notification", notification.UUID, traceID, fields)
}
