package util

import (
	"errors"
	"strings"
)

func Normalize(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return "", errors.New("value must not be empty")
	}

	return normalized, nil
}
