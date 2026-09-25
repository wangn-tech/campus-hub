package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wangn-tech/campus-hub/internal/handler"
	"github.com/wangn-tech/campus-hub/internal/health"
	"github.com/wangn-tech/campus-hub/internal/service"
	"go.uber.org/zap"
)

func TestHealthAndReady(t *testing.T) {
	r := newTestRouter(nil)
	for _, path := range []string{"/health", "/ready"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		r.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status: got %d", path, recorder.Code)
		}
		if recorder.Header().Get("X-Request-ID") == "" {
			t.Fatalf("%s missing request id", path)
		}
	}
}

func TestReadyReportsDependencyFailure(t *testing.T) {
	r := newTestRouter(map[string]error{"redis": context.DeadlineExceeded})
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("ready status: got %d", recorder.Code)
	}
	body := recorder.Body.String()
	for _, expected := range []string{"service not ready", "redis", "down", "mysql", "kafka", "elasticsearch"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("response missing %q: %s", expected, body)
		}
	}
}

func TestCORSPreflightUsesConfiguredOrigin(t *testing.T) {
	r := newTestRouter(nil)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodOptions, "/health", nil)
	request.Header.Set("Origin", "http://localhost:5173")
	r.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent || recorder.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Fatalf("unexpected cors response: status=%d origin=%q", recorder.Code, recorder.Header().Get("Access-Control-Allow-Origin"))
	}
}

func newTestRouter(failures map[string]error) *gin.Engine {
	checkers := make([]health.Checker, 0, 4)
	for _, name := range []string{"mysql", "redis", "kafka", "elasticsearch"} {
		name := name
		checkers = append(checkers, health.CheckFunc{CheckName: name, Fn: func(context.Context) error { return failures[name] }})
	}
	return New(Dependencies{Readiness: health.New(time.Second, checkers...), Logger: zap.NewNop(), AllowedOrigins: []string{"http://localhost:5173"}})
}

type stubAuthenticator struct{}

func (stubAuthenticator) Authenticate(context.Context, string) (string, error) {
	return "user-uuid", nil
}

type stubAdminChecker struct{}

func (stubAdminChecker) IsAdmin(context.Context, string) (bool, error) { return false, nil }

func TestActivityRoutesAreRegistered(t *testing.T) {
	activityService := service.NewActivityService(nil, nil, nil, nil, nil, nil)
	registrationService := service.NewRegistrationService(nil, nil, nil, nil, nil, nil, nil)
	checkInService := service.NewCheckInService(nil, nil, nil, nil)
	notificationService := service.NewNotificationService(nil)
	chatService := service.NewChatService(nil, nil, nil, nil, nil)
	userService := service.NewUserService(nil, nil, nil)
	engine := New(Dependencies{
		AuthHandler:         handler.NewAuthHandler(nil),
		UserHandler:         handler.NewUserHandler(userService),
		FileHandler:         handler.NewFileHandler(nil, userService),
		VerificationHandler: handler.NewVerificationHandler(nil, userService),
		ActivityHandler:     handler.NewActivityHandler(activityService, userService),
		RegistrationHandler: handler.NewRegistrationHandler(registrationService, userService),
		CheckInHandler:      handler.NewCheckInHandler(checkInService, userService),
		NotificationHandler: handler.NewNotificationHandler(notificationService, userService),
		ChatHandler:         handler.NewChatHandler(chatService, userService),
		Authenticator:       stubAuthenticator{},
		AdminChecker:        stubAdminChecker{},
		Readiness:           health.New(time.Second),
		Logger:              zap.NewNop(),
		AllowedOrigins:      []string{"http://localhost:5173"},
	})
	registered := make(map[string]bool)
	for _, route := range engine.Routes() {
		registered[route.Method+" "+route.Path] = true
	}
	for _, want := range []string{
		"GET /api/v1/categories",
		"GET /api/v1/tags",
		"GET /api/v1/activities",
		"GET /api/v1/activities/search",
		"GET /api/v1/activities/:id",
		"POST /api/v1/activities",
		"PUT /api/v1/activities/:id",
		"POST /api/v1/activities/:id/submit",
		"POST /api/v1/activities/:id/cancel",
		"POST /api/v1/activities/:id/registrations",
		"GET /api/v1/activities/:id/registrations",
		"GET /api/v1/registrations/:id",
		"DELETE /api/v1/registrations/:id",
		"POST /api/v1/registrations/:id/approve",
		"POST /api/v1/registrations/:id/reject",
		"GET /api/v1/users/me/activities/registered",
		"GET /api/v1/tickets",
		"GET /api/v1/tickets/:id",
		"POST /api/v1/check-ins",
		"GET /api/v1/check-ins",
		"GET /api/v1/notifications",
		"GET /api/v1/notifications/unread-count",
		"POST /api/v1/notifications/read",
		"POST /api/v1/notifications/read-all",
		"GET /api/v1/users/me/groups",
		"GET /api/v1/groups/:id",
		"GET /api/v1/groups/:id/members",
		"GET /api/v1/groups/:id/messages",
		"GET /api/v1/messages/offline",
		"POST /api/v1/activities/:id/approve",
		"POST /api/v1/activities/:id/reject",
		"POST /api/v1/activities/:id/send-back",
		"GET /api/v1/users/me/activities/created",
	} {
		if !registered[want] {
			t.Fatalf("missing route %s", want)
		}
	}
}
