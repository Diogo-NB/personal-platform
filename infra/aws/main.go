package main

import (
	"path/filepath"

	"aws/dns"
	"aws/teamspeak6"

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

	imageAssetDirectory, err := filepath.Abs(filepath.Join("..", "..", "apps", "ts6"))
	if err != nil {
		panic(err)
	}
	dnsUpdaterImageAssetDirectory, err := filepath.Abs(
		filepath.Join("..", "..", "apps", "dns-updater"),
	)
	if err != nil {
		panic(err)
	}
	managementAPIImageAssetDirectory, err := filepath.Abs(
		filepath.Join("..", "..", "apps", "ts6-management-api"),
	)
	if err != nil {
		panic(err)
	}
	managementWebDirectory, err := filepath.Abs(
		filepath.Join("..", "..", "apps", "ts6-management-web", "dist"),
	)
	if err != nil {
		panic(err)
	}
	teamspeak6.NewStack(
		app,
		"PersonalPlatformTeamspeak6Stack",
		&teamspeak6.StackProps{
			StackProps: awscdk.StackProps{
				Env: &awscdk.Environment{
					Region: jsii.String("sa-east-1"),
				},
			},
			ImageAssetDirectory:              imageAssetDirectory,
			DNSUpdaterImageAssetDirectory:    dnsUpdaterImageAssetDirectory,
			ManagementAPIImageAssetDirectory: managementAPIImageAssetDirectory,
			ManagementWebDirectory:           managementWebDirectory,
			HostedZoneID:                     hostedZoneID(app),
		},
	)

	app.Synth(nil)
}

func hostedZoneID(app awscdk.App) string {
	id, _ := app.Node().TryGetContext(jsii.String("hostedZoneId")).(string)
	return id
}
