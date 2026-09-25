package health

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestServiceReportsChecks(t *testing.T) {
	service := New(time.Second,
		CheckFunc{CheckName: "mysql", Fn: func(context.Context) error { return nil }},
		CheckFunc{CheckName: "redis", Fn: func(context.Context) error { return errors.New("offline") }},
	)
	status := service.Check(context.Background())
	if status.Ready || status.Dependencies["mysql"] != "up" || status.Dependencies["redis"] != "down" {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestServiceTimesOut(t *testing.T) {
	service := New(10*time.Millisecond, CheckFunc{CheckName: "kafka", Fn: func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}})
	status := service.Check(context.Background())
	if status.Ready || status.Dependencies["kafka"] != "down" {
		t.Fatalf("unexpected timeout status: %+v", status)
	}
}
