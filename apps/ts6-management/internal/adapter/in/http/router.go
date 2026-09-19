package http

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	sqsadapter "github.com/diogo-nb/personal-platform/apps/ts6-management/internal/adapter/in/sqs"
	"github.com/diogo-nb/personal-platform/apps/ts6-management/internal/domain/lifecycle"
	portin "github.com/diogo-nb/personal-platform/apps/ts6-management/internal/port/in"
)

type handler struct {
	lifecycle portin.Lifecycle
	logger    *slog.Logger
}

type statusResponse struct {
	State         lifecycle.State `json:"state"`
	StartedAt     *string         `json:"startedAt"`
	UptimeSeconds *int64          `json:"uptimeSeconds"`
}

func New(service portin.Lifecycle, logger *slog.Logger) (*gin.Engine, error) {
	if service == nil {
		return nil, errors.New("http: lifecycle service is required")
	}
	if logger == nil {
		return nil, errors.New("http: logger is required")
	}

	router := gin.New()
	router.HandleMethodNotAllowed = true
	h := &handler{lifecycle: service, logger: logger}
	eventHandler, err := sqsadapter.New(service, logger)
	if err != nil {
		return nil, err
	}

	// Gin's default recovery output includes the panic value and request metadata.
	router.Use(gin.CustomRecoveryWithWriter(io.Discard, func(c *gin.Context, _ any) {
		logger.Error("http handler panic", "path", c.FullPath())
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}))
	router.POST("/start", h.start)
	router.POST("/stop", h.stop)
	router.GET("/status", h.status)
	router.POST("/internal/events", eventHandler)
	router.GET("/internal/readiness", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	return router, nil
}

func (h *handler) start(c *gin.Context) {
	if err := h.lifecycle.Start(c.Request.Context()); err != nil {
		h.internalError(c, "start")
		return
	}

	c.Status(http.StatusAccepted)
}

func (h *handler) stop(c *gin.Context) {
	if err := h.lifecycle.Stop(c.Request.Context()); err != nil {
		h.internalError(c, "stop")
		return
	}

	c.Status(http.StatusAccepted)
}

func (h *handler) status(c *gin.Context) {
	status, err := h.lifecycle.Status(c.Request.Context())
	if err != nil {
		h.internalError(c, "status")
		return
	}

	response := statusResponse{
		State:         status.State,
		UptimeSeconds: status.UptimeSeconds,
	}
	if status.StartedAt != nil {
		formatted := status.StartedAt.UTC().Format(time.RFC3339)
		response.StartedAt = &formatted
	}

	c.JSON(http.StatusOK, response)
}

func (h *handler) internalError(c *gin.Context, operation string) {
	// Dependency errors can contain task identifiers, request metadata, or secret-bearing details.
	h.logger.Error("lifecycle request failed", "operation", operation)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
}
