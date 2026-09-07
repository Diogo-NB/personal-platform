package util

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

func Slug(value string) (string, error) {
	normalized, err := Normalize(value)
	if err != nil {
		return "", err
	}

	var slug strings.Builder
	lastWasSpace := false
	for _, character := range normalized {
		if unicode.IsSpace(character) {
			if !lastWasSpace {
				slug.WriteByte('-')
			}
			lastWasSpace = true
			continue
		}

		lastWasSpace = false
		if isSlugCharacter(character) {
			slug.WriteRune(character)
			continue
		}

		return "", fmt.Errorf("value contains unsupported character %q", character)
	}

	result := slug.String()
	if isPeriodOnly(result) {
		return "", errors.New("value must not contain only periods")
	}

	return result, nil
}

func isSlugCharacter(character rune) bool {
	isLowercaseLetter := character >= 'a' && character <= 'z'
	isDigit := character >= '0' && character <= '9'
	isSpecial := character == '.' || character == '_' || character == '-'
	return isLowercaseLetter || isDigit || isSpecial
}

func isPeriodOnly(value string) bool {
	if value == "" {
		return true
	}

	for _, character := range value {
		if character != '.' {
			return false
		}
	}

	return true
}
