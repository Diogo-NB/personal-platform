package category

import "testing"

func TestNewCatalog(t *testing.T) {
	t.Parallel()

	catalog, err := NewCatalog(map[string]string{
		"Photos":                "STANDARD_IA",
		"Back Ups":              "GLACIER",
		"Back Ups / University": "DEEP_ARCHIVE",
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
		categories map[string]string
	}{
		{name: "empty catalog", categories: map[string]string{}},
		{name: "invalid category", categories: map[string]string{"backups//school": "GLACIER"}},
		{name: "empty tier", categories: map[string]string{"backups": " "}},
		{name: "tier whitespace", categories: map[string]string{"backups": " GLACIER "}},
		{
			name: "normalized duplicate",
			categories: map[string]string{
				"Back Ups": "GLACIER",
				"back-ups": "STANDARD",
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

	catalog, err := NewCatalog(map[string]string{
		"backup":            "GLACIER",
		"backup/university": "DEEP_ARCHIVE",
	})
	if err != nil {
		t.Fatalf("NewCatalog() error: %v", err)
	}

	tests := []struct {
		name         string
		category     string
		wantCategory string
		wantTier     string
		wantErr      bool
	}{
		{name: "exact", category: "backup", wantCategory: "backup", wantTier: "GLACIER"},
		{name: "ancestor", category: "BACKUP/Personal/Photos", wantCategory: "backup/personal/photos", wantTier: "GLACIER"},
		{name: "specific", category: "backup/university/thesis", wantCategory: "backup/university/thesis", wantTier: "DEEP_ARCHIVE"},
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
