package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/port/in"
)

const bytesPerMegabyte = 1_000_000

func confirmLift(
	ctx context.Context,
	input io.Reader,
	output io.Writer,
	summary in.LiftSummary,
) (bool, error) {
	if _, err := fmt.Fprintf(output, "Objects: %d\n", summary.ObjectCount); err != nil {
		return false, fmt.Errorf("write upload summary: %w", err)
	}
	for _, object := range summary.Objects {
		if _, err := fmt.Fprintf(
			output,
			"- %s | %.2f MB | %.3f GB\n",
			object.Path,
			float64(object.SizeBytes)/bytesPerMegabyte,
			float64(object.SizeBytes)/bytesPerGigabyte,
		); err != nil {
			return false, fmt.Errorf("write upload summary: %w", err)
		}
	}
	if _, err := fmt.Fprintf(
		output,
		"Total size: %.2f MB | %.3f GB\nStorage tier: %s\n",
		float64(summary.TotalBytes)/bytesPerMegabyte,
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
