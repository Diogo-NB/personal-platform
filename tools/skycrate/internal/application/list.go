package application

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/domain/category"
	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/port/in"
	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/port/out"
	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/util"
)

var _ in.Lister = (*ListService)(nil)

type ListService struct {
	repository out.ObjectRepository
	catalog    *category.Catalog
}

func NewListService(
	repository out.ObjectRepository,
	catalog *category.Catalog,
) (*ListService, error) {
	if repository == nil {
		return nil, errors.New("object repository must not be nil")
	}
	if catalog == nil {
		return nil, errors.New("category catalog must not be nil")
	}

	return &ListService{repository: repository, catalog: catalog}, nil
}

func (s *ListService) List(
	ctx context.Context,
	request in.ListRequest,
) (in.Summary, error) {
	if err := ctx.Err(); err != nil {
		return in.Summary{}, fmt.Errorf("list objects: %w", err)
	}

	normalized := out.FindManyRequest{}
	if request.Category != "" {
		resolution, err := s.catalog.Resolve(request.Category)
		if err != nil {
			return in.Summary{}, err
		}
		normalized.Category = resolution.Category
	}
	if request.Tier != "" {
		tier, err := util.Normalize(request.Tier)
		if err != nil {
			return in.Summary{}, fmt.Errorf("normalize tier: %w", err)
		}
		normalized.Tier = tier
	}

	objects, err := s.repository.FindMany(ctx, normalized)
	if err != nil {
		return in.Summary{}, fmt.Errorf("find objects: %w", err)
	}

	summary := in.Summary{ObjectCount: len(objects)}
	for _, storedObject := range objects {
		if storedObject.Size > 0 && summary.TotalBytes > math.MaxInt64-storedObject.Size {
			return in.Summary{}, errors.New("summarize objects: total size overflows int64")
		}
		summary.TotalBytes += storedObject.Size
	}

	return summary, nil
}
