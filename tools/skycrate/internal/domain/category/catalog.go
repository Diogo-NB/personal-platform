package category

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/domain/storage"
)

type Resolution struct {
	Category string
	Tier     storage.Tier
}

type Catalog struct {
	tiers      map[string]storage.Tier
	categories []string
}

func NewCatalog(categories map[string]storage.Tier) (*Catalog, error) {
	if len(categories) == 0 {
		return nil, errors.New("category catalog must not be empty")
	}

	tiers := make(map[string]storage.Tier, len(categories))
	names := make([]string, 0, len(categories))
	for category, tier := range categories {
		normalized, err := NormalizePath(category)
		if err != nil {
			return nil, fmt.Errorf("normalize category %q: %w", category, err)
		}
		if _, exists := tiers[normalized]; exists {
			return nil, fmt.Errorf("category %q is duplicated after normalization", normalized)
		}
		if err := tier.Validate(); err != nil {
			return nil, fmt.Errorf("category %q: %w", normalized, err)
		}

		tiers[normalized] = tier
		names = append(names, normalized)
	}
	sort.Strings(names)

	return &Catalog{
		tiers:      tiers,
		categories: names,
	}, nil
}

func (c *Catalog) Categories() []string {
	return append([]string{}, c.categories...)
}

// Resolve preserves the full category path and uses the most specific configured
// ancestor, falling back to the default tier.
func (c *Catalog) Resolve(value string) (Resolution, error) {
	normalized, err := NormalizePath(value)
	if err != nil {
		return Resolution{}, fmt.Errorf("normalize category: %w", err)
	}

	candidate := normalized
	for {
		if tier, exists := c.tiers[candidate]; exists {
			return Resolution{Category: normalized, Tier: tier}, nil
		}

		separator := strings.LastIndexByte(candidate, '/')
		if separator < 0 {
			break
		}
		candidate = candidate[:separator]
	}

	return Resolution{Category: normalized, Tier: storage.TierDefault}, nil
}
