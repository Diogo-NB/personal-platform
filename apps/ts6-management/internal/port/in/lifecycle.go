package in

import (
	"context"

	"github.com/diogo-nb/personal-platform/apps/ts6-management/internal/domain/lifecycle"
)

type Lifecycle interface {
	Start(context.Context) error
	Stop(context.Context) error
	Status(context.Context) (lifecycle.Status, error)
}
