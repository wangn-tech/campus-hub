package repository

import (
	"testing"
	"time"
)

func TestNextRetryAt(t *testing.T) {
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

	if got := nextRetryAt(now, 1); got == nil || !got.Equal(now.Add(2*time.Second)) {
		t.Fatalf("first retry = %v, want %v", got, now.Add(2*time.Second))
	}
	if got := nextRetryAt(now, 3); got == nil || !got.Equal(now.Add(8*time.Second)) {
		t.Fatalf("third retry = %v, want %v", got, now.Add(8*time.Second))
	}
	if got := nextRetryAt(now, 9); got == nil || !got.Equal(now.Add(outboxMaxBackoff)) {
		t.Fatalf("backoff should be capped at %s, got %v", outboxMaxBackoff, got)
	}
	if got := nextRetryAt(now, outboxMaxAttempts); got != nil {
		t.Fatalf("exhausted attempts should park the event, got %v", got)
	}
}
