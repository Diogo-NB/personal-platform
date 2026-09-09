package application

import (
	"context"
	"testing"
	"time"

	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/domain/category"
	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/domain/object"
	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/domain/storage"
	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/port/out"
)

type stubRepository struct {
	saved   []out.SaveRequest
	objects []object.Object
	saveErr error
	findErr error
}

func (r *stubRepository) Save(_ context.Context, request out.SaveRequest) error {
	r.saved = append(r.saved, request)
	return r.saveErr
}

func (r *stubRepository) FindMany(_ context.Context) ([]object.Object, error) {
	return append([]object.Object{}, r.objects...), r.findErr
}

func newTestCatalog(t *testing.T) *category.Catalog {
	t.Helper()

	catalog, err := category.NewCatalog(map[string]storage.Tier{
		"backup":            storage.TierCold,
		"backup/university": storage.TierArchive,
	})
	if err != nil {
		t.Fatalf("category.NewCatalog() error: %v", err)
	}
	return catalog
}

func newTestObject(t *testing.T, name string, size int64, tier storage.Tier) object.Object {
	t.Helper()

	storedObject, err := object.New(object.NewParams{
		RelativePath: name,
		Category:     "backup",
		Size:         size,
		Tier:         tier,
		Timestamp:    time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("object.New() error: %v", err)
	}
	return storedObject
}
