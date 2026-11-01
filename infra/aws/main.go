package main

import (
	"aws/dns"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/jsii-runtime-go"
)

func main() {
	defer jsii.Close()

	app := awscdk.NewApp(nil)

	dns.NewStack(
		app,
		"PersonalPlatformDnsStack",
		&dns.StackProps{
			StackProps: awscdk.StackProps{
				Env: &awscdk.Environment{
					Region: jsii.String("sa-east-1"),
				},
			},
		},
	)

	app.Synth(nil)
}
