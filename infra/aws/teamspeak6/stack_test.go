package teamspeak6

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
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

func TestNewStackTemplate(t *testing.T) {
	template := newTemplate(t)

	t.Run("fargate runtime image ports and storage", func(t *testing.T) {
		template.HasResourceProperties(jsii.String("AWS::ECS::TaskDefinition"), map[string]any{
			"Cpu":                     "256",
			"Memory":                  "1024",
			"NetworkMode":             "awsvpc",
			"RequiresCompatibilities": []any{"FARGATE"},
			"RuntimePlatform": map[string]any{
				"CpuArchitecture":       "X86_64",
				"OperatingSystemFamily": "LINUX",
			},
			"ContainerDefinitions": assertions.Match_ArrayWith(&[]any{
				assertions.Match_ObjectLike(&map[string]any{
					"Image": assertions.Match_ObjectLike(&map[string]any{
						"Fn::Sub": assertions.Match_StringLikeRegexp(jsii.String(".*dkr\\.ecr\\..*")),
					}),
					"Environment": assertions.Match_ArrayWith(&[]any{
						map[string]any{
							"Name":  "TSSERVER_LICENSE_ACCEPTED",
							"Value": "accept",
						},
					}),
					"MountPoints": []any{
						map[string]any{
							"ContainerPath": "/var/tsserver",
							"ReadOnly":      false,
							"SourceVolume":  dataVolumeName,
						},
					},
					"PortMappings": assertions.Match_ArrayWith(&[]any{
						map[string]any{"ContainerPort": float64(9987), "Protocol": "udp"},
						map[string]any{"ContainerPort": float64(30033), "Protocol": "tcp"},
					}),
					"StopTimeout":      float64(120),
					"User":             "9987:9987",
					"WorkingDirectory": "/var/tsserver",
				}),
			}),
			"Volumes": assertions.Match_ArrayWith(&[]any{
				assertions.Match_ObjectLike(&map[string]any{
					"Name": dataVolumeName,
					"EFSVolumeConfiguration": map[string]any{
						"AuthorizationConfig": map[string]any{
							"AccessPointId": assertions.Match_AnyValue(),
							"IAM":           "ENABLED",
						},
						"FilesystemId":      assertions.Match_AnyValue(),
						"RootDirectory":     "/",
						"TransitEncryption": "ENABLED",
					},
				}),
			}),
		})
	})

	t.Run("public singleton service stops before start and rolls back", func(t *testing.T) {
		template.HasResourceProperties(jsii.String("AWS::ECS::Service"), map[string]any{
			"AvailabilityZoneRebalancing": "DISABLED",
			"DeploymentConfiguration": map[string]any{
				"DeploymentCircuitBreaker": map[string]any{
					"Enable":   true,
					"Rollback": true,
				},
				"MaximumPercent":        float64(100),
				"MinimumHealthyPercent": float64(0),
			},
			"DesiredCount":         float64(1),
			"EnableECSManagedTags": true,
			"LaunchType":           "FARGATE",
			"NetworkConfiguration": map[string]any{
				"AwsvpcConfiguration": map[string]any{
					"AssignPublicIp": "ENABLED",
				},
			},
			"PropagateTags": "SERVICE",
			"ServiceName":   "teamspeak6",
		})
		template.HasResourceProperties(jsii.String("AWS::ECS::Cluster"), map[string]any{
			"ClusterName": "personal-platform-teamspeak6",
		})
	})

	t.Run("encrypted retained one zone efs access point", func(t *testing.T) {
		template.HasResource(jsii.String("AWS::EFS::FileSystem"), map[string]any{
			"DeletionPolicy":      "Retain",
			"UpdateReplacePolicy": "Retain",
			"Properties": map[string]any{
				"AvailabilityZoneName": assertions.Match_AnyValue(),
				"Encrypted":            true,
				"PerformanceMode":      "generalPurpose",
				"ThroughputMode":       "bursting",
			},
		})
		template.HasResourceProperties(jsii.String("AWS::EFS::AccessPoint"), map[string]any{
			"PosixUser": map[string]any{
				"Gid": "9987",
				"Uid": "9987",
			},
			"RootDirectory": map[string]any{
				"CreationInfo": map[string]any{
					"OwnerGid":    "9987",
					"OwnerUid":    "9987",
					"Permissions": "750",
				},
				"Path": "/teamspeak6",
			},
		})
	})

	t.Run("network and logs", func(t *testing.T) {
		template.HasResourceProperties(jsii.String("AWS::EC2::VPC"), map[string]any{
			"CidrBlock": "10.42.0.0/24",
		})
		template.HasResourceProperties(jsii.String("AWS::EC2::Subnet"), map[string]any{
			"AvailabilityZone":    "sa-east-1a",
			"CidrBlock":           "10.42.0.0/26",
			"MapPublicIpOnLaunch": true,
		})
		template.ResourceCountIs(jsii.String("AWS::EC2::Subnet"), jsii.Number(1))
		template.ResourceCountIs(jsii.String("AWS::EC2::NatGateway"), jsii.Number(0))
		template.HasResourceProperties(jsii.String("AWS::Logs::LogGroup"), map[string]any{
			"LogGroupName":    "/personal-platform/teamspeak6",
			"RetentionInDays": float64(7),
		})

		expected := []ingressRule{
			{protocol: "tcp", fromPort: 2049, toPort: 2049, isPublic: false},
			{protocol: "udp", fromPort: 9987, toPort: 9987, isPublic: true},
			{protocol: "tcp", fromPort: 30033, toPort: 30033, isPublic: true},
		}
		if got := ingressRules(t, template); !reflect.DeepEqual(got, expected) {
			t.Errorf("ingress rules = %#v, want %#v", got, expected)
		}
	})

	t.Run("tags and outputs", func(t *testing.T) {
		assertCostAllocationTags(t, template, "teamspeak6")

		expectedOutputs := []string{
			"ClusterName",
			"ServiceName",
			"FileSystemID",
			"AccessPointID",
			"SecurityGroupID",
			"LogGroupName",
		}
		for _, output := range expectedOutputs {
			template.HasOutput(jsii.String(output), map[string]any{
				"Value": assertions.Match_AnyValue(),
			})
		}

		outputs, ok := (*template.ToJSON())["Outputs"].(map[string]any)
		if !ok {
			t.Fatal("template Outputs is not an object")
		}
		actualOutputs := make([]string, 0, len(outputs))
		for output := range outputs {
			actualOutputs = append(actualOutputs, output)
		}
		sort.Strings(actualOutputs)
		sort.Strings(expectedOutputs)
		if !reflect.DeepEqual(actualOutputs, expectedOutputs) {
			t.Errorf("outputs = %v, want %v", actualOutputs, expectedOutputs)
		}
	})

	t.Run("excluded infrastructure", func(t *testing.T) {
		for _, resourceType := range []string{
			"AWS::EC2::Instance",
			"AWS::EC2::Volume",
			"AWS::ElasticLoadBalancing::LoadBalancer",
			"AWS::ElasticLoadBalancingV2::LoadBalancer",
			"AWS::ECR::Repository",
			"AWS::Lambda::Function",
			"AWS::RDS::DBCluster",
			"AWS::RDS::DBInstance",
		} {
			template.ResourceCountIs(jsii.String(resourceType), jsii.Number(0))
		}
	})
}

func assertCostAllocationTags(
	t *testing.T,
	template assertions.Template,
	application string,
) {
	t.Helper()

	tagPropertyByResourceType := map[string]string{
		"AWS::EC2::InternetGateway": "Tags",
		"AWS::EC2::RouteTable":      "Tags",
		"AWS::EC2::SecurityGroup":   "Tags",
		"AWS::EC2::Subnet":          "Tags",
		"AWS::EC2::VPC":             "Tags",
		"AWS::ECS::Cluster":         "Tags",
		"AWS::ECS::Service":         "Tags",
		"AWS::ECS::TaskDefinition":  "Tags",
		"AWS::EFS::AccessPoint":     "AccessPointTags",
		"AWS::EFS::FileSystem":      "FileSystemTags",
		"AWS::IAM::Role":            "Tags",
		"AWS::Logs::LogGroup":       "Tags",
	}
	expected := map[string]string{
		"Application": application,
		"Environment": "production",
		"Project":     "personal-storage",
	}

	resources, ok := (*template.ToJSON())["Resources"].(map[string]any)
	if !ok {
		t.Fatal("template Resources is not an object")
	}
	seenResourceTypes := map[string]bool{}
	for logicalID, resourceValue := range resources {
		resource, ok := resourceValue.(map[string]any)
		if !ok {
			continue
		}
		resourceType, _ := resource["Type"].(string)
		tagProperty, shouldBeTagged := tagPropertyByResourceType[resourceType]
		if !shouldBeTagged {
			continue
		}
		seenResourceTypes[resourceType] = true

		properties, _ := resource["Properties"].(map[string]any)
		tagValues, ok := properties[tagProperty].([]any)
		if !ok {
			t.Errorf("%s (%s) has no %s", logicalID, resourceType, tagProperty)
			continue
		}

		actual := map[string]string{}
		for _, tagValue := range tagValues {
			tag, ok := tagValue.(map[string]any)
			if !ok {
				t.Errorf("%s (%s) tag is %T, want object", logicalID, resourceType, tagValue)
				continue
			}
			key, keyOK := tag["Key"].(string)
			value, valueOK := tag["Value"].(string)
			if keyOK && valueOK {
				actual[key] = value
			}
		}

		for key, expectedValue := range expected {
			if actual[key] != expectedValue {
				t.Errorf(
					"%s (%s) tag %s = %q, want %q",
					logicalID,
					resourceType,
					key,
					actual[key],
					expectedValue,
				)
			}
		}
		for _, omittedKey := range []string{"ManagedBy", "Owner"} {
			if _, exists := actual[omittedKey]; exists {
				t.Errorf("%s (%s) has unwanted %s tag", logicalID, resourceType, omittedKey)
			}
		}
	}

	for resourceType := range tagPropertyByResourceType {
		if !seenResourceTypes[resourceType] {
			t.Errorf("no %s resource found to verify tags", resourceType)
		}
	}
}

type ingressRule struct {
	protocol string
	fromPort int
	toPort   int
	isPublic bool
}

func newTemplate(t *testing.T) assertions.Template {
	t.Helper()

	app := awscdk.NewApp(nil)
	stack := NewStack(app, "TestStack", &StackProps{
		StackProps: awscdk.StackProps{
			Env: &awscdk.Environment{
				Account: jsii.String("123456789012"),
				Region:  jsii.String("sa-east-1"),
			},
		},
		ImageAssetDirectory: imageAssetDirectory(t),
	})

	return assertions.Template_FromStack(stack, nil)
}

func imageAssetDirectory(t *testing.T) string {
	t.Helper()

	directory, err := filepath.Abs(filepath.Join("..", "..", "..", "apps", "ts6"))
	if err != nil {
		t.Fatalf("resolve image asset directory: %v", err)
	}

	return directory
}

func ingressRules(t *testing.T, template assertions.Template) []ingressRule {
	t.Helper()

	resources, ok := (*template.ToJSON())["Resources"].(map[string]any)
	if !ok {
		t.Fatal("template Resources is not an object")
	}

	var rules []ingressRule
	for _, resourceValue := range resources {
		resource, ok := resourceValue.(map[string]any)
		if !ok {
			continue
		}
		properties, _ := resource["Properties"].(map[string]any)

		switch resource["Type"] {
		case "AWS::EC2::SecurityGroupIngress":
			rules = append(rules, parseIngressRule(t, properties))
		case "AWS::EC2::SecurityGroup":
			inline, _ := properties["SecurityGroupIngress"].([]any)
			for _, ruleValue := range inline {
				rule, ok := ruleValue.(map[string]any)
				if !ok {
					t.Fatalf("inline ingress rule is %T, want object", ruleValue)
				}
				rules = append(rules, parseIngressRule(t, rule))
			}
		}
	}

	sort.Slice(rules, func(i, j int) bool {
		if rules[i].fromPort == rules[j].fromPort {
			return rules[i].protocol < rules[j].protocol
		}
		return rules[i].fromPort < rules[j].fromPort
	})

	return rules
}

func parseIngressRule(t *testing.T, properties map[string]any) ingressRule {
	t.Helper()

	fromPort, ok := properties["FromPort"].(float64)
	if !ok {
		t.Fatalf("ingress FromPort is %T, want number", properties["FromPort"])
	}
	toPort, ok := properties["ToPort"].(float64)
	if !ok {
		t.Fatalf("ingress ToPort is %T, want number", properties["ToPort"])
	}
	protocol, ok := properties["IpProtocol"].(string)
	if !ok {
		t.Fatalf("ingress IpProtocol is %T, want string", properties["IpProtocol"])
	}

	return ingressRule{
		protocol: protocol,
		fromPort: int(fromPort),
		toPort:   int(toPort),
		isPublic: properties["CidrIp"] == "0.0.0.0/0",
	}
}
