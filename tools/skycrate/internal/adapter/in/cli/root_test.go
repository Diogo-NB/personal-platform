package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/adapter/out/config"
	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/application"
	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/domain/category"
	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/domain/object"
	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/domain/storage"
	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/port/out"
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
		"uploads files to Amazon S3",
		"lift        Store a local file or directory in Amazon S3 by object category",
		"list        Summarize stored S3 object count and size",
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
	command := New(func(context.Context, string) (Dependencies, error) {
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

func loadTestDependencies(_ context.Context, configPath string) (Dependencies, error) {
	loaded, err := config.Load(configPath)
	if err != nil {
		return Dependencies{}, err
	}
	catalog, err := category.NewCatalog(loaded.Categories)
	if err != nil {
		return Dependencies{}, err
	}
	repository := &testRepository{objects: testObjects()}
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

type testRepository struct {
	objects []object.Object
}

func (r *testRepository) Save(_ context.Context, _ out.SaveRequest) error {
	return nil
}

func (r *testRepository) FindMany(
	_ context.Context,
	request out.FindManyRequest,
) ([]object.Object, error) {
	objects := make([]object.Object, 0, len(r.objects))
	for _, storedObject := range r.objects {
		if request.Category != "" &&
			storedObject.Category != request.Category &&
			!strings.HasPrefix(storedObject.Category, request.Category+"/") {
			continue
		}
		if request.Tier != storage.TierUnknown && storedObject.Tier != request.Tier {
			continue
		}
		objects = append(objects, storedObject)
	}
	return objects, nil
}

func testObjects() []object.Object {
	timestamp := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	return []object.Object{
		{
			Path: "documents/report.pdf", Name: "report.pdf", Category: "documents",
			Size: 1_000_000_000, Tier: storage.TierInstant, CreatedAt: timestamp, UpdatedAt: timestamp,
		},
		{
			Path: "photos/image.jpg", Name: "image.jpg", Category: "photos",
			Size: 2_000_000_000, Tier: storage.TierCold, CreatedAt: timestamp, UpdatedAt: timestamp,
		},
		{
			Path: "backups/archive.tar", Name: "archive.tar", Category: "backups",
			Size: 7_000_000_000, Tier: storage.TierArchive, CreatedAt: timestamp, UpdatedAt: timestamp,
		},
	}
}

func writeCommandConfig(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")
	contents := `bucket: test-bucket
region: us-east-1
categories:
  documents:
    tier: instant
  photos:
    tier: cold
  backups:
    tier: archive
  backups/university:
    tier: cold
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
