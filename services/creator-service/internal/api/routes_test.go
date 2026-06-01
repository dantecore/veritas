package api

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRoutes(t *testing.T) {
	router := Routes(testLogger(), nil)

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
	}{
		{
			name:       "serves API healthcheck",
			method:     http.MethodGet,
			path:       "/api/healthcheck",
			wantStatus: http.StatusOK,
		},
		{
			name:       "routes API create requests",
			method:     http.MethodPost,
			path:       "/api/create",
			body:       "{",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "rejects create requests with wrong method",
			method:     http.MethodGet,
			path:       "/api/create",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "does not expose unprefixed create route",
			method:     http.MethodPost,
			path:       "/create",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d", tt.wantStatus, rec.Code)
			}
		})
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
