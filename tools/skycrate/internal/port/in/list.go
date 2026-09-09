package in

import (
	"context"

	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/domain/storage"
)

type TierSummary struct {
	Tier        storage.Tier
	ObjectCount int
	TotalBytes  int64
}

type ListSummary struct {
	Tiers       []TierSummary
	ObjectCount int
	TotalBytes  int64
}

type Lister interface {
	List(ctx context.Context) (ListSummary, error)
}
