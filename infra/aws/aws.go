package main

import (
	"os"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

type BackupStackProps struct {
	awscdk.StackProps
}

func NewBackupStack(
	scope constructs.Construct,
	id string,
	props *BackupStackProps,
) awscdk.Stack {
	var stackProps awscdk.StackProps

	if props != nil {
		stackProps = props.StackProps
	}

	stack := awscdk.NewStack(
		scope,
		jsii.String(id),
		&stackProps,
	)

	bucket := awss3.NewBucket(
		stack,
		jsii.String("BackupBucket"),
		&awss3.BucketProps{
			BucketName: jsii.String("skycrate-storage"),

			// Accidental stack deletion must NOT delete backups.
			RemovalPolicy: awscdk.RemovalPolicy_RETAIN,

			// Keep previous object versions.
			Versioned: jsii.Bool(true),

			// Explicit SSE-S3 / AES-256.
			Encryption: awss3.BucketEncryption_S3_MANAGED,

			// Disable all public access.
			BlockPublicAccess: awss3.BlockPublicAccess_BLOCK_ALL(),

			// Disable ACLs. IAM + bucket policies control access.
			ObjectOwnership: awss3.ObjectOwnership_BUCKET_OWNER_ENFORCED,

			// CDK creates the appropriate bucket policy denying HTTP.
			EnforceSSL: jsii.Bool(true),

			// Allow the cold-tier transition to include objects smaller than 128 KiB.
			TransitionDefaultMinimumObjectSize: awss3.TransitionDefaultMinimumObjectSize_VARIES_BY_STORAGE_CLASS,

			LifecycleRules: &[]*awss3.LifecycleRule{
				{
					Id:      jsii.String("AbortIncompleteMultipartUploads"),
					Enabled: jsii.Bool(true),

					AbortIncompleteMultipartUploadAfter: awscdk.Duration_Days(jsii.Number(7)),
				},
				{
					Id:      jsii.String("DeleteOldVersions"),
					Enabled: jsii.Bool(true),

					NoncurrentVersionExpiration: awscdk.Duration_Days(jsii.Number(365)),
				},
				{
					Id:                        jsii.String("DeleteExpiredDeleteMarkers"),
					Enabled:                   jsii.Bool(true),
					ExpiredObjectDeleteMarker: jsii.Bool(true),
				},
				{
					Id:      jsii.String("TransitionColdToDeepArchive"),
					Enabled: jsii.Bool(true),
					TagFilters: &map[string]any{
						"storage-tier": "cold",
					},
					Transitions: &[]*awss3.Transition{
						{
							StorageClass:    awss3.StorageClass_DEEP_ARCHIVE(),
							TransitionAfter: awscdk.Duration_Days(jsii.Number(365)),
						},
					},
				},
			},
		},
	)

	// Export the fixed bucket name for tooling and stack consumers.
	awscdk.NewCfnOutput(
		stack,
		jsii.String("BackupBucketName"),
		&awscdk.CfnOutputProps{
			Value:       bucket.BucketName(),
			Description: jsii.String("Name of the personal backup S3 bucket"),
		},
	)

	awscdk.NewCfnOutput(
		stack,
		jsii.String("BackupBucketArn"),
		&awscdk.CfnOutputProps{
			Value:       bucket.BucketArn(),
			Description: jsii.String("ARN of the personal backup S3 bucket"),
		},
	)

	return stack
}

func main() {
	defer jsii.Close()

	app := awscdk.NewApp(nil)

	NewBackupStack(
		app,
		"PersonalPlatformStorageStack",
		&BackupStackProps{
			StackProps: awscdk.StackProps{
				Env: &awscdk.Environment{
					Account: jsii.String(os.Getenv("CDK_DEFAULT_ACCOUNT")),
					Region:  jsii.String("us-east-1"),
				},
				Description: jsii.String(
					"Long-term personal backup infrastructure",
				),
			},
		},
	)

	app.Synth(nil)
}
