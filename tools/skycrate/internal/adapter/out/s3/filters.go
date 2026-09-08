package s3

import (
	"strings"

	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/domain/category"
)

func normalizeOptionalCategory(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	return category.NormalizePath(value)
}

func categoryMatches(value, filter string) bool {
	if filter == "" {
		return true
	}

	return value == filter || strings.HasPrefix(value, filter+"/")
}
