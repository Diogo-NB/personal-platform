package dns

import (
	"os"
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/jsii-runtime-go"
)

func TestMain(m *testing.M) {
	code := m.Run()
	jsii.Close()
	os.Exit(code)
}

func TestNewStack(t *testing.T) {
	app := awscdk.NewApp(nil)
	template := assertions.Template_FromStack(NewStack(app, "TestStack", nil), nil)

	t.Run("retained hosted zone", func(t *testing.T) {
		template.ResourceCountIs(jsii.String("AWS::Route53::HostedZone"), jsii.Number(1))
		template.HasResource(jsii.String("AWS::Route53::HostedZone"), map[string]any{
			"DeletionPolicy":      "Retain",
			"UpdateReplacePolicy": "Retain",
			"Properties": map[string]any{
				"Name": "diogo-nb.com.br.",
				"HostedZoneTags": []any{
					map[string]any{"Key": "Application", "Value": "dns"},
					map[string]any{"Key": "Environment", "Value": "production"},
					map[string]any{"Key": "Project", "Value": "personal-platform"},
				},
			},
		})
	})

	t.Run("vps address record", func(t *testing.T) {
		template.HasResourceProperties(jsii.String("AWS::Route53::RecordSet"), map[string]any{
			"Name":            "vps.diogo-nb.com.br.",
			"Type":            "A",
			"ResourceRecords": []any{vpsIPv4},
			"TTL":             "300",
		})
	})

	t.Run("service names alias the vps record", func(t *testing.T) {
		for _, name := range []string{"auth", "dokploy", "investflow", "teamspeak"} {
			template.HasResourceProperties(jsii.String("AWS::Route53::RecordSet"), map[string]any{
				"Name": name + ".diogo-nb.com.br.",
				"Type": "A",
				"AliasTarget": map[string]any{
					"DNSName": map[string]any{
						"Ref": assertions.Match_StringLikeRegexp(jsii.String("^VPSRecord")),
					},
					"HostedZoneId": map[string]any{
						"Ref": assertions.Match_StringLikeRegexp(jsii.String("^HostedZone")),
					},
				},
			})
		}
	})

	t.Run("hosted email records", func(t *testing.T) {
		for _, record := range []struct {
			name       string
			recordType string
			values     []any
		}{
			{
				name: "diogo-nb.com.br.", recordType: "MX",
				values: []any{"10 mail.anonaddy.me", "20 mail2.anonaddy.me"},
			},
			{
				name: "diogo-nb.com.br.", recordType: "TXT",
				values: []any{
					`"v=spf1 include:spf.anonaddy.me -all"`,
					`"aa-verify=183b894cd3b8313ce5c8d0e8ee9ffe3f9e887b41"`,
				},
			},
			{
				name: "dk1._domainkey.diogo-nb.com.br.", recordType: "CNAME",
				values: []any{"dk1._domainkey.anonaddy.me"},
			},
			{
				name: "dk2._domainkey.diogo-nb.com.br.", recordType: "CNAME",
				values: []any{"dk2._domainkey.anonaddy.me"},
			},
			{
				name: "_dmarc.diogo-nb.com.br.", recordType: "TXT",
				values: []any{`"v=DMARC1; p=quarantine; adkim=s"`},
			},
		} {
			template.HasResourceProperties(jsii.String("AWS::Route53::RecordSet"), map[string]any{
				"Name":            record.name,
				"Type":            record.recordType,
				"ResourceRecords": record.values,
				"TTL":             "300",
			})
		}
		template.ResourceCountIs(jsii.String("AWS::Route53::RecordSet"), jsii.Number(10))
	})

	t.Run("outputs", func(t *testing.T) {
		for _, output := range []string{"HostedZoneID", "NameServers"} {
			template.HasOutput(jsii.String(output), map[string]any{
				"Value": assertions.Match_AnyValue(),
			})
		}
	})
}
