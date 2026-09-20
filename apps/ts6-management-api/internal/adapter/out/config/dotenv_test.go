package config

import (
	"os"
	"strings"
	"testing"
)

func TestParseDotEnv(t *testing.T) {
	t.Parallel()

	input := strings.NewReader(`
# Local TeamSpeak configuration
CLUSTER_ARN=cluster
SERVICE_NAME="team\nspeak"
AWS_PROFILE='personal profile'
export AWS_REGION=sa-east-1
`)

	got, err := parseDotEnv(input)
	if err != nil {
		t.Fatalf("parseDotEnv() error = %v", err)
	}
	want := map[string]string{
		"AWS_PROFILE":  "personal profile",
		"AWS_REGION":   "sa-east-1",
		"CLUSTER_ARN":  "cluster",
		"SERVICE_NAME": "team\nspeak",
	}
	if len(got) != len(want) {
		t.Fatalf("parseDotEnv() length = %d, want %d", len(got), len(want))
	}
	for key, wantValue := range want {
		if got[key] != wantValue {
			t.Errorf("parseDotEnv()[%q] = %q, want %q", key, got[key], wantValue)
		}
	}
}

func TestParseDotEnvRejectsMalformedEntries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{name: "missing separator", input: "CLUSTER_ARN"},
		{name: "invalid key", input: "CLUSTER-ARN=value"},
		{name: "unterminated quote", input: `CLUSTER_ARN="value`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if _, err := parseDotEnv(strings.NewReader(test.input)); err == nil {
				t.Fatal("parseDotEnv() error = nil, want an error")
			}
		})
	}
}

func TestLoadDotEnvPreservesExistingEnvironment(t *testing.T) {
	t.Parallel()

	path := t.TempDir() + "/.env"
	if err := os.WriteFile(path, []byte("CLUSTER_ARN=from-file\nSERVICE_NAME=service\n"), 0o600); err != nil {
		t.Fatalf("write dotenv: %v", err)
	}

	environment := map[string]string{"CLUSTER_ARN": "from-process"}
	err := loadDotEnv(
		path,
		func(key string) (string, bool) {
			value, exists := environment[key]
			return value, exists
		},
		func(key, value string) error {
			environment[key] = value
			return nil
		},
	)
	if err != nil {
		t.Fatalf("loadDotEnv() error = %v", err)
	}
	if environment["CLUSTER_ARN"] != "from-process" {
		t.Errorf("CLUSTER_ARN = %q, want process value", environment["CLUSTER_ARN"])
	}
	if environment["SERVICE_NAME"] != "service" {
		t.Errorf("SERVICE_NAME = %q, want file value", environment["SERVICE_NAME"])
	}
}

func TestLoadDotEnvAllowsMissingFile(t *testing.T) {
	t.Parallel()

	err := loadDotEnv(
		t.TempDir()+"/.env",
		func(string) (string, bool) { return "", false },
		func(string, string) error { return nil },
	)
	if err != nil {
		t.Fatalf("loadDotEnv() error = %v", err)
	}
}
