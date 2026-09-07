package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newLiftCommand(runtime *commandRuntime) *cobra.Command {
	return &cobra.Command{
		Use:   "lift <file-path> [object-category]",
		Short: "Store local file metadata by object category",
		Long: `Validate a regular local file and store its metadata through the mocked S3
repository. Categories are slash-delimited paths. The most specific configured
category mapping selects the storage tier; otherwise an ancestor mapping is used.

When object-category is omitted, choose a configured category and optionally add
a descendant suffix interactively. File contents are not uploaded.`,
		Example: `  skycrate lift ./report.pdf documents
  skycrate lift ./thesis.pdf backups/university
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

			storedObject, err := runtime.dependencies.Lifter.Lift(
				cmd.Context(),
				args[0],
				categoryPath,
			)
			if err != nil {
				return fmt.Errorf("lift: %w", err)
			}

			if _, err := fmt.Fprintf(
				cmd.OutOrStdout(),
				"Lifted metadata (mock): s3://%s/%s\n",
				runtime.dependencies.Bucket,
				storedObject.Path,
			); err != nil {
				return fmt.Errorf("write lift output: %w", err)
			}

			return nil
		},
	}
}
