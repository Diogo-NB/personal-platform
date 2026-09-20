package sqs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/diogo-nb/personal-platform/apps/ts6-management-api/internal/domain/lifecycle"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

type fakeLifecycle struct {
	startError error
	stopError  error
	panicOn    string
	actions    []string
	contexts   []context.Context
}

func (f *fakeLifecycle) Start(ctx context.Context) error {
	f.actions = append(f.actions, actionStart)
	f.contexts = append(f.contexts, ctx)
	if f.panicOn == actionStart {
		panic("secret panic detail")
	}
	return f.startError
}

func (f *fakeLifecycle) Stop(ctx context.Context) error {
	f.actions = append(f.actions, actionStop)
	f.contexts = append(f.contexts, ctx)
	if f.panicOn == actionStop {
		panic("secret panic detail")
	}
	return f.stopError
}

func (f *fakeLifecycle) Status(context.Context) (lifecycle.Status, error) {
	return lifecycle.Status{}, nil
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

func TestHandlerProcessesCommands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		body       string
		wantAction string
	}{
		{name: "start", body: `{"action":"start"}`, wantAction: actionStart},
		{name: "stop", body: `{"action":"stop"}`, wantAction: actionStop},
		{
			name:       "start with additional metadata",
			body:       `{"action":"start","source":"automation"}`,
			wantAction: actionStart,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeLifecycle{}
			response := invoke(t, t.Context(), service, slog.New(slog.NewTextHandler(io.Discard, nil)), record{
				MessageID: "message-1",
				Body:      test.body,
			})

			if !reflect.DeepEqual(service.actions, []string{test.wantAction}) {
				t.Errorf("actions = %v, want %v", service.actions, []string{test.wantAction})
			}
			if len(response.Failures) != 0 {
				t.Errorf("failures = %v, want none", response.Failures)
			}
		})
	}
}

func TestHandlerPropagatesRequestContext(t *testing.T) {
	t.Parallel()

	type contextKey string
	ctx := context.WithValue(t.Context(), contextKey("invocation"), "value")
	service := &fakeLifecycle{}
	response := invoke(
		t,
		ctx,
		service,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		record{MessageID: "message-1", Body: `{"action":"start"}`},
	)

	if len(response.Failures) != 0 {
		t.Errorf("failures = %v, want none", response.Failures)
	}
	if len(service.contexts) != 1 || service.contexts[0] != ctx {
		t.Errorf("contexts = %v, want invocation request context", service.contexts)
	}
}

func TestHandlerProcessesDuplicateDeliveries(t *testing.T) {
	t.Parallel()

	service := &fakeLifecycle{}
	record := record{MessageID: "message-1", Body: `{"action":"start"}`}
	response := invoke(
		t,
		t.Context(),
		service,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		record,
		record,
	)

	if !reflect.DeepEqual(service.actions, []string{actionStart, actionStart}) {
		t.Errorf("actions = %v, want duplicate start reconciliation", service.actions)
	}
	if len(response.Failures) != 0 {
		t.Errorf("failures = %v, want none", response.Failures)
	}
}

func TestHandlerRequiresMessageGroup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		attributes map[string]string
	}{
		{name: "missing group", attributes: map[string]string{}},
		{name: "wrong group", attributes: map[string]string{"MessageGroupId": "other"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeLifecycle{}
			response := invoke(t, t.Context(), service, slog.New(slog.NewTextHandler(io.Discard, nil)), record{
				MessageID:  "failed-message",
				Body:       `{"action":"start"}`,
				Attributes: test.attributes,
			})

			if len(service.actions) != 0 {
				t.Errorf("actions = %v, want none", service.actions)
			}
			want := []batchFailure{{ItemIdentifier: "failed-message"}}
			if !reflect.DeepEqual(response.Failures, want) {
				t.Errorf("failures = %#v, want %#v", response.Failures, want)
			}
		})
	}
}

func TestHandlerRejectsInvalidCommands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
	}{
		{name: "empty body", body: ""},
		{name: "malformed json", body: `{"action":`},
		{name: "nonobject", body: `"start"`},
		{name: "missing action", body: `{}`},
		{name: "null action", body: `{"action":null}`},
		{name: "trailing value", body: `{"action":"start"} {}`},
		{name: "unknown action", body: `{"action":"restart"}`},
		{name: "wrong case", body: `{"action":"START"}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeLifecycle{}
			response := invoke(t, t.Context(), service, slog.New(slog.NewTextHandler(io.Discard, nil)), record{
				MessageID: "failed-message",
				Body:      test.body,
			})

			if len(service.actions) != 0 {
				t.Errorf("actions = %v, want none", service.actions)
			}
			want := []batchFailure{{ItemIdentifier: "failed-message"}}
			if !reflect.DeepEqual(response.Failures, want) {
				t.Errorf("failures = %#v, want %#v", response.Failures, want)
			}
		})
	}
}

func TestHandlerReportsLifecycleFailures(t *testing.T) {
	t.Parallel()

	service := &fakeLifecycle{stopError: errors.New("secret task detail")}
	var logs bytes.Buffer
	response := invoke(
		t,
		t.Context(),
		service,
		slog.New(slog.NewJSONHandler(&logs, nil)),
		record{MessageID: "successful-message", Body: `{"action":"start"}`},
		record{MessageID: "failed-message", Body: `{"action":"stop"}`},
	)

	want := []batchFailure{{ItemIdentifier: "failed-message"}}
	if !reflect.DeepEqual(response.Failures, want) {
		t.Errorf("failures = %#v, want %#v", response.Failures, want)
	}
	if bytes.Contains(logs.Bytes(), []byte("secret task detail")) {
		t.Errorf("logs leaked dependency details: %q", logs.String())
	}
}

func TestHandlerReturnsPartialBatchResponseSchema(t *testing.T) {
	t.Parallel()

	payload, err := json.Marshal(event{Records: []record{
		{
			MessageID:  "failed-message",
			Body:       `{"action":"stop"}`,
			Attributes: map[string]string{"MessageGroupId": messageGroupID},
		},
	}})
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	recorder := serve(
		t,
		t.Context(),
		&fakeLifecycle{stopError: errors.New("unavailable")},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		string(payload),
	)

	if recorder.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	const wantBody = `{"batchItemFailures":[{"itemIdentifier":"failed-message"}]}`
	if recorder.Body.String() != wantBody {
		t.Errorf("body = %q, want %q", recorder.Body.String(), wantBody)
	}
}

func TestHandlerRecoversPanicsPerRecord(t *testing.T) {
	t.Parallel()

	service := &fakeLifecycle{panicOn: actionStart}
	var logs bytes.Buffer
	response := invoke(
		t,
		t.Context(),
		service,
		slog.New(slog.NewJSONHandler(&logs, nil)),
		record{MessageID: "panicked-message", Body: `{"action":"start"}`},
		record{MessageID: "successful-message", Body: `{"action":"stop"}`},
	)

	want := []batchFailure{{ItemIdentifier: "panicked-message"}}
	if !reflect.DeepEqual(response.Failures, want) {
		t.Errorf("failures = %#v, want %#v", response.Failures, want)
	}
	if !reflect.DeepEqual(service.actions, []string{actionStart, actionStop}) {
		t.Errorf("actions = %v, want processing to continue after panic", service.actions)
	}
	if bytes.Contains(logs.Bytes(), []byte("secret panic detail")) {
		t.Errorf("logs leaked panic details: %q", logs.String())
	}
}

func TestHandlerRejectsInvalidEvents(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
	}{
		{name: "malformed json", body: `{"Records":`},
		{name: "missing records", body: `{}`},
		{name: "empty records", body: `{"Records":[]}`},
		{name: "missing message id", body: `{"Records":[{"body":"{\"action\":\"start\"}"}]}`},
		{name: "trailing value", body: `{"Records":[]} {}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeLifecycle{}
			recorder := serve(t, t.Context(), service, slog.New(slog.NewTextHandler(io.Discard, nil)), test.body)
			if recorder.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
			}
			const wantBody = `{"error":"invalid sqs event"}`
			if recorder.Body.String() != wantBody {
				t.Errorf("body = %q, want %q", recorder.Body.String(), wantBody)
			}
			if len(service.actions) != 0 {
				t.Errorf("actions = %v, want none", service.actions)
			}
		})
	}
}

func invoke(
	t *testing.T,
	ctx context.Context,
	service *fakeLifecycle,
	logger *slog.Logger,
	records ...record,
) batchResponse {
	t.Helper()

	for i := range records {
		if records[i].Attributes == nil {
			records[i].Attributes = map[string]string{"MessageGroupId": messageGroupID}
		}
	}
	payload, err := json.Marshal(event{Records: records})
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	recorder := serve(t, ctx, service, logger, string(payload))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var response batchResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return response
}

func serve(
	t *testing.T,
	ctx context.Context,
	service *fakeLifecycle,
	logger *slog.Logger,
	body string,
) *httptest.ResponseRecorder {
	t.Helper()

	handler, err := New(service, logger)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	router := gin.New()
	router.POST("/internal/events", handler)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/internal/events", bytes.NewBufferString(body)).WithContext(ctx)
	router.ServeHTTP(recorder, request)
	return recorder
}
