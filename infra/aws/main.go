package main

import (
	"os"
	"path/filepath"

	"aws/storage"
	"aws/teamspeak6"
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/jsii-runtime-go"
)

func main() {
	defer jsii.Close()

	app := awscdk.NewApp(nil)

	storage.NewBackupStack(
		app,
		"PersonalPlatformStorageStack",
		&storage.BackupStackProps{
			StackProps: awscdk.StackProps{
				Env: &awscdk.Environment{
					Account: jsii.String(os.Getenv("CDK_DEFAULT_ACCOUNT")),
					Region:  jsii.String("us-east-1"),
				},
			},
		},
	)

	imageAssetDirectory, err := filepath.Abs(filepath.Join("..", "..", "apps", "ts6"))
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
			ImageAssetDirectory: imageAssetDirectory,
		},
	)

	app.Synth(nil)
}
