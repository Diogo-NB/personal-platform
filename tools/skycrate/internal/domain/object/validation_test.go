package object

import (
	"strings"
	"testing"
	"time"
)

func TestValidate(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	valid := Object{
		Path:      "backups/report.pdf",
		Name:      "report.pdf",
		Category:  "backups",
		Size:      1,
		Tier:      "GLACIER",
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}

	tests := []struct {
		name   string
		mutate func(*Object)
	}{
		{name: "empty path", mutate: func(o *Object) { o.Path = "" }},
		{name: "empty name", mutate: func(o *Object) { o.Name = " " }},
		{name: "empty category", mutate: func(o *Object) { o.Category = "" }},
		{name: "empty tier", mutate: func(o *Object) { o.Tier = "" }},
		{name: "negative size", mutate: func(o *Object) { o.Size = -1 }},
		{name: "zero created", mutate: func(o *Object) { o.CreatedAt = time.Time{} }},
		{name: "zero updated", mutate: func(o *Object) { o.UpdatedAt = time.Time{} }},
		{name: "updated before created", mutate: func(o *Object) { o.UpdatedAt = createdAt.Add(-time.Second) }},
		{name: "invalid utf8", mutate: func(o *Object) { o.Path = string([]byte{0xff}) }},
		{name: "oversized path", mutate: func(o *Object) { o.Path = strings.Repeat("a", maximumPathBytes+1) }},
		{name: "empty segment", mutate: func(o *Object) { o.Path = "backups//report.pdf" }},
		{name: "relative path", mutate: func(o *Object) { o.Path = "backups/../report.pdf" }},
		{name: "relative category", mutate: func(o *Object) { o.Category = "backups/." }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			candidate := valid
			test.mutate(&candidate)
			if err := candidate.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want validation error")
			}
		})
	}
	if err := valid.Validate(); err != nil {
		t.Errorf("valid Object.Validate() error: %v", err)
	}
}
