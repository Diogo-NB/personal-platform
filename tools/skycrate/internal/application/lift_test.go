package application

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/port/out"
)

func TestNewLiftService(t *testing.T) {
	t.Parallel()

	repository := &stubRepository{}
	catalog := newTestCatalog(t)
	tests := []struct {
		name       string
		repository out.ObjectRepository
		catalogNil bool
		now        func() time.Time
	}{
		{name: "nil repository", catalogNil: false, now: time.Now},
		{name: "nil catalog", repository: repository, catalogNil: true, now: time.Now},
		{name: "nil clock", repository: repository, catalogNil: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			candidateCatalog := catalog
			if test.catalogNil {
				candidateCatalog = nil
			}
			if _, err := NewLiftService(test.repository, candidateCatalog, test.now); err == nil {
				t.Fatal("NewLiftService() error = nil, want dependency error")
			}
		})
	}
}

func TestLiftServiceLift(t *testing.T) {
	t.Parallel()

	filePath := filepath.Join(t.TempDir(), "My Report.PDF")
	if err := os.WriteFile(filePath, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	repository := &stubRepository{}
	now := time.Date(2026, time.April, 1, 10, 0, 0, 0, time.FixedZone("test", -3*60*60))
	service, err := NewLiftService(repository, newTestCatalog(t), func() time.Time { return now })
	if err != nil {
		t.Fatalf("NewLiftService() error: %v", err)
	}

	got, err := service.Lift(t.Context(), filePath, "BACKUP/University/Thesis")
	if err != nil {
		t.Fatalf("Lift() error: %v", err)
	}
	if len(repository.saved) != 1 || got != repository.saved[0] {
		t.Fatalf("saved objects = %#v, result = %#v", repository.saved, got)
	}
	if got.Path != "backup/university/thesis/my-report.pdf" || got.Tier != "DEEP_ARCHIVE" {
		t.Errorf("Lift() = %#v", got)
	}
	if got.Size != 5 || !got.CreatedAt.Equal(now.UTC()) || !got.UpdatedAt.Equal(now.UTC()) {
		t.Errorf("Lift() metadata = %#v", got)
	}
}

func TestLiftServiceRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		prepare func(*testing.T) string
		ctx     func(*testing.T) context.Context
	}{
		{name: "missing file", prepare: func(t *testing.T) string { return filepath.Join(t.TempDir(), "missing") }, ctx: func(t *testing.T) context.Context { return t.Context() }},
		{name: "directory", prepare: func(t *testing.T) string { return t.TempDir() }, ctx: func(t *testing.T) context.Context { return t.Context() }},
		{
			name: "symbolic link",
			prepare: func(t *testing.T) string {
				directory := t.TempDir()
				target := filepath.Join(directory, "target")
				if err := os.WriteFile(target, []byte("data"), 0o600); err != nil {
					t.Fatalf("write target: %v", err)
				}
				link := filepath.Join(directory, "link")
				if err := os.Symlink(target, link); err != nil {
					t.Fatalf("create symlink: %v", err)
				}
				return link
			},
			ctx: func(t *testing.T) context.Context { return t.Context() },
		},
		{
			name: "canceled context",
			prepare: func(t *testing.T) string {
				path := filepath.Join(t.TempDir(), "file")
				if err := os.WriteFile(path, []byte("data"), 0o600); err != nil {
					t.Fatalf("write fixture: %v", err)
				}
				return path
			},
			ctx: func(*testing.T) context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repository := &stubRepository{}
			service, err := NewLiftService(repository, newTestCatalog(t), time.Now)
			if err != nil {
				t.Fatalf("NewLiftService() error: %v", err)
			}
			if _, err := service.Lift(test.ctx(t), test.prepare(t), "backup"); err == nil {
				t.Fatal("Lift() error = nil, want input error")
			}
			if len(repository.saved) != 0 {
				t.Errorf("Save() calls = %d, want 0", len(repository.saved))
			}
		})
	}
}

func TestLiftServicePropagatesRepositoryError(t *testing.T) {
	t.Parallel()

	filePath := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(filePath, []byte("data"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	repository := &stubRepository{saveErr: errors.New("save failed")}
	service, err := NewLiftService(repository, newTestCatalog(t), time.Now)
	if err != nil {
		t.Fatalf("NewLiftService() error: %v", err)
	}
	_, err = service.Lift(t.Context(), filePath, "backup")
	if err == nil || !strings.Contains(err.Error(), "save object: save failed") {
		t.Fatalf("Lift() error = %v, want wrapped repository error", err)
	}
}
