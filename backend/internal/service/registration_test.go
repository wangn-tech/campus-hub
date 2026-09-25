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
