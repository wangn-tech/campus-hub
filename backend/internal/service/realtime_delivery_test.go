package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/wangn-tech/campus-hub/internal/model"
	"github.com/wangn-tech/campus-hub/internal/platform/kafka"
	"github.com/wangn-tech/campus-hub/internal/realtime"
	"go.uber.org/zap"
)

type recordingUserDeliverer struct {
	userID  string
	payload []byte
	calls   int
}

func (d *recordingUserDeliverer) DeliverToUser(userID string, payload []byte) int {
	d.userID, d.payload, d.calls = userID, payload, d.calls+1
	return 1
}

func TestRealtimeDeliveryConsumerDeliversNotification(t *testing.T) {
	deliverer := &recordingUserDeliverer{}
	consumer := NewRealtimeDeliveryConsumer(deliverer, nil, nil, nil, zap.NewNop())
	event, err := newNotificationDeliveryEvent("user-uuid", model.Notification{UUID: "notification-uuid", Type: "registration_approved", Title: "报名已通过", Content: "请查看票据"}, "trace-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := consumer.HandleEvent(context.Background(), kafka.Record{Value: []byte(event.Payload)}); err != nil {
		t.Fatal(err)
	}
	if deliverer.calls != 1 || deliverer.userID != "user-uuid" {
		t.Fatalf("delivery = %+v", deliverer)
	}
	var frame realtime.Envelope
	if err := json.Unmarshal(deliverer.payload, &frame); err != nil {
		t.Fatal(err)
	}
	if frame.Type != realtime.EventNotification || frame.MessageID != "notification-uuid" || frame.TraceID != "trace-1" {
		t.Fatalf("frame = %+v", frame)
	}
	var data map[string]any
	if err := json.Unmarshal(frame.Data, &data); err != nil {
		t.Fatal(err)
	}
	if data["notification_id"] != "notification-uuid" || data["notification_type"] != "registration_approved" {
		t.Fatalf("notification data = %#v", data)
	}
}

func TestRealtimeDeliveryConsumerDeliversRegistrationAndVerification(t *testing.T) {
	cases := []struct {
		name, eventType, payload, wantType, wantUser string
	}{
		{"registration", "registration.approved", `{"activity_id":"activity-uuid","registration_id":"registration-uuid","user_id":"user-uuid","from_status":0,"to_status":1}`, realtime.EventRegistrationStatusChanged, "user-uuid"},
		{"verification", "verification.confirmed", `{"verification_id":"verification-uuid","user_id":"user-uuid","status":3,"message":"认证资料已提交人工审核"}`, realtime.EventVerifyProgress, "user-uuid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			deliverer := &recordingUserDeliverer{}
			consumer := NewRealtimeDeliveryConsumer(deliverer, nil, nil, nil, zap.NewNop())
			record := kafka.Record{Value: []byte(`{"event_id":"event-uuid","event_type":"` + tc.eventType + `","trace_id":"trace-1","payload":` + tc.payload + `}`)}
			if err := consumer.HandleEvent(context.Background(), record); err != nil {
				t.Fatal(err)
			}
			var frame realtime.Envelope
			if err := json.Unmarshal(deliverer.payload, &frame); err != nil {
				t.Fatal(err)
			}
			if deliverer.calls != 1 || deliverer.userID != tc.wantUser || frame.Type != tc.wantType || frame.TraceID != "trace-1" {
				t.Fatalf("delivery=%+v frame=%+v", deliverer, frame)
			}
			if tc.name == "registration" {
				var data map[string]any
				if err := json.Unmarshal(frame.Data, &data); err != nil {
					t.Fatal(err)
				}
				if data["from_status"] != "pending" || data["to_status"] != "approved" || data["message"] != "报名已通过" {
					t.Fatalf("registration data = %#v", data)
				}
			}
		})
	}
}

func TestRealtimeDeliveryGroupUsesPrefixAndInstance(t *testing.T) {
	if got := RealtimeDeliveryGroup("campushub.realtime-delivery", "node-1"); got != "campushub.realtime-delivery.node-1" {
		t.Fatalf("group = %q", got)
	}
}
