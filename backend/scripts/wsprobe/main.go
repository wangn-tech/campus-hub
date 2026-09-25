// Command wsprobe exercises the CampusHub WebSocket endpoint end to end. It is
// used by scripts/ci-integration.sh, which has no WebSocket client of its own:
// the probe authenticates two users, sends a group message with the second one
// and requires the first one to receive the broadcast, then checks the
// idempotent resend, the heartbeat and the membership guard.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	eventAuth        = "auth"
	eventAuthSuccess = "auth_success"
	eventAuthFailed  = "auth_failed"
	eventPing        = "ping"
	eventPong        = "pong"
	eventSendMessage = "send_message"
	eventAck         = "ack"
	eventNewMessage  = "new_message"
	eventMarkRead    = "mark_read"
	eventError       = "error"
)

type envelope struct {
	Type      string          `json:"type"`
	MessageID string          `json:"message_id"`
	Timestamp int64           `json:"timestamp"`
	TraceID   string          `json:"trace_id,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
}

type ackData struct {
	ClientMessageID string `json:"client_message_id"`
	ServerMessageID string `json:"server_message_id"`
}

type newMessageData struct {
	MessageID  string `json:"message_id"`
	GroupID    string `json:"group_id"`
	SenderID   string `json:"sender_id"`
	SenderName string `json:"sender_name"`
	MsgType    uint8  `json:"msg_type"`
	Content    string `json:"content"`
}

type errorData struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type client struct {
	name   string
	socket *websocket.Conn
}

func main() {
	url := flag.String("url", "", "websocket endpoint, for example ws://127.0.0.1:18081/ws")
	token := flag.String("token", "", "access token of the user that must receive the broadcast")
	peerToken := flag.String("peer-token", "", "access token of the user that sends the message")
	outsiderToken := flag.String("outsider-token", "", "optional access token of a non member that must be rejected")
	groupUUID := flag.String("group", "", "activity group uuid")
	text := flag.String("text", "", "message content")
	clientMessageID := flag.String("client-id", "", "unique client message id")
	broadcastTimeout := flag.Duration("timeout", 25*time.Second, "how long to wait for the broadcast")
	flag.Parse()

	if err := run(*url, *token, *peerToken, *outsiderToken, *groupUUID, *text, *clientMessageID, *broadcastTimeout); err != nil {
		fmt.Fprintln(os.Stderr, "wsprobe:", err)
		os.Exit(1)
	}
}

func run(url, token, peerToken, outsiderToken, groupUUID, text, clientMessageID string, broadcastTimeout time.Duration) error {
	if url == "" || token == "" || peerToken == "" || groupUUID == "" || text == "" || clientMessageID == "" {
		return errors.New("-url, -token, -peer-token, -group, -text and -client-id are required")
	}
	receiver, err := dial(url, "receiver")
	if err != nil {
		return err
	}
	defer receiver.socket.Close()
	if err := receiver.authenticate(token); err != nil {
		return err
	}
	sender, err := dial(url, "sender")
	if err != nil {
		return err
	}
	defer sender.socket.Close()
	if err := sender.authenticate(peerToken); err != nil {
		return err
	}

	// The heartbeat keeps the connection alive and proves the pong path.
	if err := sender.expectPong(); err != nil {
		return err
	}

	if err := sender.sendMessage(envelopeUUID(), clientMessageID, groupUUID, text); err != nil {
		return err
	}
	sent, err := sender.expectAck(broadcastTimeout)
	if err != nil {
		return err
	}
	if sent.ClientMessageID != clientMessageID {
		return fmt.Errorf("ack carried client_message_id %q, want %q", sent.ClientMessageID, clientMessageID)
	}

	// The receiver must see the broadcast even though the message was published
	// through Kafka and delivered by the per instance consumer.
	delivered, err := receiver.expectNewMessage(broadcastTimeout)
	if err != nil {
		return err
	}
	if delivered.MessageID != sent.ServerMessageID {
		return fmt.Errorf("broadcast message_id %q, want %q", delivered.MessageID, sent.ServerMessageID)
	}
	if delivered.Content != text || delivered.GroupID != groupUUID {
		return fmt.Errorf("broadcast content %q group %q", delivered.Content, delivered.GroupID)
	}

	// Resending the same client message id must return the original message
	// instead of creating a second one.
	if err := sender.sendMessage(envelopeUUID(), clientMessageID, groupUUID, text); err != nil {
		return err
	}
	resent, err := sender.expectAck(broadcastTimeout)
	if err != nil {
		return err
	}
	if resent.ServerMessageID != sent.ServerMessageID {
		return fmt.Errorf("resend created message %q, want the original %q", resent.ServerMessageID, sent.ServerMessageID)
	}

	// Marking the message read is fire and forget; the integration script checks
	// the stored read pointer.
	if err := receiver.send(envelope{Type: eventMarkRead, MessageID: envelopeUUID(), Timestamp: nowMillis(),
		Data: raw(map[string]any{"group_id": groupUUID, "message_id": sent.ServerMessageID})}); err != nil {
		return err
	}

	if outsiderToken != "" {
		if err := checkOutsiderRejected(url, outsiderToken, groupUUID, text); err != nil {
			return err
		}
	}
	fmt.Printf("ok message_id=%s\n", sent.ServerMessageID)
	return nil
}

// checkOutsiderRejected proves a user outside the group cannot post into it.
func checkOutsiderRejected(url, token, groupUUID, text string) error {
	outsider, err := dial(url, "outsider")
	if err != nil {
		return err
	}
	defer outsider.socket.Close()
	if err := outsider.authenticate(token); err != nil {
		return err
	}
	if err := outsider.sendMessage(envelopeUUID(), "ci-outsider-"+uuid.NewString(), groupUUID, text); err != nil {
		return err
	}
	payload, err := outsider.expectErrorFrame(10 * time.Second)
	if err != nil {
		return err
	}
	if payload.Code != 104403 {
		return fmt.Errorf("outsider got code %d (%s), want 104403", payload.Code, payload.Message)
	}
	return nil
}

func dial(url, name string) (*client, error) {
	header := http.Header{}
	header.Set("Origin", "http://localhost:5173")
	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	socket, response, err := dialer.Dial(url, header)
	if err != nil {
		if response != nil {
			return nil, fmt.Errorf("%s dial %s: %w (status %d)", name, url, err, response.StatusCode)
		}
		return nil, fmt.Errorf("%s dial %s: %w", name, url, err)
	}
	return &client{name: name, socket: socket}, nil
}

func (c *client) authenticate(token string) error {
	if err := c.send(envelope{Type: eventAuth, MessageID: envelopeUUID(), Timestamp: nowMillis(),
		Data: raw(map[string]any{"token": token})}); err != nil {
		return err
	}
	event, _, err := c.expectAny(10*time.Second, eventAuthSuccess, eventAuthFailed)
	if err != nil {
		return err
	}
	if event.Type == eventAuthFailed {
		return fmt.Errorf("%s auth failed: %s", c.name, string(event.Data))
	}
	return nil
}

func (c *client) expectPong() error {
	if err := c.send(envelope{Type: eventPing, MessageID: envelopeUUID(), Timestamp: nowMillis()}); err != nil {
		return err
	}
	_, _, err := c.expect(eventPong, 10*time.Second)
	return err
}

func (c *client) sendMessage(messageID, clientMessageID, groupUUID, text string) error {
	return c.send(envelope{Type: eventSendMessage, MessageID: messageID, Timestamp: nowMillis(),
		Data: raw(map[string]any{"group_id": groupUUID, "msg_type": 1, "content": text, "client_message_id": clientMessageID})})
}

func (c *client) expectAck(timeout time.Duration) (ackData, error) {
	_, data, err := c.expect(eventAck, timeout)
	if err != nil {
		return ackData{}, err
	}
	var payload ackData
	if err := json.Unmarshal(data, &payload); err != nil {
		return ackData{}, fmt.Errorf("decode ack: %w", err)
	}
	return payload, nil
}

func (c *client) expectNewMessage(timeout time.Duration) (newMessageData, error) {
	_, data, err := c.expect(eventNewMessage, timeout)
	if err != nil {
		return newMessageData{}, err
	}
	var payload newMessageData
	if err := json.Unmarshal(data, &payload); err != nil {
		return newMessageData{}, fmt.Errorf("decode new_message: %w", err)
	}
	return payload, nil
}

// expectErrorFrame waits for the server to reject a request. Unlike expect it
// treats the error frame as the expected outcome rather than a failure.
func (c *client) expectErrorFrame(timeout time.Duration) (errorData, error) {
	deadline := time.Now().Add(timeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return errorData{}, fmt.Errorf("%s timed out waiting for an error frame", c.name)
		}
		event, data, err := c.read(remaining)
		if err != nil {
			return errorData{}, err
		}
		if event.Type != eventError {
			continue
		}
		var payload errorData
		if err := json.Unmarshal(data, &payload); err != nil {
			return errorData{}, fmt.Errorf("decode error frame: %w", err)
		}
		return payload, nil
	}
}

// expect waits for one specific event, surfacing any error frame the server
// sent instead.
func (c *client) expect(want string, timeout time.Duration) (envelope, json.RawMessage, error) {
	deadline := time.Now().Add(timeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return envelope{}, nil, fmt.Errorf("%s timed out waiting for %s", c.name, want)
		}
		event, data, err := c.read(remaining)
		if err != nil {
			return envelope{}, nil, err
		}
		if event.Type == eventError {
			var payload errorData
			_ = json.Unmarshal(data, &payload)
			return envelope{}, nil, fmt.Errorf("%s received error frame %d: %s", c.name, payload.Code, payload.Message)
		}
		if event.Type == want {
			return event, data, nil
		}
	}
}

func (c *client) expectAny(timeout time.Duration, want ...string) (envelope, json.RawMessage, error) {
	wanted := make(map[string]struct{}, len(want))
	for _, name := range want {
		wanted[name] = struct{}{}
	}
	deadline := time.Now().Add(timeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return envelope{}, nil, fmt.Errorf("%s timed out waiting for %v", c.name, want)
		}
		event, data, err := c.read(remaining)
		if err != nil {
			return envelope{}, nil, err
		}
		if _, ok := wanted[event.Type]; ok {
			return event, data, nil
		}
	}
}

func (c *client) read(timeout time.Duration) (envelope, json.RawMessage, error) {
	if err := c.socket.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return envelope{}, nil, err
	}
	_, payload, err := c.socket.ReadMessage()
	if err != nil {
		return envelope{}, nil, fmt.Errorf("%s read: %w", c.name, err)
	}
	var event envelope
	if err := json.Unmarshal(payload, &event); err != nil {
		return envelope{}, nil, fmt.Errorf("%s decode %s: %w", c.name, string(payload), err)
	}
	return event, event.Data, nil
}

func (c *client) send(event envelope) error {
	if err := c.socket.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	return c.socket.WriteJSON(event)
}

func raw(value any) json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return encoded
}

func envelopeUUID() string { return uuid.NewString() }

func nowMillis() int64 { return time.Now().UTC().UnixMilli() }
