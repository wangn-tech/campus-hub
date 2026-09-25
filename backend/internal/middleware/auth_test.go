package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type stubAuthenticator struct {
	uuid string
	err  error
}

func (s stubAuthenticator) Authenticate(context.Context, string) (string, error) {
	return s.uuid, s.err
}

func TestBearerToken(t *testing.T) {
	cases := []struct {
		name   string
		header string
		want   string
		ok     bool
	}{
		{name: "valid", header: "Bearer abc.def.ghi", want: "abc.def.ghi", ok: true},
		{name: "case insensitive scheme", header: "bearer token", want: "token", ok: true},
		{name: "missing header", header: "", ok: false},
		{name: "wrong scheme", header: "Basic abc", ok: false},
		{name: "empty token", header: "Bearer ", ok: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
			c.Request.Header.Set("Authorization", tc.header)
			got, ok := BearerToken(c)
			if got != tc.want || ok != tc.ok {
				t.Fatalf("BearerToken(%q) = (%q, %v), want (%q, %v)", tc.header, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestAuthRejectsMissingToken(t *testing.T) {
	recorder := httptest.NewRecorder()
	newAuthRouter(stubAuthenticator{uuid: "user-uuid"}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/me", nil))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d", recorder.Code)
	}
}

func TestAuthRejectsInvalidToken(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/me", nil)
	request.Header.Set("Authorization", "Bearer broken")
	newAuthRouter(stubAuthenticator{err: errors.New("invalid token")}).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d", recorder.Code)
	}
}

func TestAuthSetsUserUUID(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/me", nil)
	request.Header.Set("Authorization", "Bearer valid")
	newAuthRouter(stubAuthenticator{uuid: "user-uuid"}).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.String() != "user-uuid" {
		t.Fatalf("status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}

func newAuthRouter(auth Authenticator) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Auth(auth))
	r.GET("/me", func(c *gin.Context) { c.String(http.StatusOK, c.GetString(UserUUIDKey)) })
	return r
}
