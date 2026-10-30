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
		for _, name := range []string{"auth", "dokploy", "investflow"} {
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
		template.ResourceCountIs(jsii.String("AWS::Route53::RecordSet"), jsii.Number(4))
	})

	t.Run("outputs", func(t *testing.T) {
		for _, output := range []string{"HostedZoneID", "NameServers"} {
			template.HasOutput(jsii.String(output), map[string]any{
				"Value": assertions.Match_AnyValue(),
			})
		}
	})
}
