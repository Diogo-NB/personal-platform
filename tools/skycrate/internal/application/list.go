package application

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/domain/storage"
	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/port/in"
	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/port/out"
)

var _ in.Lister = (*ListService)(nil)

type ListService struct {
	repository out.ObjectRepository
}

func NewListService(repository out.ObjectRepository) (*ListService, error) {
	if repository == nil {
		return nil, errors.New("object repository must not be nil")
	}

	return &ListService{repository: repository}, nil
}

func (s *ListService) List(ctx context.Context) (in.ListSummary, error) {
	if err := ctx.Err(); err != nil {
		return in.ListSummary{}, fmt.Errorf("list objects: %w", err)
	}

	objects, err := s.repository.FindMany(ctx)
	if err != nil {
		return in.ListSummary{}, fmt.Errorf("find objects: %w", err)
	}

	summary := in.ListSummary{
		Tiers: []in.TierSummary{
			{Tier: storage.TierDefault},
			{Tier: storage.TierArchive},
			{Tier: storage.TierCold},
			{Tier: storage.TierInstant},
		},
		ObjectCount: len(objects),
	}
	tierIndexes := map[storage.Tier]int{
		storage.TierDefault: 0,
		storage.TierArchive: 1,
		storage.TierCold:    2,
		storage.TierInstant: 3,
	}
	for _, storedObject := range objects {
		if storedObject.Size > 0 && summary.TotalBytes > math.MaxInt64-storedObject.Size {
			return in.ListSummary{}, errors.New("summarize objects: total size overflows int64")
		}
		tierIndex, exists := tierIndexes[storedObject.Tier]
		if !exists {
			return in.ListSummary{}, fmt.Errorf("summarize objects: %w", storedObject.Tier.Validate())
		}

		summary.TotalBytes += storedObject.Size
		summary.Tiers[tierIndex].ObjectCount++
		summary.Tiers[tierIndex].TotalBytes += storedObject.Size
	}

	return summary, nil
}
