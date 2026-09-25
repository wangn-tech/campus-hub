package service

import (
	"testing"
	"time"

	"github.com/wangn-tech/campus-hub/internal/model"
)

func TestCheckInAllowed(t *testing.T) {
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	start := now.Add(-time.Hour)
	end := now.Add(time.Hour)
	window := func(status model.TicketStatus, validStart, validEnd time.Time) *model.Ticket {
		return &model.Ticket{Status: uint8(status), ValidStartAt: &validStart, ValidEndAt: &validEnd}
	}
	activity := func(status model.ActivityStatus) *model.Activity {
		return &model.Activity{Status: uint8(status)}
	}

	cases := []struct {
		name     string
		ticket   *model.Ticket
		activity *model.Activity
		wantErr  bool
	}{
		{name: "inside the window", ticket: window(model.TicketUnused, start, end), activity: activity(model.ActivityPublished)},
		{name: "already used", ticket: window(model.TicketUsed, start, end), activity: activity(model.ActivityPublished), wantErr: true},
		{name: "voided", ticket: window(model.TicketVoided, start, end), activity: activity(model.ActivityPublished), wantErr: true},
		{name: "expired ticket", ticket: window(model.TicketExpired, start, end), activity: activity(model.ActivityPublished), wantErr: true},
		{name: "window not open yet", ticket: window(model.TicketUnused, now.Add(time.Hour), end), activity: activity(model.ActivityPublished), wantErr: true},
		{name: "window closed", ticket: window(model.TicketUnused, start, now.Add(-time.Minute)), activity: activity(model.ActivityPublished), wantErr: true},
		{name: "cancelled activity", ticket: window(model.TicketUnused, start, end), activity: activity(model.ActivityCancelled), wantErr: true},
		{name: "finished activity inside window", ticket: window(model.TicketUnused, start, end), activity: activity(model.ActivityFinished)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkInAllowed(tc.ticket, tc.activity, now)
			if (err != nil) != tc.wantErr {
				t.Fatalf("checkInAllowed = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
