package timestamp

import (
	"encoding/json"
	"testing"
	"time"
)

func TestMillisRoundTrip(t *testing.T) {
	want := time.Date(2026, time.January, 1, 2, 0, 0, 123000000, time.UTC)
	got := FromMillis(ToMillis(want))
	if !got.Equal(want) {
		t.Fatalf("round trip mismatch: got %s want %s", got, want)
	}
}

func TestParseRejectsNegative(t *testing.T) {
	if _, err := Parse(-1); err == nil {
		t.Fatal("expected negative timestamp error")
	}
}

func TestParseZeroIsNullTime(t *testing.T) {
	got, err := Parse(0)
	if err != nil {
		t.Fatalf("parse zero: %v", err)
	}
	if !got.IsZero() {
		t.Fatalf("expected zero time, got %s", got)
	}
}

func TestMillisJSON(t *testing.T) {
	value := Millis(time.UnixMilli(1767232800123).UTC())
	encoded, err := json.Marshal(value)
	if err != nil || string(encoded) != "1767232800123" {
		t.Fatalf("marshal: %s, %v", encoded, err)
	}
	var decoded Millis
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if ToMillis(decoded.Time()) != 1767232800123 {
		t.Fatalf("decoded millis: %d", ToMillis(decoded.Time()))
	}
	if ToMillis(time.Time{}) != 0 {
		t.Fatal("zero time must serialize as zero millis")
	}
}
