package s3

import (
	"strings"

	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/domain/category"
	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/util"
)

func normalizeOptional(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	return util.Normalize(value)
}

func normalizeOptionalCategory(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	return category.NormalizePath(value)
}

func categoryMatches(value, filter string) (bool, error) {
	if filter == "" {
		return true, nil
	}

	normalized, err := category.NormalizePath(value)
	if err != nil {
		return false, err
	}

	return normalized == filter || strings.HasPrefix(normalized, filter+"/"), nil
}

func tierMatches(value, filter string) (bool, error) {
	if filter == "" {
		return true, nil
	}

	normalized, err := util.Normalize(value)
	if err != nil {
		return false, err
	}

	return normalized == filter, nil
}
