package s3

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/domain/object"
	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/port/out"
)

func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		bucket  string
		output  io.Writer
		wantErr bool
	}{
		{name: "valid", bucket: "skycrate-storage", output: io.Discard},
		{name: "empty", output: io.Discard, wantErr: true},
		{name: "whitespace", bucket: " bucket ", output: io.Discard, wantErr: true},
		{name: "nil output", bucket: "skycrate-storage", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := New(test.bucket, test.output)
			if (err != nil) != test.wantErr {
				t.Fatalf("New(%q) error = %v, wantErr %t", test.bucket, err, test.wantErr)
			}
		})
	}
}

func TestRepositorySave(t *testing.T) {
	t.Parallel()

	storedObject := validObject(t)
	output := new(bytes.Buffer)
	repository, err := New("skycrate-storage", output)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if err := repository.Save(t.Context(), storedObject); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	for _, expected := range []string{
		"Mock S3 object:",
		"Path:documents/report.pdf",
		"Name:report.pdf",
		"Category:documents",
		"Size:1",
		"Tier:STANDARD",
		"CreatedAt:",
		"UpdatedAt:",
	} {
		if !strings.Contains(output.String(), expected) {
			t.Errorf("mock output does not contain %q: %s", expected, output)
		}
	}

	invalid := storedObject
	invalid.Path = ""
	if err := repository.Save(t.Context(), invalid); err == nil {
		t.Fatal("Save(invalid) error = nil")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := repository.Save(ctx, storedObject); !errors.Is(err, context.Canceled) {
		t.Fatalf("Save(canceled) error = %v", err)
	}
}

func TestRepositoryFindMany(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		request   out.FindManyRequest
		wantCount int
		wantBytes int64
	}{
		{name: "all", wantCount: 3, wantBytes: 10_000_000_000},
		{name: "category", request: out.FindManyRequest{Category: "DOCUMENTS"}, wantCount: 1, wantBytes: 1_000_000_000},
		{name: "tier", request: out.FindManyRequest{Tier: "standard_ia"}, wantCount: 1, wantBytes: 2_000_000_000},
		{name: "combined no match", request: out.FindManyRequest{Category: "backups/university", Tier: "deep_archive"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			objects, err := newTestRepository(t).FindMany(t.Context(), test.request)
			if err != nil {
				t.Fatalf("FindMany() error: %v", err)
			}
			if len(objects) != test.wantCount {
				t.Fatalf("FindMany() count = %d, want %d", len(objects), test.wantCount)
			}
			var total int64
			for _, storedObject := range objects {
				total += storedObject.Size
			}
			if total != test.wantBytes {
				t.Errorf("FindMany() bytes = %d, want %d", total, test.wantBytes)
			}
		})
	}
}

func TestRepositoryFindManyReturnsAuthoritativeFixtures(t *testing.T) {
	t.Parallel()

	objects, err := newTestRepository(t).FindMany(t.Context(), out.FindManyRequest{})
	if err != nil {
		t.Fatalf("FindMany() error: %v", err)
	}
	wantPaths := []string{"documents/report.pdf", "photos/photo.jpg", "backups/database.dump"}
	for index, want := range wantPaths {
		if objects[index].Path != want {
			t.Errorf("FindMany()[%d].Path = %q, want %q", index, objects[index].Path, want)
		}
		if objects[index].CreatedAt.Location() != time.UTC || objects[index].UpdatedAt.Location() != time.UTC {
			t.Errorf("FindMany()[%d] timestamps are not UTC", index)
		}
	}
}

func TestRepositoryFindManyCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := newTestRepository(t).FindMany(ctx, out.FindManyRequest{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("FindMany(canceled) error = %v", err)
	}
}

func newTestRepository(t *testing.T) *Repository {
	t.Helper()
	repository, err := New("skycrate-storage", io.Discard)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	return repository
}

func validObject(t *testing.T) object.Object {
	t.Helper()
	storedObject, err := object.New(object.NewParams{
		RelativePath: "report.pdf",
		Category:     "documents",
		Size:         1,
		Tier:         "STANDARD",
		Timestamp:    time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("object.New() error: %v", err)
	}
	return storedObject
}
