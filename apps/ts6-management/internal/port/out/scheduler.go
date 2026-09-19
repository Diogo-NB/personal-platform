package out

import (
	"context"
	"time"

	"github.com/diogo-nb/personal-platform/apps/ts6-management/internal/domain/lifecycle"
)

type Scheduler interface {
	Snapshot(context.Context) (lifecycle.Snapshot, error)
	SetDesiredCount(context.Context, int32) error
	RunningTask(context.Context) (lifecycle.RunningTask, error)
}

type Clock interface {
	Now() time.Time
}
