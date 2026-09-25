package service

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/wangn-tech/campus-hub/internal/model"
)

// outboxProducer identifies this service in the event envelope.
const outboxProducer = "campushub-backend"

// newOutboxEvent builds an outbox_events row carrying the event envelope from
// the Kafka design document (§5). The envelope is stored as the row payload so
// the relay can publish it unchanged.
func newOutboxEvent(eventType, aggregateType, aggregateID, traceID string, payload map[string]any) (*model.OutboxEvent, error) {
	now := time.Now().UTC()
	eventID := uuid.NewString()
	if payload == nil {
		payload = map[string]any{}
	}
	envelope := map[string]any{
		"event_id":       eventID,
		"event_type":     eventType,
		"event_version":  1,
		"event_time":     now.UnixMilli(),
		"aggregate_type": aggregateType,
		"aggregate_id":   aggregateID,
		"trace_id":       traceID,
		"producer":       outboxProducer,
		"payload":        payload,
	}
	encoded, err := json.Marshal(envelope)
	if err != nil {
		return nil, err
	}
	return &model.OutboxEvent{
		EventID:       eventID,
		EventType:     eventType,
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		PartitionKey:  aggregateID,
		Payload:       string(encoded),
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}
