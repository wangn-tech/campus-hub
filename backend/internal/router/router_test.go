package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wangn-tech/campus-hub/internal/health"
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
