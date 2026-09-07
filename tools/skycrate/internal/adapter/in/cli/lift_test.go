package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLiftCommandHelp(t *testing.T) {
	t.Parallel()

	command := New(nil)
	output := new(bytes.Buffer)
	command.SetOut(output)
	command.SetErr(output)
	command.SetArgs([]string{"lift", "--help"})
	if err := command.Execute(); err != nil {
		t.Fatalf("execute lift help: %v", err)
	}
	for _, expected := range []string{
		"skycrate lift <source-path> [object-category]",
		"Directories are always traversed recursively",
		"no recursive flag is",
		"most specific configured",
		"ancestor mapping",
		"descendant suffix interactively",
		"File contents are not uploaded",
	} {
		if !strings.Contains(output.String(), expected) {
			t.Errorf("help does not contain %q:\n%s", expected, output)
		}
	}
}

func TestLiftDirectoryRecursively(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "Root File.TXT"), []byte("root"), 0o600); err != nil {
		t.Fatalf("write root fixture: %v", err)
	}
	nestedDirectory := filepath.Join(directory, "Research Notes")
	if err := os.Mkdir(nestedDirectory, 0o700); err != nil {
		t.Fatalf("create nested directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nestedDirectory, "Draft.MD"), []byte("draft"), 0o600); err != nil {
		t.Fatalf("write nested fixture: %v", err)
	}

	stdout, stderr, err := executeCommand(
		t,
		writeCommandConfig(t),
		nil,
		"lift",
		directory,
		"documents",
	)
	if err != nil {
		t.Fatalf("execute lift: %v", err)
	}
	const want = "Lifted metadata (mock): s3://test-bucket/documents/research-notes/draft.md\n" +
		"Lifted metadata (mock): s3://test-bucket/documents/root-file.txt\n"
	if stdout != want || stderr != "" {
		t.Errorf("output = (%q, %q), want (%q, empty)", stdout, stderr, want)
	}
}

func TestLiftWithExplicitCategory(t *testing.T) {
	t.Parallel()

	filePath := filepath.Join(t.TempDir(), "My Report.PDF")
	if err := os.WriteFile(filePath, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	stdout, stderr, err := executeCommand(
		t,
		writeCommandConfig(t),
		nil,
		"lift",
		filePath,
		"BACKUPS/University/Thesis",
	)
	if err != nil {
		t.Fatalf("execute lift: %v", err)
	}
	const want = "Lifted metadata (mock): s3://test-bucket/backups/university/thesis/my-report.pdf\n"
	if stdout != want || stderr != "" {
		t.Errorf("output = (%q, %q), want (%q, empty)", stdout, stderr, want)
	}
}

func TestLiftInteractiveCategory(t *testing.T) {
	t.Parallel()

	filePath := filepath.Join(t.TempDir(), "Thesis.PDF")
	if err := os.WriteFile(filePath, []byte("data"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	stdout, stderr, err := executeCommand(
		t,
		writeCommandConfig(t),
		strings.NewReader("9\n1\nUniversity / Computer Science\n"),
		"lift",
		filePath,
	)
	if err != nil {
		t.Fatalf("execute interactive lift: %v", err)
	}
	const want = "Lifted metadata (mock): s3://test-bucket/backups/university/computer-science/thesis.pdf\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
	for _, expected := range []string{
		"Select a category:",
		"1. backups",
		"2. backups/university",
		"Invalid selection.",
		"Subcategory (optional):",
	} {
		if !strings.Contains(stderr, expected) {
			t.Errorf("stderr does not contain %q:\n%s", expected, stderr)
		}
	}
}

func TestLiftInteractiveCategoryRetriesInvalidSuffix(t *testing.T) {
	t.Parallel()

	filePath := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(filePath, []byte("data"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	stdout, stderr, err := executeCommand(
		t,
		writeCommandConfig(t),
		strings.NewReader("1\nbad//suffix\nuniversity\n"),
		"lift",
		filePath,
	)
	if err != nil {
		t.Fatalf("execute interactive lift: %v", err)
	}
	if stdout != "Lifted metadata (mock): s3://test-bucket/backups/university/file.txt\n" {
		t.Errorf("stdout = %q", stdout)
	}
	if !strings.Contains(stderr, "Invalid subcategory:") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestLiftInteractiveCategoryEOF(t *testing.T) {
	t.Parallel()

	filePath := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(filePath, []byte("data"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	stdout, stderr, err := executeCommand(
		t,
		writeCommandConfig(t),
		strings.NewReader(""),
		"lift",
		filePath,
	)
	if err == nil || !strings.Contains(err.Error(), "read category selection") {
		t.Fatalf("Execute() error = %v", err)
	}
	if stdout != "" || !strings.Contains(stderr, "Selection:") {
		t.Errorf("output = (%q, %q)", stdout, stderr)
	}
}

func TestLiftArgumentCount(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{{"lift"}, {"lift", "a", "b", "c"}} {
		stdout, stderr, err := executeCommand(t, writeCommandConfig(t), nil, args...)
		if err == nil || !strings.Contains(err.Error(), "arg(s)") {
			t.Fatalf("Execute(%v) error = %v", args, err)
		}
		if stdout != "" || stderr != "" {
			t.Errorf("output = (%q, %q), want empty", stdout, stderr)
		}
	}
}
