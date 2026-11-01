// Package dns provisions the personal domain's Route 53 hosted zone and the
// records for VPS services and hosted email.
package dns

import (
	"aws/internal/costtags"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsroute53"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsroute53targets"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

const (
	domainName    = "diogo-nb.com.br"
	vpsRecordName = "vps." + domainName
	vpsIPv4       = "187.41.107.80"
	vpsRecordTTL  = 300
)

// vpsServiceNames are published as aliases of the VPS record, so moving to a
// new server changes only vpsIPv4.
var vpsServiceNames = []string{
	"auth",
	"dokploy",
	"investflow",
	"teamspeak",
}

type StackProps struct {
	awscdk.StackProps
}

// NewStack creates the retained hosted zone and the VPS service records.
func NewStack(scope constructs.Construct, id string, props *StackProps) awscdk.Stack {
	var stackProps awscdk.StackProps
	if props != nil {
		stackProps = props.StackProps
	}
	stackProps.Description = jsii.String("Personal domain DNS")
	stackProps.Tags = costtags.Merge(stackProps.Tags, costtags.ApplicationDNS)

	stack := awscdk.NewStack(scope, jsii.String(id), &stackProps)
	costtags.Apply(stack, costtags.ApplicationDNS)

	hostedZone := awsroute53.NewPublicHostedZone(
		stack,
		jsii.String("HostedZone"),
		&awsroute53.PublicHostedZoneProps{
			Comment:  jsii.String("Personal domain DNS"),
			ZoneName: jsii.String(domainName),
		},
	)
	// The registrar delegates to this zone's name servers; replacing it would
	// break resolution until the delegation is updated by hand.
	hostedZone.ApplyRemovalPolicy(awscdk.RemovalPolicy_RETAIN)

	vpsRecord := awsroute53.NewARecord(stack, jsii.String("VPSRecord"), &awsroute53.ARecordProps{
		RecordName: jsii.String(vpsRecordName),
		Target:     awsroute53.RecordTarget_FromIpAddresses(jsii.String(vpsIPv4)),
		Ttl:        awscdk.Duration_Seconds(jsii.Number(vpsRecordTTL)),
		Zone:       hostedZone,
	})
	for _, name := range vpsServiceNames {
		awsroute53.NewARecord(stack, jsii.String("VPSAlias-"+name), &awsroute53.ARecordProps{
			RecordName: jsii.String(name + "." + domainName),
			Target: awsroute53.RecordTarget_FromAlias(
				awsroute53targets.NewRoute53RecordTarget(vpsRecord),
			),
			Zone: hostedZone,
		})
	}

	addAddyRecords(stack, hostedZone)

	awscdk.NewCfnOutput(stack, jsii.String("HostedZoneID"), &awscdk.CfnOutputProps{
		Value:       hostedZone.HostedZoneId(),
		Description: jsii.String("Route 53 public hosted zone ID"),
	})
	awscdk.NewCfnOutput(stack, jsii.String("NameServers"), &awscdk.CfnOutputProps{
		Value:       awscdk.Fn_Join(jsii.String(","), hostedZone.HostedZoneNameServers()),
		Description: jsii.String("Name servers configured at the registrar"),
	})

	return stack
}
