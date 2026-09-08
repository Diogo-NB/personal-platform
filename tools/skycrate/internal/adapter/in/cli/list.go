package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/port/in"
	"github.com/spf13/cobra"
)

const bytesPerGigabyte = 1_000_000_000

func newListCommand(runtime *commandRuntime) *cobra.Command {
	var categoryPath string
	var tier string

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "Summarize stored S3 object count and size",
		Long: `Summarize Skycrate-managed objects in Amazon S3. Results can be filtered
by category subtree and semantic storage tier. Multiple filters use AND
semantics. Object count and combined size are reported in decimal gigabytes.`,
		Example: `  skycrate list
	  skycrate list --category backup
	  skycrate list --tier archive
	  skycrate list --category backup --tier archive`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if cmd.Flags().Changed("category") && strings.TrimSpace(categoryPath) == "" {
				return errors.New("category filter must not be empty")
			}
			if cmd.Flags().Changed("tier") && strings.TrimSpace(tier) == "" {
				return errors.New("tier filter must not be empty")
			}

			summary, err := runtime.dependencies.Lister.List(cmd.Context(), in.ListRequest{
				Category: categoryPath,
				Tier:     tier,
			})
			if err != nil {
				return fmt.Errorf("list: %w", err)
			}

			if _, err := fmt.Fprintf(
				cmd.OutOrStdout(),
				"Objects: %d\nTotal size: %.3f GB\n",
				summary.ObjectCount,
				float64(summary.TotalBytes)/bytesPerGigabyte,
			); err != nil {
				return fmt.Errorf("write list output: %w", err)
			}

			return nil
		},
	}

	listCmd.Flags().StringVar(&categoryPath, "category", "", "filter by category subtree")
	listCmd.Flags().StringVar(&tier, "tier", "", "filter by semantic storage tier")

	return listCmd
}
