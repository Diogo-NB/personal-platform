package cli

import (
	"fmt"
	"strconv"
	"text/tabwriter"

	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/port/in"
	"github.com/spf13/cobra"
)

const bytesPerGigabyte = 1_000_000_000

func newListCommand(runtime *commandRuntime) *cobra.Command {
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "Summarize stored S3 objects by storage tier",
		Long: `Summarize Skycrate-managed objects in Amazon S3 by semantic storage tier.
The table reports every tier's object count and combined size, followed by
overall totals. Sizes are reported in decimal gigabytes.`,
		Example: `  skycrate list`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			summary, err := runtime.dependencies.Lister.List(cmd.Context())
			if err != nil {
				return fmt.Errorf("list: %w", err)
			}

			if err := writeListSummary(cmd, summary); err != nil {
				return fmt.Errorf("write list output: %w", err)
			}

			return nil
		},
	}

	return listCmd
}

func writeListSummary(cmd *cobra.Command, summary in.ListSummary) error {
	objectWidth := max(len("OBJECTS"), len(strconv.Itoa(summary.ObjectCount)))
	sizeWidth := len("SIZE (GB)")
	totalSize := float64(summary.TotalBytes) / bytesPerGigabyte
	if formattedWidth := len(fmt.Sprintf("%.3f", totalSize)); formattedWidth > sizeWidth {
		sizeWidth = formattedWidth
	}

	writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintf(writer, "TIER\t%*s\t%*s\n", objectWidth, "OBJECTS", sizeWidth, "SIZE (GB)"); err != nil {
		return err
	}
	for _, tier := range summary.Tiers {
		sizeGB := float64(tier.TotalBytes) / bytesPerGigabyte
		if _, err := fmt.Fprintf(
			writer,
			"%s\t%*d\t%*.3f\n",
			tier.Tier,
			objectWidth,
			tier.ObjectCount,
			sizeWidth,
			sizeGB,
		); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(
		writer,
		"TOTAL\t%*d\t%*.3f\n",
		objectWidth,
		summary.ObjectCount,
		sizeWidth,
		totalSize,
	); err != nil {
		return err
	}

	return writer.Flush()
}
