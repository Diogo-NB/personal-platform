package storage

import (
	"fmt"
	"strings"
)

type Tier string

const (
	TierUnknown Tier = ""
	TierDefault Tier = "default"
	TierArchive Tier = "archive"
	TierCold    Tier = "cold"
	TierInstant Tier = "instant"
)

func Parse(value string) (Tier, error) {
	if strings.TrimSpace(value) != value {
		return TierUnknown, fmt.Errorf("storage tier %q must not contain surrounding whitespace", value)
	}

	tier := Tier(strings.ToLower(value))
	if err := tier.Validate(); err != nil {
		return TierUnknown, err
	}

	return tier, nil
}

func (t Tier) Validate() error {
	switch t {
	case TierDefault, TierArchive, TierCold, TierInstant:
		return nil
	default:
		return fmt.Errorf(
			"storage tier %q is invalid; supported values are default, archive, cold, and instant",
			t,
		)
	}
}

func (t Tier) String() string {
	return string(t)
}
