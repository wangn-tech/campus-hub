package service

import "testing"

func TestEventTopic(t *testing.T) {
	cases := map[string]string{
		"activity.created":        "campushub.activity.events.v1",
		"activity.status_changed": "campushub.activity.events.v1",
		"registration.created":    "campushub.registration.events.v1",
		"registration.expired":    "campushub.registration.events.v1",
		"ticket.created":          "campushub.ticket.events.v1",
		"ticket.used":             "campushub.ticket.events.v1",
		"notification.created":    "campushub.notification.events.v1",
		"chat.message_sent":       "campushub.chat.events.v1",
		"file.uploaded":           "campushub.file.events.v1",
		"system.audit_log":        "campushub.system.audit.v1",
		"something.unknown":       "campushub.system.audit.v1",
	}
	for eventType, want := range cases {
		if got := eventTopic(eventType); got != want {
			t.Fatalf("eventTopic(%q) = %q, want %q", eventType, got, want)
		}
	}
}

func TestRelayTopicsCoverEventTopic(t *testing.T) {
	known := make(map[string]bool, len(relayTopics))
	for _, topic := range relayTopics {
		known[topic] = true
	}
	for _, eventType := range []string{
		"activity.created", "registration.created", "ticket.created",
		"notification.created", "chat.message_sent", "file.uploaded",
		"system.audit_log", "unknown.thing",
	} {
		if topic := eventTopic(eventType); !known[topic] {
			t.Fatalf("eventTopic(%q) = %q is missing from relayTopics", eventType, topic)
		}
	}
}
