package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/wangn-tech/campus-hub/internal/model"
	"github.com/wangn-tech/campus-hub/internal/platform/kafka"
	"github.com/wangn-tech/campus-hub/internal/repository"
)

const (
	outboxBatchSize = 100
	// outboxPublishTimeout bounds one publish call so the claim transaction
	// cannot hold its row locks for long.
	outboxPublishTimeout = 10 * time.Second
)

// relayTopics lists every topic eventTopic can return (see the Kafka design
// document section 4.1). The relay makes sure they exist before publishing.
var relayTopics = []string{
	"campushub.activity.events.v1",
	"campushub.registration.events.v1",
	"campushub.ticket.events.v1",
	"campushub.notification.events.v1",
	"campushub.chat.events.v1",
	"campushub.file.events.v1",
	"campushub.system.audit.v1",
}

// RelayTopics returns the topics the relay publishes to.
func RelayTopics() []string {
	return append([]string(nil), relayTopics...)
}

// MessagePublisher publishes a batch of messages and reports one error per
// message.
type MessagePublisher interface {
	EnsureTopics(ctx context.Context, topics ...string) error
	Publish(ctx context.Context, messages []kafka.Message) []error
}

// OutboxRelay delivers pending outbox events to Kafka.
type OutboxRelay struct {
	outbox    *repository.OutboxRepository
	publisher MessagePublisher
}

func NewOutboxRelay(outbox *repository.OutboxRepository, publisher MessagePublisher) *OutboxRelay {
	return &OutboxRelay{outbox: outbox, publisher: publisher}
}

// Run publishes one batch and returns how many events were delivered and how
// many are scheduled for a retry.
func (r *OutboxRelay) Run(ctx context.Context) (int, int, error) {
	now := time.Now().UTC()
	if err := r.publisher.EnsureTopics(ctx, relayTopics...); err != nil {
		return 0, 0, err
	}
	var firstErr error
	sent, failed, err := r.outbox.PublishBatch(ctx, now, outboxBatchSize, func(events []model.OutboxEvent) []error {
		messages := make([]kafka.Message, 0, len(events))
		for _, event := range events {
			messages = append(messages, kafka.Message{
				Topic: eventTopic(event.EventType),
				Key:   event.PartitionKey,
				Value: []byte(event.Payload),
				Headers: map[string]string{
					"event_id":   event.EventID,
					"event_type": event.EventType,
				},
			})
		}
		publishCtx, cancel := context.WithTimeout(ctx, outboxPublishTimeout)
		defer cancel()
		results := r.publisher.Publish(publishCtx, messages)
		for _, result := range results {
			if result != nil && firstErr == nil {
				firstErr = result
			}
		}
		return results
	})
	if err != nil {
		return sent, failed, err
	}
	if failed > 0 && firstErr != nil {
		return sent, failed, fmt.Errorf("publish %d of %d outbox events: %w", failed, sent+failed, firstErr)
	}
	return sent, failed, nil
}

// eventTopic maps an event type to its Kafka topic (see the Kafka design
// document section 4.1).
func eventTopic(eventType string) string {
	switch {
	case strings.HasPrefix(eventType, "activity."):
		return "campushub.activity.events.v1"
	case strings.HasPrefix(eventType, "registration."):
		return "campushub.registration.events.v1"
	case strings.HasPrefix(eventType, "ticket."):
		return "campushub.ticket.events.v1"
	case strings.HasPrefix(eventType, "notification."):
		return "campushub.notification.events.v1"
	case strings.HasPrefix(eventType, "chat."):
		return "campushub.chat.events.v1"
	case strings.HasPrefix(eventType, "file."):
		return "campushub.file.events.v1"
	default:
		return "campushub.system.audit.v1"
	}
}
