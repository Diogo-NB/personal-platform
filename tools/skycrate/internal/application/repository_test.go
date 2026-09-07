package application

import (
	"context"
	"testing"
	"time"

	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/domain/category"
	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/domain/object"
	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/port/out"
)

type stubRepository struct {
	saved       []object.Object
	findRequest out.FindManyRequest
	objects     []object.Object
	saveErr     error
	findErr     error
}

func (r *stubRepository) Save(_ context.Context, storedObject object.Object) error {
	r.saved = append(r.saved, storedObject)
	return r.saveErr
}

func (r *stubRepository) FindMany(
	_ context.Context,
	request out.FindManyRequest,
) ([]object.Object, error) {
	r.findRequest = request
	return append([]object.Object{}, r.objects...), r.findErr
}

func newTestCatalog(t *testing.T) *category.Catalog {
	t.Helper()

	catalog, err := category.NewCatalog(map[string]string{
		"backup":            "GLACIER",
		"backup/university": "DEEP_ARCHIVE",
	})
	if err != nil {
		t.Fatalf("category.NewCatalog() error: %v", err)
	}
	return catalog
}

func newTestObject(t *testing.T, name string, size int64) object.Object {
	t.Helper()

	storedObject, err := object.New(object.NewParams{
		Name:      name,
		Category:  "backup",
		Size:      size,
		Tier:      "GLACIER",
		Timestamp: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("object.New() error: %v", err)
	}
	return storedObject
}
