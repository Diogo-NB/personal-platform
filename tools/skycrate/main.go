package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/adapter/in/cli"
	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/adapter/out/config"
	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/adapter/out/s3"
	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/application"
	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/domain/category"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := cli.New(loadDependencies).ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func loadDependencies(ctx context.Context, configPath string) (cli.Dependencies, error) {
	loaded, err := config.Load(configPath)
	if err != nil {
		return cli.Dependencies{}, err
	}

	catalog, err := category.NewCatalog(loaded.Categories)
	if err != nil {
		return cli.Dependencies{}, fmt.Errorf("create category catalog: %w", err)
	}
	awsConfig, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(loaded.Region))
	if err != nil {
		return cli.Dependencies{}, fmt.Errorf("load aws configuration: %w", err)
	}
	s3Client := awss3.NewFromConfig(awsConfig)
	repository, err := s3.New(loaded.Bucket, s3Client, transfermanager.New(s3Client))
	if err != nil {
		return cli.Dependencies{}, fmt.Errorf("create s3 repository: %w", err)
	}
	lifter, err := application.NewLiftService(repository, catalog, time.Now)
	if err != nil {
		return cli.Dependencies{}, fmt.Errorf("create lift service: %w", err)
	}
	lister, err := application.NewListService(repository, catalog)
	if err != nil {
		return cli.Dependencies{}, fmt.Errorf("create list service: %w", err)
	}

	return cli.Dependencies{
		Bucket:  loaded.Bucket,
		Catalog: catalog,
		Lifter:  lifter,
		Lister:  lister,
	}, nil
}
