package application

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/diogo-nb/personal-platform/apps/ts6-management-api/internal/domain/lifecycle"
)

type fakeScheduler struct {
	snapshot         lifecycle.Snapshot
	snapshotError    error
	runningTask      lifecycle.RunningTask
	runningTaskError error
	setError         error
	desiredUpdates   []int32
	contexts         []context.Context
}

func (f *fakeScheduler) Snapshot(ctx context.Context) (lifecycle.Snapshot, error) {
	f.contexts = append(f.contexts, ctx)
	return f.snapshot, f.snapshotError
}

func (f *fakeScheduler) SetDesiredCount(ctx context.Context, desired int32) error {
	f.contexts = append(f.contexts, ctx)
	f.desiredUpdates = append(f.desiredUpdates, desired)
	return f.setError
}

func (f *fakeScheduler) RunningTask(ctx context.Context) (lifecycle.RunningTask, error) {
	f.contexts = append(f.contexts, ctx)
	return f.runningTask, f.runningTaskError
}

type fakeClock struct {
	now time.Time
}

func (f fakeClock) Now() time.Time {
	return f.now
}

func TestNewLifecycleService(t *testing.T) {
	t.Parallel()

	scheduler := &fakeScheduler{}
	if _, err := NewLifecycleService(nil, fakeClock{}); err == nil {
		t.Error("nil scheduler error = nil, want error")
	}
	if _, err := NewLifecycleService(scheduler, nil); err == nil {
		t.Error("nil clock error = nil, want error")
	}
	if _, err := NewLifecycleService(scheduler, fakeClock{}); err != nil {
		t.Errorf("valid dependencies error = %v", err)
	}
}

func TestLifecycleService_Start(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		snapshot      lifecycle.Snapshot
		snapshotError error
		setError      error
		wantUpdates   []int32
		wantError     bool
	}{
		{name: "stopped service starts", snapshot: lifecycle.Snapshot{}, wantUpdates: []int32{1}},
		{name: "starting service is unchanged", snapshot: lifecycle.Snapshot{Desired: 1, Pending: 1}},
		{name: "running service is unchanged", snapshot: lifecycle.Snapshot{Desired: 1, Running: 1}},
		{name: "stopping service starts again", snapshot: lifecycle.Snapshot{Running: 1}, wantUpdates: []int32{1}},
		{name: "oversized desired count fails", snapshot: lifecycle.Snapshot{Desired: 2, Running: 1}, wantError: true},
		{name: "singleton violation fails", snapshot: lifecycle.Snapshot{Desired: 1, Running: 1, Pending: 1}, wantError: true},
		{name: "snapshot failure", snapshotError: errors.New("unavailable"), wantError: true},
		{name: "update failure", setError: errors.New("unavailable"), wantUpdates: []int32{1}, wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			scheduler := &fakeScheduler{
				snapshot:      test.snapshot,
				snapshotError: test.snapshotError,
				setError:      test.setError,
			}
			service := newService(t, scheduler, fakeClock{})
			err := service.Start(t.Context())
			if (err != nil) != test.wantError {
				t.Fatalf("Start() error = %v, wantError %t", err, test.wantError)
			}
			if !reflect.DeepEqual(scheduler.desiredUpdates, test.wantUpdates) {
				t.Errorf("desired updates = %v, want %v", scheduler.desiredUpdates, test.wantUpdates)
			}
		})
	}
}

func TestLifecycleService_Stop(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		snapshot      lifecycle.Snapshot
		snapshotError error
		setError      error
		wantUpdates   []int32
		wantError     bool
	}{
		{name: "running service stops", snapshot: lifecycle.Snapshot{Desired: 1, Running: 1}, wantUpdates: []int32{0}},
		{name: "stopped service is unchanged", snapshot: lifecycle.Snapshot{}},
		{name: "oversized service safely stops", snapshot: lifecycle.Snapshot{Desired: 3, Running: 2, Pending: 1}, wantUpdates: []int32{0}},
		{name: "negative counts fail", snapshot: lifecycle.Snapshot{Desired: -1}, wantError: true},
		{name: "snapshot failure", snapshotError: errors.New("unavailable"), wantError: true},
		{name: "update failure", snapshot: lifecycle.Snapshot{Desired: 1}, setError: errors.New("unavailable"), wantUpdates: []int32{0}, wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			scheduler := &fakeScheduler{
				snapshot:      test.snapshot,
				snapshotError: test.snapshotError,
				setError:      test.setError,
			}
			service := newService(t, scheduler, fakeClock{})
			err := service.Stop(t.Context())
			if (err != nil) != test.wantError {
				t.Fatalf("Stop() error = %v, wantError %t", err, test.wantError)
			}
			if !reflect.DeepEqual(scheduler.desiredUpdates, test.wantUpdates) {
				t.Errorf("desired updates = %v, want %v", scheduler.desiredUpdates, test.wantUpdates)
			}
		})
	}
}

func TestLifecycleService_Status(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 19, 18, 30, 0, 500_000_000, time.UTC)
	startedAt := now.Add(-90*time.Second - 900*time.Millisecond)
	tests := []struct {
		name             string
		scheduler        *fakeScheduler
		want             lifecycle.Status
		wantRunningCalls int
		wantError        bool
	}{
		{
			name:      "stopped status does not request a task",
			scheduler: &fakeScheduler{},
			want:      lifecycle.Status{State: lifecycle.StateStopped},
		},
		{
			name: "running status uses scheduler task and clock",
			scheduler: &fakeScheduler{
				snapshot:    lifecycle.Snapshot{Desired: 1, Running: 1},
				runningTask: lifecycle.RunningTask{StartedAt: startedAt},
			},
			want: lifecycle.Status{
				State:         lifecycle.StateRunning,
				StartedAt:     timePointer(startedAt.UTC()),
				UptimeSeconds: int64Pointer(90),
			},
			wantRunningCalls: 1,
		},
		{
			name:      "snapshot failure",
			scheduler: &fakeScheduler{snapshotError: errors.New("unavailable")},
			wantError: true,
		},
		{
			name: "singleton violation",
			scheduler: &fakeScheduler{
				snapshot: lifecycle.Snapshot{Desired: 1, Running: 1, Pending: 1},
			},
			wantError: true,
		},
		{
			name: "running task failure",
			scheduler: &fakeScheduler{
				snapshot:         lifecycle.Snapshot{Desired: 1, Running: 1},
				runningTaskError: errors.New("unavailable"),
			},
			wantRunningCalls: 1,
			wantError:        true,
		},
		{
			name: "future task start",
			scheduler: &fakeScheduler{
				snapshot:    lifecycle.Snapshot{Desired: 1, Running: 1},
				runningTask: lifecycle.RunningTask{StartedAt: now.Add(time.Second)},
			},
			wantRunningCalls: 1,
			wantError:        true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			service := newService(t, test.scheduler, fakeClock{now: now})
			got, err := service.Status(t.Context())
			if (err != nil) != test.wantError {
				t.Fatalf("Status() error = %v, wantError %t", err, test.wantError)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("Status() = %#v, want %#v", got, test.want)
			}
			gotRunningCalls := len(test.scheduler.contexts) - 1
			if gotRunningCalls != test.wantRunningCalls {
				t.Errorf("running task calls = %d, want %d", gotRunningCalls, test.wantRunningCalls)
			}
		})
	}
}

func TestLifecycleService_PropagatesContext(t *testing.T) {
	t.Parallel()

	type contextKey string
	ctx := context.WithValue(t.Context(), contextKey("request"), "value")
	now := time.Now().UTC()
	statusScheduler := &fakeScheduler{
		snapshot:    lifecycle.Snapshot{Desired: 1, Running: 1},
		runningTask: lifecycle.RunningTask{StartedAt: now},
	}
	statusService := newService(t, statusScheduler, fakeClock{now: now})

	if _, err := statusService.Status(ctx); err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	for i, got := range statusScheduler.contexts {
		if got != ctx {
			t.Errorf("status context %d was not propagated", i)
		}
	}

	startScheduler := &fakeScheduler{}
	if err := newService(t, startScheduler, fakeClock{}).Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	for i, got := range startScheduler.contexts {
		if got != ctx {
			t.Errorf("start context %d was not propagated", i)
		}
	}

	stopScheduler := &fakeScheduler{snapshot: lifecycle.Snapshot{Desired: 1}}
	if err := newService(t, stopScheduler, fakeClock{}).Stop(ctx); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	for i, got := range stopScheduler.contexts {
		if got != ctx {
			t.Errorf("stop context %d was not propagated", i)
		}
	}
}

func newService(t *testing.T, scheduler *fakeScheduler, clock fakeClock) *LifecycleService {
	t.Helper()

	service, err := NewLifecycleService(scheduler, clock)
	if err != nil {
		t.Fatalf("NewLifecycleService() error = %v", err)
	}

	return service
}

func timePointer(value time.Time) *time.Time {
	return &value
}

func int64Pointer(value int64) *int64 {
	return &value
}
