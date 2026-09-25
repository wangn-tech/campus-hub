package service

import "testing"

func TestClampMessageLimit(t *testing.T) {
	cases := []struct {
		in   int
		want int
	}{
		{in: 0, want: chatMessageDefaultLimit},
		{in: -5, want: chatMessageDefaultLimit},
		{in: 1, want: 1},
		{in: 60, want: 60},
		{in: chatMessageMaxLimit, want: chatMessageMaxLimit},
		{in: chatMessageMaxLimit + 1, want: chatMessageMaxLimit},
	}
	for _, tc := range cases {
		if got := clampMessageLimit(tc.in); got != tc.want {
			t.Fatalf("clampMessageLimit(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestChatMembershipActionFor(t *testing.T) {
	cases := []struct {
		eventType string
		want      chatMembershipAction
	}{
		{eventType: "registration.approved", want: chatMembershipJoin},
		{eventType: "registration.cancelled", want: chatMembershipLeave},
		{eventType: "registration.rejected", want: chatMembershipLeave},
		{eventType: "registration.expired", want: chatMembershipLeave},
		{eventType: "registration.created", want: chatMembershipIgnore},
		{eventType: "ticket.created", want: chatMembershipIgnore},
		{eventType: "", want: chatMembershipIgnore},
	}
	for _, tc := range cases {
		if got := chatMembershipActionFor(tc.eventType); got != tc.want {
			t.Fatalf("chatMembershipActionFor(%q) = %d, want %d", tc.eventType, got, tc.want)
		}
	}
}
