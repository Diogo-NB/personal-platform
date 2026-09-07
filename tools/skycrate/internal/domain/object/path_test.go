package object

import "testing"

func TestNormalizeRelativePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "nested path",
			input:    " Research Notes / My Report.PDF ",
			expected: "research-notes/my-report.pdf",
		},
		{name: "empty", input: "", wantErr: true},
		{name: "leading slash", input: "/report.pdf", wantErr: true},
		{name: "trailing slash", input: "reports/", wantErr: true},
		{name: "empty segment", input: "reports//report.pdf", wantErr: true},
		{name: "relative segment", input: "reports/../report.pdf", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := normalizeRelativePath(test.input)
			if (err != nil) != test.wantErr {
				t.Fatalf("normalizeRelativePath(%q) error = %v, wantErr %t", test.input, err, test.wantErr)
			}
			if got != test.expected {
				t.Errorf("normalizeRelativePath(%q) = %q, want %q", test.input, got, test.expected)
			}
		})
	}
}
