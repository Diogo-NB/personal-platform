// Command dns-updater reconciles a Route 53 record with a singleton running
// ECS service task's public IPv4 address.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/route53"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := start(logger); err != nil {
		logger.Error("dns updater initialization failed", "error", err)
		os.Exit(1)
	}
}

func start(logger *slog.Logger) error {
	config, err := loadConfig(os.Getenv)
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	ctx := context.Background()
	awsConfig, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("loading aws configuration: %w", err)
	}

	handler := newUpdater(config, updaterDependencies{
		ecs:     ecs.NewFromConfig(awsConfig),
		ec2:     ec2.NewFromConfig(awsConfig),
		route53: route53.NewFromConfig(awsConfig),
		logger:  logger,
		wait:    waitForContext,
	})
	lambda.Start(handler.handle)
	return nil
}
