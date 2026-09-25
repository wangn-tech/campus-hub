package repository

import (
	"testing"
	"time"

	"github.com/wangn-tech/campus-hub/internal/model"
)

func TestNextTimedStatus(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		current model.ActivityStatus
		start   time.Time
		end     time.Time
		now     time.Time
		want    model.ActivityStatus
		ok      bool
	}{
		{name: "published before start", current: model.ActivityPublished, start: base.Add(time.Hour), end: base.Add(2 * time.Hour), now: base, ok: false},
		{name: "published after start", current: model.ActivityPublished, start: base.Add(-time.Hour), end: base.Add(time.Hour), now: base, want: model.ActivityOngoing, ok: true},
		{name: "published after end", current: model.ActivityPublished, start: base.Add(-2 * time.Hour), end: base.Add(-time.Hour), now: base, want: model.ActivityFinished, ok: true},
		{name: "ongoing after end", current: model.ActivityOngoing, start: base.Add(-2 * time.Hour), end: base.Add(-time.Minute), now: base, want: model.ActivityFinished, ok: true},
		{name: "ongoing still running", current: model.ActivityOngoing, start: base.Add(-time.Hour), end: base.Add(time.Hour), now: base, ok: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := nextTimedStatus(tc.current, tc.start, tc.end, tc.now)
			if ok != tc.ok || (ok && got != tc.want) {
				t.Fatalf("nextTimedStatus = (%v, %v), want (%v, %v)", got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestActivityOrderIsWhitelisted(t *testing.T) {
	if got := activityOrder("start_time"); got != "activities.activity_start_at ASC, activities.id ASC" {
		t.Fatalf("default order = %q", got)
	}
	if got := activityOrder("hot"); got != "activities.view_count DESC, activities.id DESC" {
		t.Fatalf("hot order = %q", got)
	}
	if got := activityOrder("created_at"); got != "activities.created_at DESC, activities.id DESC" {
		t.Fatalf("created_at order = %q", got)
	}
	// An unknown value must fall back to the default instead of reaching SQL.
	if got := activityOrder("title; DROP TABLE activities"); got != "activities.activity_start_at ASC, activities.id ASC" {
		t.Fatalf("unexpected fallback order = %q", got)
	}
}
