package service

import (
	"testing"
	"time"

	"github.com/wangn-tech/campus-hub/internal/model"
)

func TestIsRegisterableActivityStatus(t *testing.T) {
	cases := []struct {
		status model.ActivityStatus
		want   bool
	}{
		{status: model.ActivityDraft, want: false},
		{status: model.ActivityPendingReview, want: false},
		{status: model.ActivityPublished, want: true},
		{status: model.ActivityOngoing, want: true},
		{status: model.ActivityFinished, want: false},
		{status: model.ActivityRejected, want: false},
		{status: model.ActivityCancelled, want: false},
	}
	for _, tc := range cases {
		if got := isRegisterableActivityStatus(uint8(tc.status)); got != tc.want {
			t.Fatalf("isRegisterableActivityStatus(%d) = %v, want %v", tc.status, got, tc.want)
		}
	}
}

func TestCanCancelRegistration(t *testing.T) {
	cases := []struct {
		status model.RegistrationStatus
		want   bool
	}{
		{status: model.RegistrationPending, want: true},
		{status: model.RegistrationApproved, want: true},
		{status: model.RegistrationRejected, want: false},
		{status: model.RegistrationCancelled, want: false},
		{status: model.RegistrationFailed, want: false},
		{status: model.RegistrationExpired, want: false},
	}
	for _, tc := range cases {
		if got := canCancelRegistration(uint8(tc.status)); got != tc.want {
			t.Fatalf("canCancelRegistration(%d) = %v, want %v", tc.status, got, tc.want)
		}
	}
}

func TestApprovalDeadline(t *testing.T) {
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

	// The activity starts after the approval window, so the window wins.
	far := now.Add(7 * 24 * time.Hour)
	if got := approvalDeadline(now, far); !got.Equal(now.Add(registrationApprovalWindow)) {
		t.Fatalf("approvalDeadline = %s, want %s", got, now.Add(registrationApprovalWindow))
	}

	// The activity starts sooner than the window, so it caps the deadline.
	soon := now.Add(3 * time.Hour)
	if got := approvalDeadline(now, soon); !got.Equal(soon) {
		t.Fatalf("approvalDeadline = %s, want %s", got, soon)
	}
}

func TestRegistrationStatusFilter(t *testing.T) {
	cases := []struct {
		in   string
		want int
		ok   bool
	}{
		{in: "", want: -1, ok: true},
		{in: "pending", want: int(model.RegistrationPending), ok: true},
		{in: "approved", want: int(model.RegistrationApproved), ok: true},
		{in: "expired", want: int(model.RegistrationExpired), ok: true},
		{in: "bogus", ok: false},
	}
	for _, tc := range cases {
		statuses, ok := registrationStatusFilter(tc.in)
		if ok != tc.ok {
			t.Fatalf("registrationStatusFilter(%q) ok = %v, want %v", tc.in, ok, tc.ok)
		}
		if !ok {
			continue
		}
		if tc.want < 0 {
			if statuses != nil {
				t.Fatalf("empty filter should mean all statuses, got %v", statuses)
			}
			continue
		}
		if len(statuses) != 1 || statuses[0] != tc.want {
			t.Fatalf("registrationStatusFilter(%q) = %v, want [%d]", tc.in, statuses, tc.want)
		}
	}
}

func TestReviewOperatorType(t *testing.T) {
	activity := &model.Activity{OrganizerID: 7}
	if got := reviewOperatorType(activity, &model.User{ID: 7}); got != model.RegistrationOperatorOrganizer {
		t.Fatalf("organizer action recorded as %d", got)
	}
	if got := reviewOperatorType(activity, &model.User{ID: 9}); got != model.RegistrationOperatorAdmin {
		t.Fatalf("admin action recorded as %d", got)
	}
}
