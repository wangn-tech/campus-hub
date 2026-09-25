package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/wangn-tech/campus-hub/internal/platform/kafka"
	"github.com/wangn-tech/campus-hub/internal/realtime"
	"go.uber.org/zap"
)

type recordingDeliverer struct {
	groupUUID string
	payload   []byte
	calls     int
}

func (d *recordingDeliverer) DeliverToGroup(groupUUID string, payload []byte) int {
	d.groupUUID = groupUUID
	d.payload = payload
	d.calls++
	return 1
}

func TestChatDeliveryConsumerBuildsNewMessageFrame(t *testing.T) {
	payload := chatMessagePayload{
		MessageID:  "message-uuid",
		GroupID:    "group-uuid",
		SenderID:   "sender-uuid",
		SenderName: "张三",
		MsgType:    1,
		Content:    "你好",
		CreatedAt:  1700000000000,
	}
	event, err := newChatMessageSentEvent(payload, "trace-1")
	if err != nil {
		t.Fatalf("newChatMessageSentEvent: %v", err)
	}
	deliverer := &recordingDeliverer{}
	consumer := NewChatDeliveryConsumer(deliverer, zap.NewNop())
	if err := consumer.HandleEvent(context.Background(), kafka.Record{Value: event}); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if deliverer.calls != 1 || deliverer.groupUUID != "group-uuid" {
		t.Fatalf("delivered %d times to %q", deliverer.calls, deliverer.groupUUID)
	}
	var envelope realtime.Envelope
	if err := json.Unmarshal(deliverer.payload, &envelope); err != nil {
		t.Fatalf("decode frame: %v", err)
	}
	if envelope.Type != realtime.EventNewMessage || envelope.MessageID != "message-uuid" || envelope.TraceID != "trace-1" {
		t.Fatalf("unexpected envelope %+v", envelope)
	}
	var data chatMessagePayload
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		t.Fatalf("decode frame data: %v", err)
	}
	if data.Content != "你好" || data.SenderName != "张三" || data.GroupID != "group-uuid" {
		t.Fatalf("unexpected frame data %+v", data)
	}
}

func TestChatDeliveryConsumerSkipsOtherEvents(t *testing.T) {
	deliverer := &recordingDeliverer{}
	consumer := NewChatDeliveryConsumer(deliverer, zap.NewNop())
	record := kafka.Record{Value: []byte(`{"event_type":"chat.message_recalled","payload":{"group_id":"g","message_id":"m"}}`)}
	if err := consumer.HandleEvent(context.Background(), record); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
	if deliverer.calls != 0 {
		t.Fatalf("unexpected delivery for %d calls", deliverer.calls)
	}
}

func TestChatDeliveryConsumerSkipsMalformedEvents(t *testing.T) {
	deliverer := &recordingDeliverer{}
	consumer := NewChatDeliveryConsumer(deliverer, zap.NewNop())
	if err := consumer.HandleEvent(context.Background(), kafka.Record{Value: []byte("not json")}); err != nil {
		t.Fatalf("malformed events must be skipped, got %v", err)
	}
	if deliverer.calls != 0 {
		t.Fatalf("unexpected delivery for %d calls", deliverer.calls)
	}
}

func TestChatDeliveryGroupUsesPrefixAndInstance(t *testing.T) {
	if got := ChatDeliveryGroup("campushub.chat-delivery", "node-1"); got != "campushub.chat-delivery.node-1" {
		t.Fatalf("ChatDeliveryGroup = %q", got)
	}
}
