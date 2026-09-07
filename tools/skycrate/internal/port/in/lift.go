package in

import (
	"context"

	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/domain/object"
)

type Lifter interface {
	Lift(ctx context.Context, filePath, category string) (object.Object, error)
}
