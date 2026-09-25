package router

import (
	"net/http/httptest"
	"testing"
)

func TestHealthAndReady(t *testing.T) {
	r := New()
	for _, path := range []string{"/health", "/ready"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest("GET", path, nil)
		r.ServeHTTP(recorder, request)
		if recorder.Code != 200 {
			t.Fatalf("%s status: got %d", path, recorder.Code)
		}
	}
}
