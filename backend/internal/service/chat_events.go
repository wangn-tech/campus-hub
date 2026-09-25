package service

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ChatMessageSentEvent is the Kafka event published when a group message has
// been persisted. The per instance delivery consumers turn it into the
// `new_message` WebSocket frame, which is why the payload already carries the
// rendered message (see the WebSocket design document section 8.2).
const ChatMessageSentEvent = "chat.message_sent"

const chatEventsTopic = "campushub.chat.events.v1"

// ChatTopics lists the topics the chat delivery consumer subscribes to.
func ChatTopics() []string { return []string{chatEventsTopic} }

// ChatDeliveryGroup builds the per instance consumer group so every instance
// receives every chat event and can deliver it to its own connections.
func ChatDeliveryGroup(prefix, instanceID string) string {
	return prefix + "." + instanceID
}

// chatMessagePayload is the `new_message` data shared by the publisher and the
// delivery consumer.
type chatMessagePayload struct {
	MessageID    string `json:"message_id"`
	GroupID      string `json:"group_id"`
	SenderID     string `json:"sender_id"`
	SenderName   string `json:"sender_name"`
	SenderAvatar string `json:"sender_avatar"`
	MsgType      uint8  `json:"msg_type"`
	Content      string `json:"content"`
	ImageURL     string `json:"image_url,omitempty"`
	CreatedAt    int64  `json:"created_at"`
}

// chatEvent is the Kafka envelope from the Kafka design document section 5.
type chatEvent struct {
	EventID       string             `json:"event_id"`
	EventType     string             `json:"event_type"`
	EventVersion  int                `json:"event_version"`
	EventTime     int64              `json:"event_time"`
	AggregateType string             `json:"aggregate_type"`
	AggregateID   string             `json:"aggregate_id"`
	TraceID       string             `json:"trace_id,omitempty"`
	Producer      string             `json:"producer"`
	Payload       chatMessagePayload `json:"payload"`
}

func newChatMessageSentEvent(payload chatMessagePayload, traceID string) ([]byte, error) {
	return json.Marshal(chatEvent{
		EventID:       uuid.NewString(),
		EventType:     ChatMessageSentEvent,
		EventVersion:  1,
		EventTime:     time.Now().UTC().UnixMilli(),
		AggregateType: "chat_message",
		AggregateID:   payload.MessageID,
		TraceID:       traceID,
		Producer:      outboxProducer,
		Payload:       payload,
	})
}
