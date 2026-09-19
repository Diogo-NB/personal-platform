package http

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/diogo-nb/personal-platform/apps/ts6-management/internal/domain/lifecycle"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

type fakeLifecycle struct {
	startError  error
	stopError   error
	status      lifecycle.Status
	statusError error
	contexts    []context.Context
	panicOn     string
}

func (f *fakeLifecycle) Start(ctx context.Context) error {
	f.contexts = append(f.contexts, ctx)
	if f.panicOn == "start" {
		panic("secret panic detail")
	}
	return f.startError
}

func (f *fakeLifecycle) Stop(ctx context.Context) error {
	f.contexts = append(f.contexts, ctx)
	if f.panicOn == "stop" {
		panic("secret panic detail")
	}
	return f.stopError
}

func (f *fakeLifecycle) Status(ctx context.Context) (lifecycle.Status, error) {
	f.contexts = append(f.contexts, ctx)
	if f.panicOn == "status" {
		panic("secret panic detail")
	}
	return f.status, f.statusError
}

func TestNew(t *testing.T) {
	t.Parallel()

	service := &fakeLifecycle{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if _, err := New(nil, logger); err == nil {
		t.Error("nil lifecycle service error = nil, want error")
	}
	if _, err := New(service, nil); err == nil {
		t.Error("nil logger error = nil, want error")
	}
	if _, err := New(service, logger); err != nil {
		t.Errorf("valid dependencies error = %v", err)
	}
}

func TestNewRoutes(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, time.September, 19, 15, 30, 0, 0, time.FixedZone("BRT", -3*60*60))
	uptime := int64(1234)
	tests := []struct {
		name       string
		method     string
		path       string
		service    *fakeLifecycle
		wantStatus int
		wantBody   string
	}{
		{name: "start", method: http.MethodPost, path: "/start", service: &fakeLifecycle{}, wantStatus: http.StatusAccepted},
		{name: "stop", method: http.MethodPost, path: "/stop", service: &fakeLifecycle{}, wantStatus: http.StatusAccepted},
		{
			name:   "running status uses utc rfc3339",
			method: http.MethodGet,
			path:   "/status",
			service: &fakeLifecycle{status: lifecycle.Status{
				State:         lifecycle.StateRunning,
				StartedAt:     &startedAt,
				UptimeSeconds: &uptime,
			}},
			wantStatus: http.StatusOK,
			wantBody:   `{"state":"running","startedAt":"2026-09-19T18:30:00Z","uptimeSeconds":1234}`,
		},
		{
			name:       "nonrunning status has null fields",
			method:     http.MethodGet,
			path:       "/status",
			service:    &fakeLifecycle{status: lifecycle.Status{State: lifecycle.StateStopped}},
			wantStatus: http.StatusOK,
			wantBody:   `{"state":"stopped","startedAt":null,"uptimeSeconds":null}`,
		},
		{name: "readiness", method: http.MethodGet, path: "/internal/readiness", service: &fakeLifecycle{}, wantStatus: http.StatusNoContent},
		{name: "wrong start method", method: http.MethodGet, path: "/start", service: &fakeLifecycle{}, wantStatus: http.StatusMethodNotAllowed, wantBody: "405 method not allowed"},
		{name: "wrong stop method", method: http.MethodGet, path: "/stop", service: &fakeLifecycle{}, wantStatus: http.StatusMethodNotAllowed, wantBody: "405 method not allowed"},
		{name: "wrong status method", method: http.MethodPost, path: "/status", service: &fakeLifecycle{}, wantStatus: http.StatusMethodNotAllowed, wantBody: "405 method not allowed"},
		{name: "unknown path", method: http.MethodGet, path: "/unknown", service: &fakeLifecycle{}, wantStatus: http.StatusNotFound, wantBody: "404 page not found"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(test.method, test.path, nil)
			newRouter(t, test.service, slog.New(slog.NewTextHandler(io.Discard, nil))).ServeHTTP(recorder, request)

			if recorder.Code != test.wantStatus {
				t.Errorf("status = %d, want %d", recorder.Code, test.wantStatus)
			}
			if recorder.Body.String() != test.wantBody {
				t.Errorf("body = %q, want %q", recorder.Body.String(), test.wantBody)
			}
		})
	}
}

func TestNewPropagatesRequestContext(t *testing.T) {
	t.Parallel()

	type contextKey string
	ctx := context.WithValue(t.Context(), contextKey("request"), "value")
	service := &fakeLifecycle{}
	request := httptest.NewRequest(http.MethodPost, "/start", nil).WithContext(ctx)
	recorder := httptest.NewRecorder()

	newRouter(t, service, slog.New(slog.NewTextHandler(io.Discard, nil))).ServeHTTP(recorder, request)

	if len(service.contexts) != 1 || service.contexts[0] != ctx {
		t.Errorf("contexts = %v, want request context", service.contexts)
	}
}

func TestNewReturnsGenericErrorsWithoutDetails(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		method  string
		path    string
		service *fakeLifecycle
	}{
		{name: "start", method: http.MethodPost, path: "/start", service: &fakeLifecycle{startError: errors.New("secret task detail")}},
		{name: "stop", method: http.MethodPost, path: "/stop", service: &fakeLifecycle{stopError: errors.New("secret task detail")}},
		{name: "status", method: http.MethodGet, path: "/status", service: &fakeLifecycle{statusError: errors.New("secret task detail")}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var logs bytes.Buffer
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(test.method, test.path, nil)
			logger := slog.New(slog.NewJSONHandler(&logs, nil))
			newRouter(t, test.service, logger).ServeHTTP(recorder, request)

			if recorder.Code != http.StatusInternalServerError {
				t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
			}
			const wantBody = `{"error":"internal server error"}`
			if recorder.Body.String() != wantBody {
				t.Errorf("body = %q, want %q", recorder.Body.String(), wantBody)
			}
			if bytes.Contains(logs.Bytes(), []byte("secret task detail")) {
				t.Errorf("logs leaked dependency details: %q", logs.String())
			}
		})
	}
}

func TestNewRecoversWithGenericErrorWithoutPanicDetails(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/start", nil)
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	newRouter(t, &fakeLifecycle{panicOn: "start"}, logger).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	const wantBody = `{"error":"internal server error"}`
	if recorder.Body.String() != wantBody {
		t.Errorf("body = %q, want %q", recorder.Body.String(), wantBody)
	}
	if bytes.Contains(logs.Bytes(), []byte("secret panic detail")) {
		t.Errorf("logs leaked panic details: %q", logs.String())
	}
}

func newRouter(t *testing.T, service *fakeLifecycle, logger *slog.Logger) http.Handler {
	t.Helper()

	router, err := New(service, logger)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	return router
}
