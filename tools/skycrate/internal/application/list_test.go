package application

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/domain/object"
	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/domain/storage"
	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/port/in"
)

func TestNewListService(t *testing.T) {
	t.Parallel()

	if _, err := NewListService(nil); err == nil {
		t.Fatal("NewListService(nil) error = nil")
	}
}

func TestListServiceList(t *testing.T) {
	t.Parallel()

	repository := &stubRepository{objects: []object.Object{
		newTestObject(t, "default.txt", 1_000_000_000, storage.TierDefault),
		newTestObject(t, "archive.txt", 2_000_000_000, storage.TierArchive),
		newTestObject(t, "cold.txt", 3_000_000_000, storage.TierCold),
		newTestObject(t, "instant.txt", 4_000_000_000, storage.TierInstant),
	}}
	service, err := NewListService(repository)
	if err != nil {
		t.Fatalf("NewListService() error: %v", err)
	}

	got, err := service.List(t.Context())
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	want := in.ListSummary{
		Tiers: []in.TierSummary{
			{Tier: storage.TierDefault, ObjectCount: 1, TotalBytes: 1_000_000_000},
			{Tier: storage.TierArchive, ObjectCount: 1, TotalBytes: 2_000_000_000},
			{Tier: storage.TierCold, ObjectCount: 1, TotalBytes: 3_000_000_000},
			{Tier: storage.TierInstant, ObjectCount: 1, TotalBytes: 4_000_000_000},
		},
		ObjectCount: 4,
		TotalBytes:  10_000_000_000,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("List() = %#v, want %#v", got, want)
	}
}

func TestListServiceIncludesEmptyTiers(t *testing.T) {
	t.Parallel()

	repository := &stubRepository{}
	service, err := NewListService(repository)
	if err != nil {
		t.Fatalf("NewListService() error: %v", err)
	}

	got, err := service.List(t.Context())
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	want := in.ListSummary{
		Tiers: []in.TierSummary{
			{Tier: storage.TierDefault},
			{Tier: storage.TierArchive},
			{Tier: storage.TierCold},
			{Tier: storage.TierInstant},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("List() = %#v, want %#v", got, want)
	}
}

func TestListServiceRejectsInvalidObjectTier(t *testing.T) {
	t.Parallel()

	storedObject := newTestObject(t, "invalid.txt", 1, storage.TierCold)
	storedObject.Tier = storage.TierUnknown
	repository := &stubRepository{objects: []object.Object{storedObject}}
	service, err := NewListService(repository)
	if err != nil {
		t.Fatalf("NewListService() error: %v", err)
	}
	if _, err := service.List(t.Context()); err == nil || !strings.Contains(err.Error(), "storage tier") {
		t.Fatalf("List() error = %v, want invalid tier error", err)
	}
}

func TestListServicePropagatesRepositoryError(t *testing.T) {
	t.Parallel()

	repository := &stubRepository{findErr: errors.New("find failed")}
	service, err := NewListService(repository)
	if err != nil {
		t.Fatalf("NewListService() error: %v", err)
	}
	_, err = service.List(t.Context())
	if err == nil || !strings.Contains(err.Error(), "find objects: find failed") {
		t.Fatalf("List() error = %v, want wrapped repository error", err)
	}
}

func TestListServicePropagatesCancellation(t *testing.T) {
	t.Parallel()

	service, err := NewListService(&stubRepository{})
	if err != nil {
		t.Fatalf("NewListService() error: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err = service.List(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("List() error = %v, want context canceled", err)
	}
}

func TestListServiceRejectsTotalOverflow(t *testing.T) {
	t.Parallel()

	repository := &stubRepository{objects: []object.Object{
		newTestObject(t, "first.txt", math.MaxInt64, storage.TierCold),
		newTestObject(t, "second.txt", 1, storage.TierArchive),
	}}
	service, err := NewListService(repository)
	if err != nil {
		t.Fatalf("NewListService() error: %v", err)
	}
	if _, err := service.List(t.Context()); err == nil {
		t.Fatal("List() error = nil, want overflow error")
	}
}
