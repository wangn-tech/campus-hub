package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/wangn-tech/campus-hub/internal/platform/kafka"
	"github.com/wangn-tech/campus-hub/internal/realtime"
	"go.uber.org/zap"
)

// GroupDeliverer pushes a rendered frame to the live connections of one group
// room. The realtime hub implements it, which keeps this consumer free of any
// dependency on the WebSocket transport.
type GroupDeliverer interface {
	DeliverToGroup(groupUUID string, payload []byte) int
}

// ChatDeliveryConsumer turns chat events into `new_message` frames for the
// connections held by this instance. Every instance runs its own consumer group
// so all of them see every message.
type ChatDeliveryConsumer struct {
	deliverer GroupDeliverer
	logger    *zap.Logger
}

func NewChatDeliveryConsumer(deliverer GroupDeliverer, logger *zap.Logger) *ChatDeliveryConsumer {
	return &ChatDeliveryConsumer{deliverer: deliverer, logger: logger}
}

// HandleEvent delivers one chat event. Unknown events and malformed records are
// skipped so a single bad message cannot stall the delivery loop.
func (c *ChatDeliveryConsumer) HandleEvent(_ context.Context, record kafka.Record) error {
	var event chatEvent
	if err := json.Unmarshal(record.Value, &event); err != nil {
		c.logger.Warn("skipping malformed chat event", zap.String("topic", record.Topic), zap.Error(err))
		return nil
	}
	if event.EventType != ChatMessageSentEvent {
		return nil
	}
	data, err := json.Marshal(event.Payload)
	if err != nil {
		return fmt.Errorf("encode chat message payload: %w", err)
	}
	envelope := realtime.Envelope{
		Type:      realtime.EventNewMessage,
		MessageID: event.Payload.MessageID,
		Timestamp: time.Now().UTC().UnixMilli(),
		TraceID:   event.TraceID,
		Data:      data,
	}
	payload, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("encode new_message frame: %w", err)
	}
	c.deliverer.DeliverToGroup(event.Payload.GroupID, payload)
	return nil
}
