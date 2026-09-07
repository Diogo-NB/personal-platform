package category

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/util"
)

func NormalizePath(value string) (string, error) {
	normalized, err := util.Normalize(value)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(normalized, "/") || strings.HasSuffix(normalized, "/") {
		return "", errors.New("category must not start or end with a slash")
	}

	segments := strings.Split(normalized, "/")
	result := make([]string, 0, len(segments))
	for _, segment := range segments {
		if segment == "" {
			return "", errors.New("category must not contain empty segments")
		}

		slug, err := util.Slug(segment)
		if err != nil {
			return "", fmt.Errorf("normalize category segment %q: %w", segment, err)
		}
		result = append(result, slug)
	}

	return strings.Join(result, "/"), nil
}
