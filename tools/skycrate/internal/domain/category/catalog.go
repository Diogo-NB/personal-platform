package category

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

type Resolution struct {
	Category string
	Tier     string
}

type Catalog struct {
	tiers      map[string]string
	categories []string
}

func NewCatalog(categories map[string]string) (*Catalog, error) {
	if len(categories) == 0 {
		return nil, errors.New("category catalog must not be empty")
	}

	tiers := make(map[string]string, len(categories))
	names := make([]string, 0, len(categories))
	for category, tier := range categories {
		normalized, err := NormalizePath(category)
		if err != nil {
			return nil, fmt.Errorf("normalize category %q: %w", category, err)
		}
		if _, exists := tiers[normalized]; exists {
			return nil, fmt.Errorf("category %q is duplicated after normalization", normalized)
		}
		if strings.TrimSpace(tier) == "" {
			return nil, fmt.Errorf("category %q tier must not be empty", normalized)
		}
		if strings.TrimSpace(tier) != tier {
			return nil, fmt.Errorf("category %q tier must not contain surrounding whitespace", normalized)
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

// Resolve uses the most specific configured ancestor while preserving the full category path.
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

	return Resolution{}, fmt.Errorf("category %q is not configured", normalized)
}
