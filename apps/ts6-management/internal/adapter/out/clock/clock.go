package clock

import (
	"time"

	portout "github.com/diogo-nb/personal-platform/apps/ts6-management/internal/port/out"
)

type System struct{}

func (System) Now() time.Time {
	return time.Now()
}

var _ portout.Clock = System{}
