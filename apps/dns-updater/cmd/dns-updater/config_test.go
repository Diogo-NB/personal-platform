package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	values := validEnvironment()
	values["DNS_NAME"] = "TS.DIOGO-NB.COM.BR."

	got, err := loadConfig(environment(values))
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	want := updaterConfig{
		clusterARN:    "arn:aws:ecs:sa-east-1:123456789012:cluster/personal-platform-teamspeak6",
		serviceName:   "teamspeak6",
		hostedZoneID:  "Z1234567890",
		dnsName:       "ts.diogo-nb.com.br",
		dnsTTLSeconds: 60,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("loadConfig() = %#v, want %#v", got, want)
	}
}

func TestLoadConfigRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		variable   string
		value      string
		errorMatch string
	}{
		{name: "missing cluster arn", variable: "CLUSTER_ARN", value: "", errorMatch: "CLUSTER_ARN is required"},
		{name: "invalid cluster arn", variable: "CLUSTER_ARN", value: "cluster/name", errorMatch: "CLUSTER_ARN must be"},
		{name: "missing service name", variable: "SERVICE_NAME", value: "", errorMatch: "SERVICE_NAME is required"},
		{name: "service name whitespace", variable: "SERVICE_NAME", value: "team speak", errorMatch: "SERVICE_NAME must not"},
		{name: "missing hosted zone id", variable: "HOSTED_ZONE_ID", value: "", errorMatch: "HOSTED_ZONE_ID is required"},
		{name: "hosted zone id whitespace", variable: "HOSTED_ZONE_ID", value: "zone id", errorMatch: "HOSTED_ZONE_ID must not"},
		{name: "missing dns name", variable: "DNS_NAME", value: "", errorMatch: "DNS_NAME is required"},
		{name: "invalid dns name", variable: "DNS_NAME", value: "-invalid.example", errorMatch: "DNS_NAME must be"},
		{name: "missing dns ttl", variable: "DNS_TTL", value: "", errorMatch: "DNS_TTL is required"},
		{name: "non-numeric dns ttl", variable: "DNS_TTL", value: "minute", errorMatch: "DNS_TTL must be"},
		{name: "zero dns ttl", variable: "DNS_TTL", value: "0", errorMatch: "DNS_TTL must be"},
		{name: "negative dns ttl", variable: "DNS_TTL", value: "-1", errorMatch: "DNS_TTL must be"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			values := validEnvironment()
			values[test.variable] = test.value
			_, err := loadConfig(environment(values))
			if err == nil {
				t.Fatal("loadConfig() error = nil, want error")
			}
			if !strings.Contains(err.Error(), test.errorMatch) {
				t.Errorf("loadConfig() error = %q, want substring %q", err, test.errorMatch)
			}
		})
	}
}

func validEnvironment() map[string]string {
	return map[string]string{
		"CLUSTER_ARN":    "arn:aws:ecs:sa-east-1:123456789012:cluster/personal-platform-teamspeak6",
		"SERVICE_NAME":   "teamspeak6",
		"HOSTED_ZONE_ID": "Z1234567890",
		"DNS_NAME":       "ts.diogo-nb.com.br",
		"DNS_TTL":        "60",
	}
}

func environment(values map[string]string) func(string) string {
	return func(name string) string {
		return values[name]
	}
}
