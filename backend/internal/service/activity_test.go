package service

import (
	"testing"
	"time"

	"github.com/wangn-tech/campus-hub/internal/model"
)

func TestNormalizePage(t *testing.T) {
	cases := []struct {
		page, pageSize         int
		wantPage, wantPageSize int
	}{
		{page: 0, pageSize: 0, wantPage: 1, wantPageSize: 10},
		{page: -3, pageSize: -1, wantPage: 1, wantPageSize: 10},
		{page: 2, pageSize: 12, wantPage: 2, wantPageSize: 12},
		{page: 5, pageSize: 500, wantPage: 5, wantPageSize: 50},
	}
	for _, tc := range cases {
		page, pageSize := NormalizePage(tc.page, tc.pageSize)
		if page != tc.wantPage || pageSize != tc.wantPageSize {
			t.Fatalf("NormalizePage(%d, %d) = (%d, %d)", tc.page, tc.pageSize, page, pageSize)
		}
	}
}

func TestRequestedPublicStatuses(t *testing.T) {
	statuses, ok := requestedPublicStatuses(nil)
	if !ok || len(statuses) != 3 {
		t.Fatalf("nil status should default to the public set, got %v ok=%v", statuses, ok)
	}
	published := int(2)
	statuses, ok = requestedPublicStatuses(&published)
	if !ok || len(statuses) != 1 || statuses[0] != 2 {
		t.Fatalf("published filter not applied: %v ok=%v", statuses, ok)
	}
	draft := 0
	if _, ok := requestedPublicStatuses(&draft); ok {
		t.Fatal("draft must not be requestable on a public list")
	}
}

func TestIsPublicActivityStatus(t *testing.T) {
	for _, public := range []uint8{2, 3, 4} {
		if !isPublicActivityStatus(public) {
			t.Fatalf("status %d should be public", public)
		}
	}
	for _, private := range []uint8{0, 1, 5, 6} {
		if isPublicActivityStatus(private) {
			t.Fatalf("status %d should not be public", private)
		}
	}
}

func TestValidActivityWindow(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	registerStart := base
	registerEnd := base.Add(time.Hour)
	activityStart := base.Add(2 * time.Hour)
	activityEnd := base.Add(3 * time.Hour)
	if !validActivityWindow(registerStart, registerEnd, activityStart, activityEnd) {
		t.Fatal("a coherent window should be accepted")
	}
	if validActivityWindow(registerEnd, registerStart, activityStart, activityEnd) {
		t.Fatal("registration start after end must be rejected")
	}
	if validActivityWindow(registerStart, activityStart.Add(time.Hour), activityStart, activityEnd) {
		t.Fatal("registration closing after the activity starts must be rejected")
	}
	if validActivityWindow(registerStart, registerEnd, activityEnd, activityStart) {
		t.Fatal("activity start after end must be rejected")
	}
	if validActivityWindow(time.Time{}, registerEnd, activityStart, activityEnd) {
		t.Fatal("missing timestamps must be rejected")
	}
}

func TestValidCoordinate(t *testing.T) {
	outOfRange := 181.0
	if validCoordinate(&outOfRange, 180) {
		t.Fatal("longitude above 180 must be rejected")
	}
	ok := -120.5
	if !validCoordinate(&ok, 180) || !validCoordinate(nil, 180) {
		t.Fatal("valid coordinate handling failed")
	}
}

func TestUniqueHelpers(t *testing.T) {
	got := uniqueStrings([]string{"a", " a ", "", "b", "b"})
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("uniqueStrings = %v", got)
	}
	ids := uniqueUint64([]uint64{1, 2, 1, 3})
	if len(ids) != 3 {
		t.Fatalf("uniqueUint64 = %v", ids)
	}
}

func TestActivityStatusRules(t *testing.T) {
	cases := []struct {
		name        string
		status      model.ActivityStatus
		editable    bool
		cancellable bool
		reviewable  bool
	}{
		{name: "draft", status: model.ActivityDraft, editable: true, cancellable: true, reviewable: false},
		{name: "pending review", status: model.ActivityPendingReview, editable: false, cancellable: true, reviewable: true},
		{name: "published", status: model.ActivityPublished, editable: false, cancellable: true, reviewable: false},
		{name: "ongoing", status: model.ActivityOngoing, editable: false, cancellable: true, reviewable: false},
		{name: "finished", status: model.ActivityFinished, editable: false, cancellable: false, reviewable: false},
		{name: "rejected", status: model.ActivityRejected, editable: true, cancellable: false, reviewable: false},
		{name: "cancelled", status: model.ActivityCancelled, editable: false, cancellable: false, reviewable: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status := uint8(tc.status)
			if got := isEditableActivityStatus(status); got != tc.editable {
				t.Fatalf("isEditableActivityStatus(%d) = %v, want %v", status, got, tc.editable)
			}
			if got := canCancelActivity(status); got != tc.cancellable {
				t.Fatalf("canCancelActivity(%d) = %v, want %v", status, got, tc.cancellable)
			}
			if got := canReviewActivity(status); got != tc.reviewable {
				t.Fatalf("canReviewActivity(%d) = %v, want %v", status, got, tc.reviewable)
			}
		})
	}
}
