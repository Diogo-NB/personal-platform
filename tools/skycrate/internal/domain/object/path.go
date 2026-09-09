package object

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

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

		slug, err := normalizePathSegment(segment)
		if err != nil {
			return "", fmt.Errorf("normalize path segment %q: %w", segment, err)
		}
		normalized = append(normalized, slug)
	}

	return strings.Join(normalized, "/"), nil
}

func normalizePathSegment(value string) (string, error) {
	normalized, err := util.Normalize(value)
	if err != nil {
		return "", err
	}

	var slug strings.Builder
	hasPendingSeparator := false
	lastWasPeriod := false
	for _, character := range normalized {
		character = foldPortugueseCharacter(character)
		isLetter := character >= 'a' && character <= 'z'
		isDigit := character >= '0' && character <= '9'
		switch {
		case isLetter || isDigit:
			if hasPendingSeparator && slug.Len() > 0 && !lastWasPeriod {
				slug.WriteByte('-')
			}
			slug.WriteRune(character)
			hasPendingSeparator = false
			lastWasPeriod = false
		case character == '.':
			slug.WriteByte('.')
			hasPendingSeparator = false
			lastWasPeriod = true
		case unicode.Is(unicode.Mn, character):
			continue
		default:
			hasPendingSeparator = true
		}
	}

	result := slug.String()
	if result == "" {
		return "", errors.New("value must contain a letter, number, or period")
	}
	if isPeriodOnly(result) {
		return "", errors.New("value must not contain only periods")
	}

	return result, nil
}

func foldPortugueseCharacter(character rune) rune {
	switch character {
	case 'á', 'à', 'â', 'ã', 'ä':
		return 'a'
	case 'é', 'è', 'ê', 'ë':
		return 'e'
	case 'í', 'ì', 'î', 'ï':
		return 'i'
	case 'ó', 'ò', 'ô', 'õ', 'ö':
		return 'o'
	case 'ú', 'ù', 'û', 'ü':
		return 'u'
	case 'ç':
		return 'c'
	default:
		return character
	}
}

func isPeriodOnly(value string) bool {
	for _, character := range value {
		if character != '.' {
			return false
		}
	}

	return true
}
