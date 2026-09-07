package object

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/util"
)

func normalizeRelativePath(value string) (string, error) {
	if strings.HasPrefix(value, "/") || strings.HasSuffix(value, "/") {
		return "", errors.New("path must not start or end with a slash")
	}

	segments := strings.Split(value, "/")
	normalized := make([]string, 0, len(segments))
	for _, segment := range segments {
		if segment == "" {
			return "", errors.New("path must not contain empty segments")
		}

		slug, err := util.Slug(segment)
		if err != nil {
			return "", fmt.Errorf("normalize path segment %q: %w", segment, err)
		}
		normalized = append(normalized, slug)
	}

	return strings.Join(normalized, "/"), nil
}
