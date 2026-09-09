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
		{
			name:     "portuguese accents and punctuation",
			input:    " Relatórios / Apresentação_Final (Versão #2).PDF ",
			expected: "relatorios/apresentacao-final-versao-2.pdf",
		},
		{
			name:     "portuguese accent variants",
			input:    "ÁÀÂÃÄ ÉÈÊË ÍÌÎÏ ÓÒÔÕÖ ÚÙÛÜ Ç.txt",
			expected: "aaaaa-eeee-iiii-ooooo-uuuu-c.txt",
		},
		{
			name:     "decomposed accents",
			input:    "Relato\u0301rio_C\u0327.PDF",
			expected: "relatorio-c.pdf",
		},
		{
			name:     "separator runs",
			input:    "--My__  Report@@.PDF--",
			expected: "my-report.pdf",
		},
		{name: "hidden file", input: ".ENV", expected: ".env"},
		{name: "empty", input: "", wantErr: true},
		{name: "unsupported characters only", input: "###", wantErr: true},
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
