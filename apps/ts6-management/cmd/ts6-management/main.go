package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/gin-gonic/gin"

	httpadapter "github.com/diogo-nb/personal-platform/apps/ts6-management/internal/adapter/in/http"
	"github.com/diogo-nb/personal-platform/apps/ts6-management/internal/adapter/out/clock"
	"github.com/diogo-nb/personal-platform/apps/ts6-management/internal/adapter/out/config"
	ecsadapter "github.com/diogo-nb/personal-platform/apps/ts6-management/internal/adapter/out/ecs"
	"github.com/diogo-nb/personal-platform/apps/ts6-management/internal/application"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	configuration, err := config.Load()
	if err != nil {
		logger.Error("configuration failed")
		os.Exit(1)
	}

	awsConfiguration, err := awsconfig.LoadDefaultConfig(context.Background())
	if err != nil {
		logger.Error("aws configuration failed")
		os.Exit(1)
	}

	gin.SetMode(gin.ReleaseMode)
	scheduler, err := ecsadapter.New(
		ecs.NewFromConfig(awsConfiguration),
		configuration.ClusterARN,
		configuration.ServiceName,
	)
	if err != nil {
		logger.Error("ecs adapter initialization failed")
		os.Exit(1)
	}

	service, err := application.NewLifecycleService(scheduler, clock.System{})
	if err != nil {
		logger.Error("lifecycle service initialization failed")
		os.Exit(1)
	}

	router, err := httpadapter.New(service, logger)
	if err != nil {
		logger.Error("http adapter initialization failed")
		os.Exit(1)
	}

	server := &http.Server{
		Addr:              ":8080",
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	logger.Info("management server listening", "port", 8080)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("management server stopped unexpectedly")
		os.Exit(1)
	}
}
