package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/port/in"
)

func confirmLift(
	ctx context.Context,
	input io.Reader,
	output io.Writer,
	summary in.LiftSummary,
) (bool, error) {
	if _, err := fmt.Fprintf(
		output,
		"Objects: %d\nTotal size: %d bytes (%.3f GB)\nStorage tier: %s\n",
		summary.ObjectCount,
		summary.TotalBytes,
		float64(summary.TotalBytes)/bytesPerGigabyte,
		summary.Tier,
	); err != nil {
		return false, fmt.Errorf("write upload summary: %w", err)
	}

	reader := bufio.NewReader(input)
	for {
		if err := ctx.Err(); err != nil {
			return false, fmt.Errorf("confirm upload: %w", err)
		}
		if _, err := fmt.Fprint(output, "Continue with upload? [y/N]: "); err != nil {
			return false, fmt.Errorf("write upload confirmation: %w", err)
		}

		line, err := readLine(reader)
		if err != nil {
			return false, fmt.Errorf("read upload confirmation: %w", err)
		}

		switch strings.ToLower(strings.TrimSpace(line)) {
		case "y", "yes":
			return true, nil
		case "", "n", "no":
			return false, nil
		default:
			if _, err := fmt.Fprintln(
				output,
				"Invalid response. Enter y/yes or n/no.",
			); err != nil {
				return false, fmt.Errorf("write upload confirmation validation: %w", err)
			}
		}
	}
}
