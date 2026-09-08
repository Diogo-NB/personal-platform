package object

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

const maximumPathBytes = 1024

func (o Object) Validate() error {
	for field, value := range map[string]string{
		"path":     o.Path,
		"name":     o.Name,
		"category": o.Category,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("object %s must not be empty", field)
		}
	}
	if err := o.Tier.Validate(); err != nil {
		return fmt.Errorf("object: %w", err)
	}

	if o.Size < 0 {
		return errors.New("object size must not be negative")
	}
	if o.CreatedAt.IsZero() {
		return errors.New("object created at must not be zero")
	}
	if o.UpdatedAt.IsZero() {
		return errors.New("object updated at must not be zero")
	}
	if o.UpdatedAt.Before(o.CreatedAt) {
		return errors.New("object updated at must not precede created at")
	}
	if !utf8.ValidString(o.Path) {
		return errors.New("object path must be valid utf-8")
	}
	if len(o.Path) > maximumPathBytes {
		return fmt.Errorf("object path must not exceed %d bytes", maximumPathBytes)
	}
	if err := validateSegments(o.Path); err != nil {
		return fmt.Errorf("object path: %w", err)
	}
	if err := validateSegments(o.Category); err != nil {
		return fmt.Errorf("object category: %w", err)
	}

	return nil
}

func validateSegments(value string) error {
	for segment := range strings.SplitSeq(value, "/") {
		if segment == "" {
			return errors.New("must not contain empty segments")
		}
		if segment == "." || segment == ".." {
			return errors.New("must not contain relative segments")
		}
	}

	return nil
}
