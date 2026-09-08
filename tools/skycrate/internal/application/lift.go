package application

import (
	"context"
	"errors"
	"fmt"
	"math"
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
	request in.LiftRequest,
) (in.LiftResult, error) {
	if err := ctx.Err(); err != nil {
		return in.LiftResult{}, fmt.Errorf("lift objects: %w", err)
	}
	if request.Approve == nil {
		return in.LiftResult{}, errors.New("lift approval must not be nil")
	}

	sourcePath := request.SourcePath
	if sourcePath == "" {
		return in.LiftResult{}, errors.New("inspect source: path must not be empty")
	}
	sourcePath = filepath.Clean(sourcePath)

	info, err := os.Lstat(sourcePath)
	if err != nil {
		return in.LiftResult{}, fmt.Errorf("inspect source %q: %w", sourcePath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return in.LiftResult{}, fmt.Errorf(
			"inspect source %q: symbolic links are not supported",
			sourcePath,
		)
	}
	if !info.Mode().IsRegular() && !info.IsDir() {
		return in.LiftResult{}, fmt.Errorf(
			"inspect source %q: not a regular file or directory",
			sourcePath,
		)
	}

	resolution, err := s.catalog.Resolve(request.Category)
	if err != nil {
		return in.LiftResult{}, err
	}

	files, err := collectFiles(ctx, sourcePath, info)
	if err != nil {
		return in.LiftResult{}, err
	}
	if len(files) == 0 {
		return in.LiftResult{}, fmt.Errorf(
			"inspect directory %q: no regular files found",
			sourcePath,
		)
	}

	timestamp := s.now()
	objects := make([]object.Object, 0, len(files))
	paths := make(map[string]string, len(files))
	var totalBytes int64
	for _, file := range files {
		storedObject, err := object.New(object.NewParams{
			RelativePath: file.relativePath,
			Category:     resolution.Category,
			Size:         file.size,
			Tier:         resolution.Tier,
			Timestamp:    timestamp,
		})
		if err != nil {
			return in.LiftResult{}, fmt.Errorf("create object for %q: %w", file.path, err)
		}
		if otherSource, exists := paths[storedObject.Path]; exists {
			return in.LiftResult{}, fmt.Errorf(
				"create object for %q: normalized path conflicts with %q",
				file.path,
				otherSource,
			)
		}
		if file.size > math.MaxInt64-totalBytes {
			return in.LiftResult{}, errors.New("calculate lift size: total size exceeds int64")
		}

		paths[storedObject.Path] = file.path
		objects = append(objects, storedObject)
		totalBytes += file.size
	}

	objectSummaries := make([]in.LiftObjectSummary, len(objects))
	for index, storedObject := range objects {
		objectSummaries[index] = in.LiftObjectSummary{
			Path:      storedObject.Path,
			SizeBytes: storedObject.Size,
		}
	}

	approved, err := request.Approve(ctx, in.LiftSummary{
		ObjectCount: len(objects),
		Objects:     objectSummaries,
		TotalBytes:  totalBytes,
		Tier:        resolution.Tier,
	})
	if err != nil {
		return in.LiftResult{}, fmt.Errorf("approve lift: %w", err)
	}
	if !approved {
		return in.LiftResult{
			Objects:    []object.Object{},
			IsCanceled: true,
		}, nil
	}

	for _, storedObject := range objects {
		sourcePath := paths[storedObject.Path]
		if err := s.repository.Save(ctx, out.SaveRequest{
			SourcePath:   sourcePath,
			StoredObject: storedObject,
		}); err != nil {
			return in.LiftResult{}, fmt.Errorf("save object %q: %w", storedObject.Path, err)
		}
	}

	return in.LiftResult{
		Objects:    objects,
		IsCanceled: false,
	}, nil
}
