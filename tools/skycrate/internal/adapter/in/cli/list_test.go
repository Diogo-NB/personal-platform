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
		"every tier's object count",
		"overall totals",
		"decimal gigabytes",
	} {
		if !strings.Contains(output.String(), expected) {
			t.Errorf("help does not contain %q:\n%s", expected, output)
		}
	}
	for _, removedFlag := range []string{"--category", "--tier", "--s3-tier"} {
		if strings.Contains(output.String(), removedFlag) {
			t.Errorf("help contains removed %s flag:\n%s", removedFlag, output)
		}
	}
}

func TestList(t *testing.T) {
	t.Parallel()

	stdout, stderr, err := executeCommand(t, writeCommandConfig(t), nil, "list")
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	want := "TIER     OBJECTS  SIZE (GB)\n" +
		"default        0      0.000\n" +
		"archive        1      7.000\n" +
		"cold           1      2.000\n" +
		"instant        1      1.000\n" +
		"TOTAL          3     10.000\n"
	if stdout != want || stderr != "" {
		t.Errorf("output = (%q, %q), want (%q, empty)", stdout, stderr, want)
	}
}

func TestListRejectsRemovedFilters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		flag string
	}{
		{name: "category", flag: "--category"},
		{name: "tier", flag: "--tier"},
		{name: "legacy S3 tier", flag: "--s3-tier"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			stdout, stderr, err := executeCommand(
				t,
				writeCommandConfig(t),
				nil,
				"list",
				test.flag,
				"archive",
			)
			want := "unknown flag: " + test.flag
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("Execute() error = %v, want %q", err, want)
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
