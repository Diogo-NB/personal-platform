package category

import "testing"

func TestNormalizePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		want    string
		wantErr bool
	}{
		{name: "hierarchy", value: " Back Ups / University Files ", want: "back-ups/university-files"},
		{name: "leading slash", value: "/backups", wantErr: true},
		{name: "trailing slash", value: "backups/", wantErr: true},
		{name: "empty segment", value: "backups//university", wantErr: true},
		{name: "relative segment", value: "backups/../university", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := NormalizePath(test.value)
			if (err != nil) != test.wantErr {
				t.Fatalf("NormalizePath(%q) error = %v, wantErr %t", test.value, err, test.wantErr)
			}
			if got != test.want {
				t.Errorf("NormalizePath(%q) = %q, want %q", test.value, got, test.want)
			}
		})
	}
}
