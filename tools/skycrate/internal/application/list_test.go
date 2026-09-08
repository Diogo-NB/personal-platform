package application

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/domain/object"
	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/domain/storage"
	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/port/in"
	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/port/out"
)

func TestNewListService(t *testing.T) {
	t.Parallel()

	if _, err := NewListService(nil, newTestCatalog(t)); err == nil {
		t.Fatal("NewListService(nil) error = nil")
	}
	if _, err := NewListService(&stubRepository{}, nil); err == nil {
		t.Fatal("NewListService(nil catalog) error = nil")
	}
}

func TestListServiceList(t *testing.T) {
	t.Parallel()

	repository := &stubRepository{objects: []object.Object{
		newTestObject(t, "first.txt", 1_000_000_000),
		newTestObject(t, "second.txt", 2_000_000_000),
	}}
	service, err := NewListService(repository, newTestCatalog(t))
	if err != nil {
		t.Fatalf("NewListService() error: %v", err)
	}

	got, err := service.List(t.Context(), in.ListRequest{
		Category: " BACKUP / University ",
		Tier:     "ARCHIVE",
	})
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	wantRequest := out.FindManyRequest{Category: "backup/university", Tier: storage.TierArchive}
	if repository.findRequest != wantRequest {
		t.Errorf("FindMany() request = %#v, want %#v", repository.findRequest, wantRequest)
	}
	if got.ObjectCount != 2 || got.TotalBytes != 3_000_000_000 {
		t.Errorf("List() = %#v", got)
	}
}

func TestListServiceRejectsTierWhitespace(t *testing.T) {
	t.Parallel()

	repository := &stubRepository{}
	service, err := NewListService(repository, newTestCatalog(t))
	if err != nil {
		t.Fatalf("NewListService() error: %v", err)
	}
	if _, err := service.List(t.Context(), in.ListRequest{Tier: " archive "}); err == nil {
		t.Fatal("List() error = nil, want tier validation error")
	}
}

func TestListServicePropagatesRepositoryError(t *testing.T) {
	t.Parallel()

	repository := &stubRepository{findErr: errors.New("find failed")}
	service, err := NewListService(repository, newTestCatalog(t))
	if err != nil {
		t.Fatalf("NewListService() error: %v", err)
	}
	_, err = service.List(t.Context(), in.ListRequest{})
	if err == nil || !strings.Contains(err.Error(), "find objects: find failed") {
		t.Fatalf("List() error = %v, want wrapped repository error", err)
	}
}

func TestListServiceRejectsTotalOverflow(t *testing.T) {
	t.Parallel()

	repository := &stubRepository{objects: []object.Object{
		newTestObject(t, "first.txt", math.MaxInt64),
		newTestObject(t, "second.txt", 1),
	}}
	service, err := NewListService(repository, newTestCatalog(t))
	if err != nil {
		t.Fatalf("NewListService() error: %v", err)
	}
	if _, err := service.List(t.Context(), in.ListRequest{}); err == nil {
		t.Fatal("List() error = nil, want overflow error")
	}
}
