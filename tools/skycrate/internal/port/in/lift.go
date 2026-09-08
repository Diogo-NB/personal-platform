package in

import (
	"context"

	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/domain/object"
	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/domain/storage"
)

type LiftApproval func(ctx context.Context, summary LiftSummary) (bool, error)

type LiftRequest struct {
	SourcePath string
	Category   string
	Approve    LiftApproval
}

type LiftSummary struct {
	ObjectCount int
	TotalBytes  int64
	Tier        storage.Tier
}

type LiftResult struct {
	Objects    []object.Object
	IsCanceled bool
}

type Lifter interface {
	Lift(ctx context.Context, request LiftRequest) (LiftResult, error)
}
