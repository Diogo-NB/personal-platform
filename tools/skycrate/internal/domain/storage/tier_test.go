package storage

import "testing"

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  Tier
	}{
		{input: "archive", want: TierArchive},
		{input: "COLD", want: TierCold},
		{input: "Instant", want: TierInstant},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			t.Parallel()
			got, err := Parse(test.input)
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", test.input, err)
			}
			if got != test.want {
				t.Errorf("Parse(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

func TestParseRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	for _, input := range []string{"", " archive", "cold ", "warm", "GLACIER"} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			if _, err := Parse(input); err == nil {
				t.Fatalf("Parse(%q) error = nil, want validation error", input)
			}
		})
	}
}
