package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestNewLiftCommandHelp(t *testing.T) {
	t.Parallel()

	command := newRootCommand()
	output := new(bytes.Buffer)
	command.SetOut(output)
	command.SetErr(output)
	command.SetArgs([]string{"lift", "--help"})

	if err := command.Execute(); err != nil {
		t.Fatalf("execute lift help: %v", err)
	}

	help := output.String()
	for _, expected := range []string{
		"skycrate lift <file-path> <bucket-name>",
		"skycrate lift ./file.extension bucket-name",
		"This command does not upload data or contact AWS yet.",
	} {
		if !strings.Contains(help, expected) {
			t.Errorf("help does not contain %q:\n%s", expected, help)
		}
	}
}

func TestRunLift(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		contents string
		want     string
	}{
		{
			name:     "without trailing newline",
			contents: "hello, skycrate",
			want:     "File contents:\nhello, skycrate\nBucket: archive\n",
		},
		{
			name:     "with trailing newline",
			contents: "first line\nsecond line\n",
			want:     "File contents:\nfirst line\nsecond line\nBucket: archive\n",
		},
		{
			name: "empty file",
			want: "File contents:\nBucket: archive\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			filePath := filepath.Join(t.TempDir(), "input.txt")
			if err := os.WriteFile(filePath, []byte(test.contents), 0o600); err != nil {
				t.Fatalf("write fixture: %v", err)
			}

			output := new(bytes.Buffer)
			if err := runLift(output, filePath, "archive"); err != nil {
				t.Fatalf("runLift() error: %v", err)
			}
			if got := output.String(); got != test.want {
				t.Errorf("runLift() output = %q, want %q", got, test.want)
			}
		})
	}
}

func TestRunLiftMissingFile(t *testing.T) {
	t.Parallel()

	filePath := filepath.Join(t.TempDir(), "missing.txt")
	output := new(bytes.Buffer)

	err := runLift(output, filePath, "archive")
	if err == nil {
		t.Fatal("runLift() error = nil, want an error")
	}
	if !strings.Contains(err.Error(), "read file "+strconv.Quote(filePath)) {
		t.Errorf("runLift() error = %q, want file context", err)
	}
	if got := output.String(); got != "" {
		t.Errorf("runLift() output = %q, want no partial output", got)
	}
}

func TestLiftArgumentCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "no arguments"},
		{name: "file only", args: []string{"input.txt"}},
		{name: "too many", args: []string{"input.txt", "archive", "extra"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			command := newRootCommand()
			output := new(bytes.Buffer)
			command.SetOut(output)
			command.SetErr(output)
			command.SetArgs(append([]string{"lift"}, test.args...))

			err := command.Execute()
			if err == nil {
				t.Fatal("Execute() error = nil, want an argument count error")
			}
			if !strings.Contains(err.Error(), "accepts 2 arg(s)") {
				t.Errorf("Execute() error = %q, want exact argument count error", err)
			}
			if got := output.String(); got != "" {
				t.Errorf("Execute() output = %q, want no output", got)
			}
		})
	}
}

func TestLiftArgumentOrdering(t *testing.T) {
	t.Parallel()

	filePath := filepath.Join(t.TempDir(), "ordered.txt")
	if err := os.WriteFile(filePath, []byte("ordered"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	command := newRootCommand()
	output := new(bytes.Buffer)
	command.SetOut(output)
	command.SetErr(output)
	command.SetArgs([]string{"lift", filePath, "destination-bucket"})

	if err := command.Execute(); err != nil {
		t.Fatalf("execute lift: %v", err)
	}
	const want = "File contents:\nordered\nBucket: destination-bucket\n"
	if got := output.String(); got != want {
		t.Errorf("Execute() output = %q, want %q", got, want)
	}
}
