package lifecycle

import (
	"testing"
	"time"
)

func TestSnapshot_ValidateNonNegative(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		snapshot  Snapshot
		wantError bool
	}{
		{name: "zero counts", snapshot: Snapshot{}},
		{name: "oversized counts", snapshot: Snapshot{Desired: 3, Running: 2, Pending: 1}},
		{name: "negative desired", snapshot: Snapshot{Desired: -1}, wantError: true},
		{name: "negative running", snapshot: Snapshot{Running: -1}, wantError: true},
		{name: "negative pending", snapshot: Snapshot{Pending: -1}, wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := test.snapshot.ValidateNonNegative()
			if (err != nil) != test.wantError {
				t.Fatalf("ValidateNonNegative() error = %v, wantError %t", err, test.wantError)
			}
		})
	}
}

func TestSnapshot_ValidateSingleton(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		snapshot  Snapshot
		wantError bool
	}{
		{name: "stopped", snapshot: Snapshot{}},
		{name: "single pending", snapshot: Snapshot{Desired: 1, Pending: 1}},
		{name: "single running", snapshot: Snapshot{Desired: 1, Running: 1}},
		{name: "negative count", snapshot: Snapshot{Desired: -1}, wantError: true},
		{name: "oversized desired", snapshot: Snapshot{Desired: 2}, wantError: true},
		{name: "multiple running", snapshot: Snapshot{Desired: 1, Running: 2}, wantError: true},
		{name: "multiple pending", snapshot: Snapshot{Desired: 1, Pending: 2}, wantError: true},
		{name: "running and pending", snapshot: Snapshot{Desired: 1, Running: 1, Pending: 1}, wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := test.snapshot.ValidateSingleton()
			if (err != nil) != test.wantError {
				t.Fatalf("ValidateSingleton() error = %v, wantError %t", err, test.wantError)
			}
		})
	}
}

func TestSnapshot_State(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		snapshot  Snapshot
		want      State
		wantError bool
	}{
		{name: "stopped", snapshot: Snapshot{}, want: StateStopped},
		{name: "starting without pending task", snapshot: Snapshot{Desired: 1}, want: StateStarting},
		{name: "starting with pending task", snapshot: Snapshot{Desired: 1, Pending: 1}, want: StateStarting},
		{name: "running", snapshot: Snapshot{Desired: 1, Running: 1}, want: StateRunning},
		{name: "stopping running task", snapshot: Snapshot{Running: 1}, want: StateStopping},
		{name: "stopping pending task", snapshot: Snapshot{Pending: 1}, want: StateStopping},
		{name: "singleton violation", snapshot: Snapshot{Desired: 1, Running: 1, Pending: 1}, wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := test.snapshot.State()
			if (err != nil) != test.wantError {
				t.Fatalf("State() error = %v, wantError %t", err, test.wantError)
			}
			if got != test.want {
				t.Errorf("State() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestRunningStatus(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 19, 15, 30, 0, 500_000_000, time.FixedZone("BRT", -3*60*60))
	startedAt := now.Add(-1234*time.Second - 900*time.Millisecond)

	got, err := RunningStatus(RunningTask{StartedAt: startedAt}, now)
	if err != nil {
		t.Fatalf("RunningStatus() error = %v", err)
	}
	if got.State != StateRunning {
		t.Errorf("state = %q, want %q", got.State, StateRunning)
	}
	if got.StartedAt == nil || !got.StartedAt.Equal(startedAt.UTC()) || got.StartedAt.Location() != time.UTC {
		t.Errorf("started at = %v, want UTC %v", got.StartedAt, startedAt.UTC())
	}
	if got.UptimeSeconds == nil || *got.UptimeSeconds != 1234 {
		t.Errorf("uptime seconds = %v, want 1234", got.UptimeSeconds)
	}
}

func TestRunningStatusRejectsFutureStart(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 19, 18, 30, 0, 0, time.UTC)
	if _, err := RunningStatus(RunningTask{StartedAt: now.Add(time.Nanosecond)}, now); err == nil {
		t.Fatal("RunningStatus() error = nil, want error")
	}
}
