package s3

import "testing"

func TestCategoryMatches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		category string
		filter   string
		want     bool
	}{
		{name: "exact", category: "backups", filter: "backups", want: true},
		{name: "descendant", category: "backups/university", filter: "backups", want: true},
		{name: "segment boundary", category: "backups-old", filter: "backups", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := categoryMatches(test.category, test.filter)
			if err != nil {
				t.Fatalf("categoryMatches() error: %v", err)
			}
			if got != test.want {
				t.Errorf("categoryMatches(%q, %q) = %t, want %t", test.category, test.filter, got, test.want)
			}
		})
	}
}
