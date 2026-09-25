package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestRecoveryReturnsUnifiedResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Request(zap.NewNop()), Recovery(zap.NewNop()))
	r.GET("/panic", func(*gin.Context) { panic("test panic") })
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/panic", nil))
	if recorder.Code != http.StatusInternalServerError || !strings.Contains(recorder.Body.String(), "internal server error") || recorder.Header().Get("X-Request-ID") == "" {
		t.Fatalf("unexpected recovery response: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
