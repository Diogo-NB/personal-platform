package out

import (
	"context"

	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/domain/object"
)

type FindManyRequest struct {
	Category string
	Tier     string
}

type ObjectRepository interface {
	Save(ctx context.Context, storedObject object.Object) error
	FindMany(ctx context.Context, request FindManyRequest) ([]object.Object, error)
}
