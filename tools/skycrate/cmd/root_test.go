package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewRootCommandHelp(t *testing.T) {
	t.Parallel()

	command := newRootCommand()
	output := new(bytes.Buffer)
	command.SetOut(output)
	command.SetErr(output)
	command.SetArgs([]string{"--help"})
	if command.Short != "Manage files in cloud storage" {
		t.Errorf("Short = %q, want %q", command.Short, "Manage files in cloud storage")
	}

	if err := command.Execute(); err != nil {
		t.Fatalf("execute root help: %v", err)
	}

	help := output.String()
	for _, expected := range []string{
		"Skycrate is a command-line tool for uploading files to named buckets",
		"skycrate lift ./file.extension bucket-name",
		"lift        Preview a file upload to a bucket",
	} {
		if !strings.Contains(help, expected) {
			t.Errorf("help does not contain %q:\n%s", expected, help)
		}
	}
}
