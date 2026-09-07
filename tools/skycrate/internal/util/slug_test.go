package util

import "testing"

func TestSlug(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		want    string
		wantErr bool
	}{
		{name: "filename", value: " My   Report.PDF ", want: "my-report.pdf"},
		{name: "safe punctuation", value: "report_final-2.pdf", want: "report_final-2.pdf"},
		{name: "unsupported unicode", value: "relatório.pdf", wantErr: true},
		{name: "slash", value: "nested/file", wantErr: true},
		{name: "period only", value: "...", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := Slug(test.value)
			if (err != nil) != test.wantErr {
				t.Fatalf("Slug(%q) error = %v, wantErr %t", test.value, err, test.wantErr)
			}
			if got != test.want {
				t.Errorf("Slug(%q) = %q, want %q", test.value, got, test.want)
			}
		})
	}
}
