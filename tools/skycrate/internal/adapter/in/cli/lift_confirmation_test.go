package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/domain/storage"
	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/port/in"
)

func TestConfirmLift(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		input            string
		expectedApproval bool
		expectsRetry     bool
	}{
		{name: "short approval", input: "y\n", expectedApproval: true},
		{name: "case insensitive approval", input: "YES\n", expectedApproval: true},
		{name: "approval without newline", input: "yes", expectedApproval: true},
		{name: "short rejection", input: "n\n", expectedApproval: false},
		{name: "case insensitive rejection", input: "NO\n", expectedApproval: false},
		{name: "empty rejection", input: "\n", expectedApproval: false},
		{
			name:             "invalid response retries",
			input:            "perhaps\nyes\n",
			expectedApproval: true,
			expectsRetry:     true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			output := new(bytes.Buffer)
			approved, err := confirmLift(
				t.Context(),
				strings.NewReader(test.input),
				output,
				in.LiftSummary{
					ObjectCount: 2,
					TotalBytes:  1_500_000_000,
					Tier:        storage.TierCold,
				},
			)
			if err != nil {
				t.Fatalf("confirmLift() error: %v", err)
			}
			if approved != test.expectedApproval {
				t.Errorf("confirmLift() = %t, want %t", approved, test.expectedApproval)
			}

			text := output.String()
			for _, expected := range []string{
				"Objects: 2\n",
				"Total size: 1500000000 bytes (1.500 GB)\n",
				"Storage tier: cold\n",
				"Continue with upload? [y/N]: ",
			} {
				if !strings.Contains(text, expected) {
					t.Errorf("output does not contain %q:\n%s", expected, text)
				}
			}
			if got := strings.Contains(text, "Invalid response."); got != test.expectsRetry {
				t.Errorf("retry message present = %t, want %t", got, test.expectsRetry)
			}
		})
	}
}
