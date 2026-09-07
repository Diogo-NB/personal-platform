package cli

import (
	"errors"
	"fmt"

	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/domain/category"
	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/port/in"
	"github.com/spf13/cobra"
)

type Dependencies struct {
	Bucket  string
	Catalog *category.Catalog
	Lifter  in.Lifter
	Lister  in.Lister
}

type Loader func(configPath string) (Dependencies, error)

type commandRuntime struct {
	configPath   string
	load         Loader
	dependencies Dependencies
}

func New(load Loader) *cobra.Command {
	runtime := &commandRuntime{load: load}
	rootCmd := &cobra.Command{
		Use:   "skycrate",
		Short: "Manage files in cloud storage",
		Long: `Skycrate routes local file metadata through hierarchical object categories.
It uses one configured bucket, resolves each category to a storage tier, and
summarizes objects returned by the configured storage repository.

The current S3 repository is mocked. Lift validates and models a local file but
stores only its metadata; list returns deterministic mocked objects.`,
		Example: `  skycrate --config ./skycrate.yaml lift ./report.pdf documents
  skycrate --config ./skycrate.yaml list --category backups`,
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRunE: func(*cobra.Command, []string) error {
			return runtime.initialize()
		},
	}

	rootCmd.PersistentFlags().StringVar(
		&runtime.configPath,
		"config",
		"",
		"path to the required YAML config file",
	)
	rootCmd.AddCommand(newLiftCommand(runtime), newListCommand(runtime))

	return rootCmd
}

func (r *commandRuntime) initialize() error {
	if r.load == nil {
		return errors.New("load application dependencies: loader must not be nil")
	}

	dependencies, err := r.load(r.configPath)
	if err != nil {
		return fmt.Errorf("load application dependencies: %w", err)
	}
	if dependencies.Bucket == "" {
		return errors.New("load application dependencies: bucket must not be empty")
	}
	if dependencies.Catalog == nil {
		return errors.New("load application dependencies: category catalog must not be nil")
	}
	if dependencies.Lifter == nil {
		return errors.New("load application dependencies: lifter must not be nil")
	}
	if dependencies.Lister == nil {
		return errors.New("load application dependencies: lister must not be nil")
	}

	r.dependencies = dependencies
	return nil
}
