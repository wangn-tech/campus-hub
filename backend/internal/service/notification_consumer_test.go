package service

import "testing"

func TestNotificationKeyIsStablePerEventAndRecipient(t *testing.T) {
	first := notificationKey("11111111-1111-1111-1111-111111111111", 7)
	if len(first) != 36 {
		t.Fatalf("expected a UUID, got %q", first)
	}
	if first != notificationKey("11111111-1111-1111-1111-111111111111", 7) {
		t.Fatal("notification key must be deterministic")
	}
	if first == notificationKey("11111111-1111-1111-1111-111111111111", 8) {
		t.Fatal("different recipients must map to different keys")
	}
	if first == notificationKey("22222222-2222-2222-2222-222222222222", 7) {
		t.Fatal("different events must map to different keys")
	}
}
