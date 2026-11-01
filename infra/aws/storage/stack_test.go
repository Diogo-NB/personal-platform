package storage

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

func TestNewBackupStackTagsBucket(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := NewBackupStack(app, "TestStack", nil)
	template := assertions.Template_FromStack(stack, nil)

	template.HasResourceProperties(jsii.String("AWS::S3::Bucket"), map[string]any{
		"Tags": []any{
			map[string]any{"Key": "Application", "Value": "skycrate"},
			map[string]any{"Key": "Environment", "Value": "production"},
			map[string]any{"Key": "Project", "Value": "personal-platform"},
		},
	})
}

func TestNewBackupStackRetainsBucketAndPolicy(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := NewBackupStack(app, "TestStack", nil)

	template := assertions.Template_FromStack(stack, nil)
	for _, resourceType := range []string{"AWS::S3::Bucket", "AWS::S3::BucketPolicy"} {
		t.Run(resourceType, func(t *testing.T) {
			template.HasResource(jsii.String(resourceType), map[string]any{
				"DeletionPolicy":      "Retain",
				"UpdateReplacePolicy": "Retain",
			})
		})
	}
}
