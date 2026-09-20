// Package costtags defines the shared AWS cost-allocation tag taxonomy.
package costtags

import (
	"maps"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

const (
	// ApplicationSkycrate identifies resources used by the Skycrate application.
	ApplicationSkycrate = "skycrate"
	// ApplicationTeamSpeak6 identifies resources used by the TeamSpeak 6 application.
	ApplicationTeamSpeak6 = "teamspeak6"
	// ComponentServer identifies the TeamSpeak server and its supporting resources.
	ComponentServer = "server"
	// ComponentManagementAPI identifies the TeamSpeak lifecycle API resources.
	ComponentManagementAPI = "management-api"
	// ComponentManagementWeb identifies the TeamSpeak browser application resources.
	ComponentManagementWeb = "management-web"
)

const project = "personal-platform"

// Merge returns stack tags containing the shared taxonomy and any additional
// caller-provided tags. Shared keys take precedence to keep cost reporting
// consistent across stacks.
func Merge(existing *map[string]*string, application string) *map[string]*string {
	tags := map[string]*string{}
	if existing != nil {
		maps.Copy(tags, *existing)
	}

	for key, value := range values(application) {
		tags[key] = jsii.String(value)
	}

	return &tags
}

// Apply adds the shared taxonomy to every taggable resource in the stack.
func Apply(stack awscdk.Stack, application string) {
	for key, value := range values(application) {
		awscdk.Tags_Of(stack).Add(jsii.String(key), jsii.String(value), nil)
	}
}

// ApplyComponent adds the component dimension to a construct and its taggable descendants.
func ApplyComponent(scope constructs.IConstruct, component string) {
	awscdk.Tags_Of(scope).Add(jsii.String("Component"), jsii.String(component), nil)
}

func values(application string) map[string]string {
	return map[string]string{
		"Project":     project,
		"Application": application,
		"Environment": "production",
	}
}
