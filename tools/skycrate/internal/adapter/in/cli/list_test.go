package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestListCommandHelp(t *testing.T) {
	t.Parallel()

	command := New(nil)
	output := new(bytes.Buffer)
	command.SetOut(output)
	command.SetErr(output)
	command.SetArgs([]string{"list", "--help"})
	if err := command.Execute(); err != nil {
		t.Fatalf("execute list help: %v", err)
	}
	for _, expected := range []string{
		"category subtree",
		"Multiple filters use AND",
		"decimal gigabytes",
		"--category string",
		"--tier string",
	} {
		if !strings.Contains(output.String(), expected) {
			t.Errorf("help does not contain %q:\n%s", expected, output)
		}
	}
	if strings.Contains(output.String(), "--s3-tier") {
		t.Errorf("help contains removed --s3-tier flag:\n%s", output)
	}
}

func TestList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "all", want: "Objects: 3\nTotal size: 10.000 GB\n"},
		{name: "category", args: []string{"--category", "BACKUPS"}, want: "Objects: 1\nTotal size: 7.000 GB\n"},
		{name: "tier", args: []string{"--tier", "standard_ia"}, want: "Objects: 1\nTotal size: 2.000 GB\n"},
		{name: "combined", args: []string{"--category", "documents", "--tier", "STANDARD"}, want: "Objects: 1\nTotal size: 1.000 GB\n"},
		{name: "no matches", args: []string{"--category", "backups/university"}, want: "Objects: 0\nTotal size: 0.000 GB\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			args := append([]string{"list"}, test.args...)
			stdout, stderr, err := executeCommand(t, writeCommandConfig(t), nil, args...)
			if err != nil {
				t.Fatalf("Execute() error: %v", err)
			}
			if stdout != test.want || stderr != "" {
				t.Errorf("output = (%q, %q), want (%q, empty)", stdout, stderr, test.want)
			}
		})
	}
}

func TestListRejectsInvalidFilters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "blank category", args: []string{"--category", " "}, want: "category filter must not be empty"},
		{name: "blank tier", args: []string{"--tier", " "}, want: "tier filter must not be empty"},
		{name: "unconfigured category", args: []string{"--category", "unknown"}, want: "is not configured"},
		{name: "removed tier flag", args: []string{"--s3-tier", "STANDARD"}, want: "unknown flag: --s3-tier"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			args := append([]string{"list"}, test.args...)
			stdout, stderr, err := executeCommand(t, writeCommandConfig(t), nil, args...)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Execute() error = %v, want %q", err, test.want)
			}
			if stdout != "" || stderr != "" {
				t.Errorf("output = (%q, %q), want empty", stdout, stderr)
			}
		})
	}
}

func TestListRejectsPositionalArguments(t *testing.T) {
	t.Parallel()

	stdout, stderr, err := executeCommand(t, writeCommandConfig(t), nil, "list", "unexpected")
	if err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("Execute() error = %v", err)
	}
	if stdout != "" || stderr != "" {
		t.Errorf("output = (%q, %q), want empty", stdout, stderr)
	}
}
