package cmd

import "github.com/spf13/cobra"

// Execute runs the Skycrate command tree.
func Execute() error {
	return newRootCommand().Execute()
}

func newRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "skycrate",
		Short: "Manage files in cloud storage",
		Long: `Skycrate is a command-line tool for uploading files to named buckets and
listing stored files.

The current lift command previews a local file and its destination bucket
without contacting AWS.`,
		Example:       "  skycrate lift ./file.extension bucket-name",
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	rootCmd.AddCommand(newLiftCommand())

	return rootCmd
}
