package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

func newLiftCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "lift <file-path> <bucket-name>",
		Short: "Preview a file upload to a bucket",
		Long: `Read a local file and preview its contents and destination bucket.
This command does not upload data or contact AWS yet.`,
		Example: "  skycrate lift ./file.extension bucket-name",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLift(cmd.OutOrStdout(), args[0], args[1])
		},
	}
}

func runLift(output io.Writer, filePath, bucketName string) error {
	contents, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read file %q: %w", filePath, err)
	}

	var preview bytes.Buffer
	preview.WriteString("File contents:\n")
	preview.Write(contents)
	if len(contents) > 0 && contents[len(contents)-1] != '\n' {
		preview.WriteByte('\n')
	}
	preview.WriteString("Bucket: ")
	preview.WriteString(bucketName)
	preview.WriteByte('\n')

	if _, err := output.Write(preview.Bytes()); err != nil {
		return fmt.Errorf("write lift output: %w", err)
	}

	return nil
}
