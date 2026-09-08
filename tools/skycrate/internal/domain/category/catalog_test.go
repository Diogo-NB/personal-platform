package category

import (
	"testing"

	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/domain/storage"
)

func TestNewCatalog(t *testing.T) {
	t.Parallel()

	catalog, err := NewCatalog(map[string]storage.Tier{
		"Photos":                storage.TierInstant,
		"Back Ups":              storage.TierCold,
		"Back Ups / University": storage.TierArchive,
	})
	if err != nil {
		t.Fatalf("NewCatalog() error: %v", err)
	}

	want := []string{"back-ups", "back-ups/university", "photos"}
	got := catalog.Categories()
	for index := range want {
		if got[index] != want[index] {
			t.Errorf("Categories()[%d] = %q, want %q", index, got[index], want[index])
		}
	}

	got[0] = "mutated"
	if catalog.Categories()[0] != "back-ups" {
		t.Error("Categories() exposed catalog state")
	}
}

func TestNewCatalogRejectsInvalidMappings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		categories map[string]storage.Tier
	}{
		{name: "empty catalog", categories: map[string]storage.Tier{}},
		{name: "invalid category", categories: map[string]storage.Tier{"backups//school": storage.TierCold}},
		{name: "empty tier", categories: map[string]storage.Tier{"backups": storage.TierUnknown}},
		{name: "unsupported tier", categories: map[string]storage.Tier{"backups": storage.Tier("warm")}},
		{
			name: "normalized duplicate",
			categories: map[string]storage.Tier{
				"Back Ups": storage.TierCold,
				"back-ups": storage.TierInstant,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := NewCatalog(test.categories); err == nil {
				t.Fatal("NewCatalog() error = nil, want validation error")
			}
		})
	}
}

func TestCatalogResolve(t *testing.T) {
	t.Parallel()

	catalog, err := NewCatalog(map[string]storage.Tier{
		"backup":            storage.TierCold,
		"backup/university": storage.TierArchive,
	})
	if err != nil {
		t.Fatalf("NewCatalog() error: %v", err)
	}

	tests := []struct {
		name         string
		category     string
		wantCategory string
		wantTier     storage.Tier
		wantErr      bool
	}{
		{name: "exact", category: "backup", wantCategory: "backup", wantTier: storage.TierCold},
		{name: "ancestor", category: "BACKUP/Personal/Photos", wantCategory: "backup/personal/photos", wantTier: storage.TierCold},
		{name: "specific", category: "backup/university/thesis", wantCategory: "backup/university/thesis", wantTier: storage.TierArchive},
		{name: "segment boundary", category: "backup-old", wantErr: true},
		{name: "unconfigured root", category: "documents", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := catalog.Resolve(test.category)
			if (err != nil) != test.wantErr {
				t.Fatalf("Resolve(%q) error = %v, wantErr %t", test.category, err, test.wantErr)
			}
			if got.Category != test.wantCategory || got.Tier != test.wantTier {
				t.Errorf("Resolve(%q) = %#v", test.category, got)
			}
		})
	}
}
