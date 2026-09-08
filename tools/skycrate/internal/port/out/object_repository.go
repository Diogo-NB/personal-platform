package out

import (
	"context"

	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/domain/object"
	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/domain/storage"
)

type FindManyRequest struct {
	Category string
	Tier     storage.Tier
}

type SaveRequest struct {
	SourcePath   string
	StoredObject object.Object
}

type ObjectRepository interface {
	Save(ctx context.Context, request SaveRequest) error
	FindMany(ctx context.Context, request FindManyRequest) ([]object.Object, error)
}
