package util

import "testing"

func TestNormalize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		want    string
		wantErr bool
	}{
		{name: "trim and lowercase", value: "  DEEP_ARCHIVE  ", want: "deep_archive"},
		{name: "blank", value: " \t\n", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := Normalize(test.value)
			if (err != nil) != test.wantErr {
				t.Fatalf("Normalize(%q) error = %v, wantErr %t", test.value, err, test.wantErr)
			}
			if got != test.want {
				t.Errorf("Normalize(%q) = %q, want %q", test.value, got, test.want)
			}
		})
	}
}
