package cli

import (
	"context"
	"fmt"

	"github.com/Diogo-NB/personal-platform/tools/skycrate/internal/port/in"
	"github.com/spf13/cobra"
)

func newLiftCommand(runtime *commandRuntime) *cobra.Command {
	var yes bool

	liftCmd := &cobra.Command{
		Use:   "lift <source-path> [object-category]",
		Short: "Store a local file or directory in Amazon S3 by object category",
		Long: `Validate a local file or directory and upload it to Amazon S3. Directories
are always traversed recursively; no recursive flag is required. Categories are
slash-delimited paths. The most specific configured category mapping selects the
semantic storage tier; otherwise an ancestor mapping is used.

When object-category is omitted, choose a configured category and optionally add
a descendant suffix interactively. Before uploading, review the object count,
combined size, and storage tier. Use --yes to skip this confirmation.`,
		Example: `  skycrate lift ./report.pdf documents
  skycrate lift ./recordings recordings
  skycrate lift ./thesis.pdf backup/university
  skycrate lift --yes ./photo.jpg documents
  skycrate lift ./photo.jpg`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			categoryPath := ""
			if len(args) == 2 {
				categoryPath = args[1]
			} else {
				selected, err := selectCategory(
					cmd.Context(),
					cmd.InOrStdin(),
					cmd.ErrOrStderr(),
					runtime.dependencies.Catalog,
				)
				if err != nil {
					return err
				}
				categoryPath = selected
			}

			approve := func(ctx context.Context, summary in.LiftSummary) (bool, error) {
				if yes {
					return true, nil
				}

				return confirmLift(
					ctx,
					cmd.InOrStdin(),
					cmd.ErrOrStderr(),
					summary,
				)
			}

			result, err := runtime.dependencies.Lifter.Lift(cmd.Context(), in.LiftRequest{
				SourcePath: args[0],
				Category:   categoryPath,
				Approve:    approve,
			})

			if err != nil {
				return fmt.Errorf("lift: %w", err)
			}
			if result.IsCanceled {
				if _, err := fmt.Fprintln(cmd.ErrOrStderr(), "Upload canceled."); err != nil {
					return fmt.Errorf("write lift cancellation: %w", err)
				}
				return nil
			}

			for _, storedObject := range result.Objects {
				if _, err := fmt.Fprintf(
					cmd.OutOrStdout(),
					"Lifted: s3://%s/%s\n",
					runtime.dependencies.Bucket,
					storedObject.Path,
				); err != nil {
					return fmt.Errorf("write lift output: %w", err)
				}
			}

			return nil
		},
	}

	liftCmd.Flags().BoolVarP(&yes, "yes", "y", false, "confirm the upload without prompting")

	return liftCmd
}
