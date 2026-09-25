package realtime

import (
	"context"
	"encoding/json"
	"testing"
)

func newTestConnection(userUUID string) *Connection {
	conn := NewConnection(nil)
	conn.UserUUID = userUUID
	return conn
}

func drain(t *testing.T, conn *Connection) Envelope {
	t.Helper()
	select {
	case payload := <-conn.send:
		var envelope Envelope
		if err := json.Unmarshal(payload, &envelope); err != nil {
			t.Fatalf("decode envelope: %v", err)
		}
		return envelope
	default:
		t.Fatal("expected a queued frame")
		return Envelope{}
	}
}

func TestHubDeliversToGroupMembers(t *testing.T) {
	ctx := context.Background()
	hub := NewHub(nil, nil)
	member := newTestConnection("user-1")
	outsider := newTestConnection("user-2")
	hub.Register(ctx, member)
	hub.Register(ctx, outsider)
	hub.Join("group-1", member)

	if delivered := hub.DeliverToGroup("group-1", []byte(`{"type":"new_message"}`)); delivered != 1 {
		t.Fatalf("DeliverToGroup delivered %d, want 1", delivered)
	}
	if envelope := drain(t, member); envelope.Type != EventNewMessage {
		t.Fatalf("envelope type = %q", envelope.Type)
	}
	if delivered := hub.DeliverToGroup("group-2", []byte(`{}`)); delivered != 0 {
		t.Fatalf("DeliverToGroup to an empty room delivered %d, want 0", delivered)
	}
	if hub.GroupMemberCount("group-1") != 1 {
		t.Fatalf("GroupMemberCount = %d, want 1", hub.GroupMemberCount("group-1"))
	}
	// A connection that never joined the room must not receive the frame.
	select {
	case <-outsider.send:
		t.Fatal("outsider received a group frame")
	default:
	}
}

func TestHubUnregisterLeavesRooms(t *testing.T) {
	ctx := context.Background()
	hub := NewHub(nil, nil)
	member := newTestConnection("user-1")
	hub.Register(ctx, member)
	hub.JoinGroups([]string{"group-1", "group-2"}, member)
	hub.Unregister(ctx, member)

	for _, groupUUID := range []string{"group-1", "group-2"} {
		if delivered := hub.DeliverToGroup(groupUUID, []byte(`{}`)); delivered != 0 {
			t.Fatalf("unregistered connection still received %d frames for %s", delivered, groupUUID)
		}
	}
	if delivered := hub.DeliverToUser("user-1", []byte(`{}`)); delivered != 0 {
		t.Fatalf("unregistered user still received %d frames", delivered)
	}
}

func TestHubDeliversToEveryUserConnection(t *testing.T) {
	ctx := context.Background()
	hub := NewHub(nil, nil)
	first := newTestConnection("user-1")
	second := newTestConnection("user-1")
	hub.Register(ctx, first)
	hub.Register(ctx, second)

	if delivered := hub.DeliverToUser("user-1", []byte(`{"type":"notification"}`)); delivered != 2 {
		t.Fatalf("DeliverToUser delivered %d, want 2", delivered)
	}
	if envelope := drain(t, first); envelope.Type != EventNotification {
		t.Fatalf("envelope type = %q", envelope.Type)
	}
	if envelope := drain(t, second); envelope.Type != EventNotification {
		t.Fatalf("envelope type = %q", envelope.Type)
	}
}

func TestHubDropsSlowPeer(t *testing.T) {
	ctx := context.Background()
	hub := NewHub(nil, nil)
	slow := newTestConnection("user-1")
	hub.Register(ctx, slow)
	hub.Join("group-1", slow)
	// Fill the outbound queue; the next delivery must drop the peer instead of
	// blocking the broadcaster.
	for i := 0; i < writeQueueSize; i++ {
		slow.send <- []byte(`{}`)
	}
	if delivered := hub.DeliverToGroup("group-1", []byte(`{}`)); delivered != 0 {
		t.Fatalf("slow peer counted as delivered: %d", delivered)
	}
	select {
	case <-slow.Done():
	default:
		t.Fatal("slow peer was not closed")
	}
}

func TestNewEnvelopeCarriesData(t *testing.T) {
	envelope, err := NewEnvelope(EventAck, "message-1", 1700000000000, map[string]string{"server_message_id": "abc"})
	if err != nil {
		t.Fatalf("NewEnvelope: %v", err)
	}
	if envelope.Type != EventAck || envelope.MessageID != "message-1" || envelope.Timestamp != 1700000000000 {
		t.Fatalf("unexpected envelope %+v", envelope)
	}
	var data map[string]string
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		t.Fatalf("decode data: %v", err)
	}
	if data["server_message_id"] != "abc" {
		t.Fatalf("data = %v", data)
	}
}
