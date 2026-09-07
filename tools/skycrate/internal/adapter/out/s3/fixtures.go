package s3

import (
	"fmt"
	"time"

	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/domain/object"
)

func mockedObjects() ([]object.Object, error) {
	params := []object.RehydrateParams{
		{
			Path:      "documents/report.pdf",
			Name:      "report.pdf",
			Category:  "documents",
			Size:      1_000_000_000,
			Tier:      "STANDARD",
			CreatedAt: time.Date(2026, time.January, 1, 9, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, time.January, 2, 9, 0, 0, 0, time.UTC),
		},
		{
			Path:      "photos/photo.jpg",
			Name:      "photo.jpg",
			Category:  "photos",
			Size:      2_000_000_000,
			Tier:      "STANDARD_IA",
			CreatedAt: time.Date(2026, time.February, 1, 10, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, time.February, 2, 10, 0, 0, 0, time.UTC),
		},
		{
			Path:      "backups/database.dump",
			Name:      "database.dump",
			Category:  "backups",
			Size:      7_000_000_000,
			Tier:      "DEEP_ARCHIVE",
			CreatedAt: time.Date(2026, time.March, 1, 11, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, time.March, 2, 11, 0, 0, 0, time.UTC),
		},
	}

	objects := make([]object.Object, 0, len(params))
	for _, values := range params {
		storedObject, err := object.Rehydrate(values)
		if err != nil {
			return nil, fmt.Errorf("rehydrate mocked object %q: %w", values.Path, err)
		}
		objects = append(objects, storedObject)
	}

	return objects, nil
}
