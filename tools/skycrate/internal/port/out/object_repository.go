package out

import (
	"context"

	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/domain/object"
)

type SaveRequest struct {
	SourcePath   string
	StoredObject object.Object
}

type ObjectRepository interface {
	Save(ctx context.Context, request SaveRequest) error
	FindMany(ctx context.Context) ([]object.Object, error)
}
