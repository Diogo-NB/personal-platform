package object

import (
	"fmt"
	"path"
	"time"

	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/domain/category"
)

type Object struct {
	Path      string
	Name      string
	Category  string
	Size      int64
	Tier      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type NewParams struct {
	RelativePath string
	Category     string
	Size         int64
	Tier         string
	Timestamp    time.Time
}

type RehydrateParams struct {
	Path      string
	Name      string
	Category  string
	Size      int64
	Tier      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func New(params NewParams) (Object, error) {
	relativePath, err := normalizeRelativePath(params.RelativePath)
	if err != nil {
		return Object{}, fmt.Errorf("normalize object relative path: %w", err)
	}

	categoryPath, err := category.NormalizePath(params.Category)
	if err != nil {
		return Object{}, fmt.Errorf("normalize object category: %w", err)
	}

	timestamp := params.Timestamp.UTC()
	storedObject := Object{
		Path:      categoryPath + "/" + relativePath,
		Name:      path.Base(relativePath),
		Category:  categoryPath,
		Size:      params.Size,
		Tier:      params.Tier,
		CreatedAt: timestamp,
		UpdatedAt: timestamp,
	}
	if err := storedObject.Validate(); err != nil {
		return Object{}, err
	}

	return storedObject, nil
}

// Rehydrate preserves provider values instead of deriving them from current configuration.
func Rehydrate(params RehydrateParams) (Object, error) {
	storedObject := Object{
		Path:      params.Path,
		Name:      params.Name,
		Category:  params.Category,
		Size:      params.Size,
		Tier:      params.Tier,
		CreatedAt: params.CreatedAt,
		UpdatedAt: params.UpdatedAt,
	}
	if err := storedObject.Validate(); err != nil {
		return Object{}, err
	}

	return storedObject, nil
}
