package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/diogo-nb/personal-platform/apps/ts6-management/internal/domain/lifecycle"
	portin "github.com/diogo-nb/personal-platform/apps/ts6-management/internal/port/in"
	portout "github.com/diogo-nb/personal-platform/apps/ts6-management/internal/port/out"
)

type LifecycleService struct {
	scheduler portout.Scheduler
	clock     portout.Clock
}

func NewLifecycleService(scheduler portout.Scheduler, clock portout.Clock) (*LifecycleService, error) {
	if scheduler == nil {
		return nil, errors.New("application: scheduler is required")
	}
	if clock == nil {
		return nil, errors.New("application: clock is required")
	}

	return &LifecycleService{scheduler: scheduler, clock: clock}, nil
}

func (s *LifecycleService) Start(ctx context.Context) error {
	snapshot, err := s.scheduler.Snapshot(ctx)
	if err != nil {
		return fmt.Errorf("describing service before start: %w", err)
	}
	if err := snapshot.ValidateSingleton(); err != nil {
		return err
	}
	if snapshot.Desired == 1 {
		return nil
	}
	if err := s.scheduler.SetDesiredCount(ctx, 1); err != nil {
		return fmt.Errorf("starting service: %w", err)
	}

	return nil
}

func (s *LifecycleService) Stop(ctx context.Context) error {
	snapshot, err := s.scheduler.Snapshot(ctx)
	if err != nil {
		return fmt.Errorf("describing service before stop: %w", err)
	}
	if err := snapshot.ValidateNonNegative(); err != nil {
		return err
	}
	if snapshot.Desired == 0 {
		return nil
	}
	if err := s.scheduler.SetDesiredCount(ctx, 0); err != nil {
		return fmt.Errorf("stopping service: %w", err)
	}

	return nil
}

func (s *LifecycleService) Status(ctx context.Context) (lifecycle.Status, error) {
	snapshot, err := s.scheduler.Snapshot(ctx)
	if err != nil {
		return lifecycle.Status{}, fmt.Errorf("describing service status: %w", err)
	}

	state, err := snapshot.State()
	if err != nil {
		return lifecycle.Status{}, err
	}
	if state != lifecycle.StateRunning {
		return lifecycle.Status{State: state}, nil
	}

	task, err := s.scheduler.RunningTask(ctx)
	if err != nil {
		return lifecycle.Status{}, fmt.Errorf("describing running task: %w", err)
	}

	return lifecycle.RunningStatus(task, s.clock.Now())
}

var _ portin.Lifecycle = (*LifecycleService)(nil)
