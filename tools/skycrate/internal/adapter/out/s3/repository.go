package s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/domain/object"
	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/port/out"
)

var _ out.ObjectRepository = (*Repository)(nil)

// Repository is a mock: Save does not persist objects and FindMany returns fixed fixtures.
type Repository struct {
	bucket string
	output io.Writer
}

func New(bucket string, output io.Writer) (*Repository, error) {
	if strings.TrimSpace(bucket) == "" {
		return nil, errors.New("s3 repository bucket must not be empty")
	}
	if strings.TrimSpace(bucket) != bucket {
		return nil, errors.New("s3 repository bucket must not contain surrounding whitespace")
	}
	if output == nil {
		return nil, errors.New("s3 repository output must not be nil")
	}

	return &Repository{bucket: bucket, output: output}, nil
}

func (r *Repository) Save(ctx context.Context, storedObject object.Object) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("save object to bucket %q: %w", r.bucket, err)
	}
	if err := storedObject.Validate(); err != nil {
		return fmt.Errorf("save object to bucket %q: %w", r.bucket, err)
	}
	if _, err := fmt.Fprintf(r.output, "Mock S3 object: %+v\n", storedObject); err != nil {
		return fmt.Errorf("print mock s3 object: %w", err)
	}

	return nil
}

func (r *Repository) FindMany(
	ctx context.Context,
	request out.FindManyRequest,
) ([]object.Object, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("find objects in bucket %q: %w", r.bucket, err)
	}

	categoryFilter, err := normalizeOptionalCategory(request.Category)
	if err != nil {
		return nil, fmt.Errorf("filter objects by category: %w", err)
	}
	tierFilter, err := normalizeOptional(request.Tier)
	if err != nil {
		return nil, fmt.Errorf("filter objects by tier: %w", err)
	}

	fixtures, err := mockedObjects()
	if err != nil {
		return nil, err
	}

	objects := make([]object.Object, 0, len(fixtures))
	for _, storedObject := range fixtures {
		matchesCategory, err := categoryMatches(storedObject.Category, categoryFilter)
		if err != nil {
			return nil, fmt.Errorf("compare object category: %w", err)
		}
		matchesTier, err := tierMatches(storedObject.Tier, tierFilter)
		if err != nil {
			return nil, fmt.Errorf("compare object tier: %w", err)
		}
		if matchesCategory && matchesTier {
			objects = append(objects, storedObject)
		}
	}

	return objects, nil
}
