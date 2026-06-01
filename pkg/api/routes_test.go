package api

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nouvadev/veritas/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestCreateURLRoutes(t *testing.T) {
	router := CreateURLRoutes(testApp())

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

			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}

func TestRedirectRoutes(t *testing.T) {
	router := RedirectRoutes(testApp())

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{
			name:       "serves healthcheck",
			method:     http.MethodGet,
			path:       "/healthcheck",
			wantStatus: http.StatusOK,
		},
		{
			name:       "rejects redirects with wrong method",
			method:     http.MethodPost,
			path:       "/abc123",
			wantStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}

func testApp() *config.AppConfig {
	return &config.AppConfig{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}
