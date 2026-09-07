package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/adapter/out/config"
	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/adapter/out/s3"
	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/application"
	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/domain/category"
)

func TestRootCommandHelp(t *testing.T) {
	t.Parallel()

	command := New(nil)
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	command.SetOut(stdout)
	command.SetErr(stderr)
	command.SetArgs([]string{"--help"})

	if err := command.Execute(); err != nil {
		t.Fatalf("execute root help: %v", err)
	}
	for _, expected := range []string{
		"hierarchical object categories",
		"current S3 repository is mocked",
		"lift        Store local file or directory metadata by object category",
		"list        Summarize stored object count and size",
		"--config string",
	} {
		if !strings.Contains(stdout.String(), expected) {
			t.Errorf("help does not contain %q:\n%s", expected, stdout)
		}
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want empty", stderr)
	}
}

func TestCommandReportsLoaderFailure(t *testing.T) {
	t.Parallel()

	loadErr := errors.New("configuration unavailable")
	command := New(func(string) (Dependencies, error) {
		return Dependencies{}, loadErr
	})
	command.SetArgs([]string{"list"})
	if err := command.Execute(); !errors.Is(err, loadErr) {
		t.Fatalf("Execute() error = %v, want loader error", err)
	}
}

func TestCommandRequiresConfig(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "missing.yaml")
	stdout, stderr, err := executeCommand(t, missing, nil, "list")
	if err == nil || !strings.Contains(err.Error(), "read config") {
		t.Fatalf("Execute() error = %v, want config error", err)
	}
	if stdout != "" || stderr != "" {
		t.Errorf("output = (%q, %q), want empty", stdout, stderr)
	}
}

func executeCommand(
	t *testing.T,
	configPath string,
	input io.Reader,
	args ...string,
) (string, string, error) {
	t.Helper()

	command := New(loadTestDependencies)
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	command.SetOut(stdout)
	command.SetErr(stderr)
	if input != nil {
		command.SetIn(input)
	}
	command.SetArgs(append([]string{"--config", configPath}, args...))

	err := command.Execute()
	return stdout.String(), stderr.String(), err
}

func loadTestDependencies(configPath string) (Dependencies, error) {
	loaded, err := config.Load(configPath)
	if err != nil {
		return Dependencies{}, err
	}
	catalog, err := category.NewCatalog(loaded.Categories)
	if err != nil {
		return Dependencies{}, err
	}
	repository, err := s3.New(loaded.Bucket, io.Discard)
	if err != nil {
		return Dependencies{}, err
	}
	lifter, err := application.NewLiftService(repository, catalog, time.Now)
	if err != nil {
		return Dependencies{}, err
	}
	lister, err := application.NewListService(repository, catalog)
	if err != nil {
		return Dependencies{}, err
	}
	return Dependencies{
		Bucket:  loaded.Bucket,
		Catalog: catalog,
		Lifter:  lifter,
		Lister:  lister,
	}, nil
}

func writeCommandConfig(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")
	contents := `bucket: test-bucket
categories:
  documents:
    tier: STANDARD
  photos:
    tier: STANDARD_IA
  backups:
    tier: DEEP_ARCHIVE
  backups/university:
    tier: GLACIER
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
