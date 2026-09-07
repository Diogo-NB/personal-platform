package application

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/domain/category"
	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/domain/object"
	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/port/in"
	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/port/out"
)

var _ in.Lifter = (*LiftService)(nil)

type LiftService struct {
	repository out.ObjectRepository
	catalog    *category.Catalog
	now        func() time.Time
}

func NewLiftService(
	repository out.ObjectRepository,
	catalog *category.Catalog,
	now func() time.Time,
) (*LiftService, error) {
	if repository == nil {
		return nil, errors.New("object repository must not be nil")
	}
	if catalog == nil {
		return nil, errors.New("category catalog must not be nil")
	}
	if now == nil {
		return nil, errors.New("clock must not be nil")
	}

	return &LiftService{
		repository: repository,
		catalog:    catalog,
		now:        now,
	}, nil
}

func (s *LiftService) Lift(
	ctx context.Context,
	sourcePath string,
	categoryPath string,
) ([]object.Object, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("lift objects: %w", err)
	}
	if sourcePath == "" {
		return nil, errors.New("inspect source: path must not be empty")
	}
	sourcePath = filepath.Clean(sourcePath)

	info, err := os.Lstat(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("inspect source %q: %w", sourcePath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("inspect source %q: symbolic links are not supported", sourcePath)
	}
	if !info.Mode().IsRegular() && !info.IsDir() {
		return nil, fmt.Errorf("inspect source %q: not a regular file or directory", sourcePath)
	}

	resolution, err := s.catalog.Resolve(categoryPath)
	if err != nil {
		return nil, err
	}

	files, err := collectFiles(ctx, sourcePath, info)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("inspect directory %q: no regular files found", sourcePath)
	}

	timestamp := s.now()
	objects := make([]object.Object, 0, len(files))
	paths := make(map[string]string, len(files))
	for _, file := range files {
		storedObject, err := object.New(object.NewParams{
			RelativePath: file.relativePath,
			Category:     resolution.Category,
			Size:         file.size,
			Tier:         resolution.Tier,
			Timestamp:    timestamp,
		})
		if err != nil {
			return nil, fmt.Errorf("create object for %q: %w", file.path, err)
		}
		if otherSource, exists := paths[storedObject.Path]; exists {
			return nil, fmt.Errorf(
				"create object for %q: normalized path conflicts with %q",
				file.path,
				otherSource,
			)
		}
		paths[storedObject.Path] = file.path
		objects = append(objects, storedObject)
	}

	for _, storedObject := range objects {
		if err := s.repository.Save(ctx, storedObject); err != nil {
			return nil, fmt.Errorf("save object %q: %w", storedObject.Path, err)
		}
	}

	return objects, nil
}
