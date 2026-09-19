// Package sqs translates native SQS events forwarded by Lambda Web Adapter into lifecycle commands.
package sqs

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	portin "github.com/diogo-nb/personal-platform/apps/ts6-management/internal/port/in"
)

const (
	actionStart    = "start"
	actionStop     = "stop"
	messageGroupID = "teamspeak6"
)

type handler struct {
	lifecycle portin.Lifecycle
	logger    *slog.Logger
}

type event struct {
	Records []record `json:"Records"`
}

type command struct {
	Action string `json:"action"`
}

type record struct {
	MessageID  string            `json:"messageId"`
	Body       string            `json:"body"`
	Attributes map[string]string `json:"attributes"`
}

type batchResponse struct {
	Failures []batchFailure `json:"batchItemFailures"`
}

type batchFailure struct {
	ItemIdentifier string `json:"itemIdentifier"`
}

// New returns a handler for Lambda Web Adapter's private non-HTTP event route.
// Record failures stay in the response so SQS can retry only unsuccessful commands.
func New(service portin.Lifecycle, logger *slog.Logger) (gin.HandlerFunc, error) {
	if service == nil {
		return nil, errors.New("sqs: lifecycle service is required")
	}
	if logger == nil {
		return nil, errors.New("sqs: logger is required")
	}

	h := &handler{lifecycle: service, logger: logger}
	return h.handle, nil
}

func (h *handler) handle(c *gin.Context) {
	event, err := decodeEvent(c.Request.Body)
	if err != nil {
		h.logger.Error("sqs event rejected")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sqs event"})
		return
	}

	failures := make([]batchFailure, 0)
	for _, record := range event.Records {
		if h.process(c.Request.Context(), record) {
			failures = append(failures, batchFailure{ItemIdentifier: record.MessageID})
		}
	}

	c.JSON(http.StatusOK, batchResponse{Failures: failures})
}

func (h *handler) process(ctx context.Context, record record) (failed bool) {
	operation := "unknown"
	defer func() {
		if recover() == nil {
			return
		}

		h.logger.Error("sqs command panic", "operation", operation)
		failed = true
	}()
	if record.Attributes["MessageGroupId"] != messageGroupID {
		h.logger.Warn("sqs command rejected")
		return true
	}

	action, err := decodeCommand(record.Body)
	if err != nil {
		h.logger.Warn("sqs command rejected")
		return true
	}
	operation = action

	switch action {
	case actionStart:
		err = h.lifecycle.Start(ctx)
	case actionStop:
		err = h.lifecycle.Stop(ctx)
	}
	if err != nil {
		h.logger.Error("sqs lifecycle command failed", "operation", operation)
		return true
	}

	return false
}

func decodeEvent(reader io.Reader) (event, error) {
	payload, err := io.ReadAll(reader)
	if err != nil {
		return event{}, err
	}

	var value event
	if err := json.Unmarshal(payload, &value); err != nil {
		return event{}, err
	}
	if len(value.Records) == 0 {
		return event{}, errors.New("sqs event has no records")
	}
	for _, record := range value.Records {
		if record.MessageID == "" {
			return event{}, errors.New("sqs record has no message id")
		}
	}

	return value, nil
}

func decodeCommand(body string) (string, error) {
	var value command
	if err := json.Unmarshal([]byte(body), &value); err != nil {
		return "", err
	}

	switch value.Action {
	case actionStart, actionStop:
		return value.Action, nil
	default:
		return "", errors.New("sqs command has an unsupported action")
	}
}
