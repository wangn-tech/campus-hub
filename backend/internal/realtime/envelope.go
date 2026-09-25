// Package realtime holds the WebSocket transport primitives described in the
// WebSocket protocol design document: the event envelope, the connection hub
// that maps users and groups to live connections, and online presence.
//
// The package deliberately knows nothing about business services so both the
// HTTP/WebSocket handler and the Kafka delivery consumers can share it without
// introducing an import cycle.
package realtime

import "encoding/json"

// Event names from the WebSocket design document section 9. The names are part
// of the client contract and must stay snake_case.
const (
	EventAuth                      = "auth"
	EventAuthSuccess               = "auth_success"
	EventAuthFailed                = "auth_failed"
	EventPing                      = "ping"
	EventPong                      = "pong"
	EventSendMessage               = "send_message"
	EventAck                       = "ack"
	EventNewMessage                = "new_message"
	EventMarkRead                  = "mark_read"
	EventError                     = "error"
	EventNotification              = "notification"
	EventVerifyProgress            = "verify_progress"
	EventRegistrationStatusChanged = "registration_status_changed"
)

// Envelope is the shared message frame: every inbound and outbound WebSocket
// message uses this shape.
type Envelope struct {
	Type      string          `json:"type"`
	MessageID string          `json:"message_id"`
	Timestamp int64           `json:"timestamp"`
	TraceID   string          `json:"trace_id,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
}

// NewEnvelope builds an outbound envelope with the current Unix millisecond
// timestamp already filled in.
func NewEnvelope(eventType, messageID string, now int64, data any) (Envelope, error) {
	envelope := Envelope{Type: eventType, MessageID: messageID, Timestamp: now}
	if data == nil {
		return envelope, nil
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return Envelope{}, err
	}
	envelope.Data = raw
	return envelope, nil
}
