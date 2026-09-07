package object

import (
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	t.Parallel()

	timestamp := time.Date(2026, time.April, 5, 12, 30, 0, 0, time.FixedZone("test", -3*60*60))
	got, err := New(NewParams{
		Name:      " My Report.PDF ",
		Category:  " Back Ups / University ",
		Size:      42,
		Tier:      "GLACIER",
		Timestamp: timestamp,
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if got.Path != "back-ups/university/my-report.pdf" {
		t.Errorf("Path = %q, want normalized path", got.Path)
	}
	if got.Name != "my-report.pdf" || got.Category != "back-ups/university" {
		t.Errorf("normalized identity = %#v", got)
	}
	if got.Size != 42 || got.Tier != "GLACIER" {
		t.Errorf("metadata = (%d, %q)", got.Size, got.Tier)
	}
	if !got.CreatedAt.Equal(timestamp.UTC()) || !got.UpdatedAt.Equal(timestamp.UTC()) {
		t.Errorf("timestamps = (%v, %v), want %v", got.CreatedAt, got.UpdatedAt, timestamp.UTC())
	}
}

func TestRehydratePreservesProviderValues(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	got, err := Rehydrate(RehydrateParams{
		Path:      "Backups/University/My Report.PDF",
		Name:      "My Report.PDF",
		Category:  "Backups/University",
		Size:      12,
		Tier:      "GLACIER",
		CreatedAt: createdAt,
		UpdatedAt: createdAt.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("Rehydrate() error: %v", err)
	}
	if got.Path != "Backups/University/My Report.PDF" || got.Category != "Backups/University" {
		t.Errorf("provider values were normalized: %#v", got)
	}
}
