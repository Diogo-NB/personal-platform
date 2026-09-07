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
	filePath string,
	categoryPath string,
) (object.Object, error) {
	if err := ctx.Err(); err != nil {
		return object.Object{}, fmt.Errorf("lift object: %w", err)
	}

	info, err := os.Lstat(filePath)
	if err != nil {
		return object.Object{}, fmt.Errorf("inspect file %q: %w", filePath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return object.Object{}, fmt.Errorf("inspect file %q: symbolic links are not supported", filePath)
	}
	if !info.Mode().IsRegular() {
		return object.Object{}, fmt.Errorf("inspect file %q: not a regular file", filePath)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return object.Object{}, fmt.Errorf("open file %q: %w", filePath, err)
	}
	if err := file.Close(); err != nil {
		return object.Object{}, fmt.Errorf("close file %q: %w", filePath, err)
	}

	resolution, err := s.catalog.Resolve(categoryPath)
	if err != nil {
		return object.Object{}, err
	}

	storedObject, err := object.New(object.NewParams{
		Name:      filepath.Base(filePath),
		Category:  resolution.Category,
		Size:      info.Size(),
		Tier:      resolution.Tier,
		Timestamp: s.now(),
	})
	if err != nil {
		return object.Object{}, fmt.Errorf("create object: %w", err)
	}

	if err := s.repository.Save(ctx, storedObject); err != nil {
		return object.Object{}, fmt.Errorf("save object: %w", err)
	}

	return storedObject, nil
}
