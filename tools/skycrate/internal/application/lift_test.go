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
	if len(got) != 1 {
		t.Fatalf("Lift() object count = %d, want 1", len(got))
	}
	if len(repository.saved) != 1 || got[0] != repository.saved[0] {
		t.Fatalf("saved objects = %#v, result = %#v", repository.saved, got)
	}
	if got[0].Path != "backup/university/thesis/my-report.pdf" || got[0].Tier != "DEEP_ARCHIVE" {
		t.Errorf("Lift() = %#v", got[0])
	}
	if got[0].Size != 5 || !got[0].CreatedAt.Equal(now.UTC()) || !got[0].UpdatedAt.Equal(now.UTC()) {
		t.Errorf("Lift() metadata = %#v", got[0])
	}
}

func TestLiftServiceLiftsDirectoryRecursively(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	writeLiftFixture(t, filepath.Join(directory, "Root File.TXT"), "root")
	writeLiftFixture(t, filepath.Join(directory, "Research Notes", "Draft One.MD"), "draft")
	writeLiftFixture(t, filepath.Join(directory, "Research Notes", "Final.PDF"), "final report")
	if err := os.Mkdir(filepath.Join(directory, "empty"), 0o700); err != nil {
		t.Fatalf("create empty directory: %v", err)
	}

	repository := &stubRepository{}
	now := time.Date(2026, time.April, 1, 10, 0, 0, 0, time.UTC)
	service, err := NewLiftService(repository, newTestCatalog(t), func() time.Time { return now })
	if err != nil {
		t.Fatalf("NewLiftService() error: %v", err)
	}

	objects, err := service.Lift(t.Context(), directory, "backup")
	if err != nil {
		t.Fatalf("Lift() error: %v", err)
	}
	wantPaths := []string{
		"backup/research-notes/draft-one.md",
		"backup/research-notes/final.pdf",
		"backup/root-file.txt",
	}
	if len(objects) != len(wantPaths) || len(repository.saved) != len(wantPaths) {
		t.Fatalf("object counts = (%d, %d), want %d", len(objects), len(repository.saved), len(wantPaths))
	}
	for index, wantPath := range wantPaths {
		if objects[index].Path != wantPath || repository.saved[index] != objects[index] {
			t.Errorf("object %d = %#v, want path %q", index, objects[index], wantPath)
		}
		if objects[index].Category != "backup" || objects[index].Tier != "GLACIER" {
			t.Errorf("object %d routing = %#v", index, objects[index])
		}
		if !objects[index].CreatedAt.Equal(now) || !objects[index].UpdatedAt.Equal(now) {
			t.Errorf(
				"object %d timestamps = (%v, %v), want %v",
				index,
				objects[index].CreatedAt,
				objects[index].UpdatedAt,
				now,
			)
		}
	}
}

func TestLiftServiceRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		prepare func(*testing.T) string
		ctx     func(*testing.T) context.Context
	}{
		{
			name:    "empty path",
			prepare: func(*testing.T) string { return "" },
			ctx:     func(t *testing.T) context.Context { return t.Context() },
		},
		{
			name:    "missing file",
			prepare: func(t *testing.T) string { return filepath.Join(t.TempDir(), "missing") },
			ctx:     func(t *testing.T) context.Context { return t.Context() },
		},
		{
			name:    "empty directory",
			prepare: func(t *testing.T) string { return t.TempDir() },
			ctx:     func(t *testing.T) context.Context { return t.Context() },
		},
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
				return link + string(os.PathSeparator)
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

func TestLiftServiceRejectsSymbolicLinkInDirectory(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	target := filepath.Join(directory, "target.txt")
	writeLiftFixture(t, target, "data")
	if err := os.Symlink(target, filepath.Join(directory, "link.txt")); err != nil {
		t.Fatalf("create symbolic link: %v", err)
	}

	repository := &stubRepository{}
	service, err := NewLiftService(repository, newTestCatalog(t), time.Now)
	if err != nil {
		t.Fatalf("NewLiftService() error: %v", err)
	}
	_, err = service.Lift(t.Context(), directory, "backup")
	if err == nil || !strings.Contains(err.Error(), "symbolic links are not supported") {
		t.Fatalf("Lift() error = %v, want symbolic link error", err)
	}
	if len(repository.saved) != 0 {
		t.Errorf("Save() calls = %d, want 0", len(repository.saved))
	}
}

func TestLiftServiceRejectsNormalizedPathConflicts(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	writeLiftFixture(t, filepath.Join(directory, "My File.txt"), "one")
	writeLiftFixture(t, filepath.Join(directory, "my-file.txt"), "two")

	repository := &stubRepository{}
	service, err := NewLiftService(repository, newTestCatalog(t), time.Now)
	if err != nil {
		t.Fatalf("NewLiftService() error: %v", err)
	}
	_, err = service.Lift(t.Context(), directory, "backup")
	if err == nil || !strings.Contains(err.Error(), "normalized path conflicts") {
		t.Fatalf("Lift() error = %v, want conflict error", err)
	}
	if len(repository.saved) != 0 {
		t.Errorf("Save() calls = %d, want 0", len(repository.saved))
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
	if err == nil || !strings.Contains(err.Error(), "save object \"backup/file.txt\": save failed") {
		t.Fatalf("Lift() error = %v, want wrapped repository error", err)
	}
}

func writeLiftFixture(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
}
