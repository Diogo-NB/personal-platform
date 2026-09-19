package lifecycle

import (
	"errors"
	"time"
)

type State string

const (
	StateStopped  State = "stopped"
	StateStarting State = "starting"
	StateRunning  State = "running"
	StateStopping State = "stopping"
)

type Snapshot struct {
	Desired int32
	Running int32
	Pending int32
}

type RunningTask struct {
	StartedAt time.Time
}

type Status struct {
	State         State
	StartedAt     *time.Time
	UptimeSeconds *int64
}

func (s Snapshot) ValidateNonNegative() error {
	hasNegativeDesired := s.Desired < 0
	hasNegativeRunning := s.Running < 0
	hasNegativePending := s.Pending < 0
	if hasNegativeDesired || hasNegativeRunning || hasNegativePending {
		return errors.New("lifecycle: scheduler counts cannot be negative")
	}

	return nil
}

func (s Snapshot) ValidateSingleton() error {
	if err := s.ValidateNonNegative(); err != nil {
		return err
	}
	if s.Desired > 1 {
		return errors.New("lifecycle: desired count exceeds singleton limit")
	}

	hasMultipleRunning := s.Running > 1
	hasMultiplePending := s.Pending > 1
	hasMultipleActive := s.Running+s.Pending > 1
	if hasMultipleRunning || hasMultiplePending || hasMultipleActive {
		return errors.New("lifecycle: scheduler counts violate singleton limit")
	}

	return nil
}

func (s Snapshot) State() (State, error) {
	if err := s.ValidateSingleton(); err != nil {
		return "", err
	}
	if s.Desired == 0 {
		if s.Running == 0 && s.Pending == 0 {
			return StateStopped, nil
		}

		return StateStopping, nil
	}
	if s.Running == 1 && s.Pending == 0 {
		return StateRunning, nil
	}

	return StateStarting, nil
}

func RunningStatus(task RunningTask, now time.Time) (Status, error) {
	now = now.UTC()
	startedAt := task.StartedAt.UTC()
	if startedAt.After(now) {
		return Status{}, errors.New("lifecycle: running task start time is in the future")
	}

	uptimeSeconds := int64(now.Sub(startedAt) / time.Second)
	return Status{
		State:         StateRunning,
		StartedAt:     &startedAt,
		UptimeSeconds: &uptimeSeconds,
	}, nil
}
