package teamspeak6

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
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
			"Memory":                  "512",
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
			"CapacityProviderStrategy": []any{
				map[string]any{
					"CapacityProvider": "FARGATE_SPOT",
					"Weight":           float64(1),
				},
			},
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
			"LaunchType":           assertions.Match_Absent(),
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

	t.Run("automatic dns for the current singleton task", func(t *testing.T) {
		template.HasResource(jsii.String("AWS::Route53::HostedZone"), map[string]any{
			"DeletionPolicy":      "Retain",
			"UpdateReplacePolicy": "Retain",
			"Properties": map[string]any{
				"Name": domainName + ".",
			},
		})
		template.HasResourceProperties(jsii.String("AWS::Lambda::Function"), map[string]any{
			"Architectures": []any{"x86_64"},
			"Code": map[string]any{
				"ImageUri": assertions.Match_ObjectLike(&map[string]any{
					"Fn::Sub": assertions.Match_StringLikeRegexp(jsii.String(".*dkr\\.ecr\\..*")),
				}),
			},
			"Environment": map[string]any{
				"Variables": map[string]any{
					"CLUSTER_ARN":    assertions.Match_AnyValue(),
					"DNS_NAME":       dnsRecordName,
					"DNS_TTL":        "60",
					"HOSTED_ZONE_ID": assertions.Match_AnyValue(),
					"SERVICE_NAME":   "teamspeak6",
				},
			},
			"FunctionName":                 "personal-platform-teamspeak6-dns-updater",
			"Handler":                      assertions.Match_Absent(),
			"MemorySize":                   float64(128),
			"PackageType":                  "Image",
			"ReservedConcurrentExecutions": assertions.Match_Absent(),
			"Runtime":                      assertions.Match_Absent(),
			"Timeout":                      float64(30),
		})
		template.HasResourceProperties(jsii.String("AWS::Lambda::EventInvokeConfig"), map[string]any{
			"MaximumRetryAttempts": float64(2),
		})
		template.HasResourceProperties(jsii.String("AWS::Logs::LogGroup"), map[string]any{
			"LogGroupName":    "/personal-platform/teamspeak6/dns-updater",
			"RetentionInDays": float64(7),
		})
		template.HasResourceProperties(jsii.String("AWS::Events::Rule"), map[string]any{
			"EventPattern": map[string]any{
				"detail": map[string]any{
					"clusterArn": assertions.Match_AnyValue(),
					"group":      []any{"service:teamspeak6"},
					"lastStatus": []any{"RUNNING"},
				},
				"detail-type": []any{"ECS Task State Change"},
				"source":      []any{"aws.ecs"},
			},
			"Name":               "personal-platform-teamspeak6-dns-updater",
			"ScheduleExpression": "rate(5 minutes)",
			"State":              "ENABLED",
			"Targets": assertions.Match_ArrayWith(&[]any{
				assertions.Match_ObjectLike(&map[string]any{
					"RetryPolicy": map[string]any{
						"MaximumEventAgeInSeconds": float64(3600),
						"MaximumRetryAttempts":     float64(10),
					},
				}),
			}),
		})
		template.HasResourceProperties(jsii.String("AWS::IAM::Policy"), map[string]any{
			"PolicyDocument": map[string]any{
				"Statement": assertions.Match_ArrayWith(&[]any{
					assertions.Match_ObjectLike(&map[string]any{
						"Action":    "ecs:ListTasks",
						"Condition": assertions.Match_AnyValue(),
						"Effect":    "Allow",
						"Resource":  "*",
					}),
					assertions.Match_ObjectLike(&map[string]any{
						"Action":   "ecs:DescribeTasks",
						"Effect":   "Allow",
						"Resource": assertions.Match_AnyValue(),
					}),
					assertions.Match_ObjectLike(&map[string]any{
						"Action":   "ec2:DescribeNetworkInterfaces",
						"Effect":   "Allow",
						"Resource": "*",
					}),
					assertions.Match_ObjectLike(&map[string]any{
						"Action": assertions.Match_ArrayWith(&[]any{
							"route53:ChangeResourceRecordSets",
							"route53:ListResourceRecordSets",
						}),
						"Effect":   "Allow",
						"Resource": assertions.Match_AnyValue(),
					}),
				}),
				"Version": "2012-10-17",
			},
		})
		template.ResourceCountIs(jsii.String("AWS::Route53::RecordSet"), jsii.Number(0))
		template.HasOutput(jsii.String("TeamSpeakAddress"), map[string]any{
			"Value": dnsRecordName,
		})
		assertDistinctApplicationImageAssets(t, template)
	})

	t.Run("management lambda and rest api", func(t *testing.T) {
		template.HasResourceProperties(jsii.String("AWS::Lambda::Function"), map[string]any{
			"Architectures": []any{"x86_64"},
			"Code": map[string]any{
				"ImageUri": assertions.Match_ObjectLike(&map[string]any{
					"Fn::Sub": assertions.Match_StringLikeRegexp(jsii.String(".*dkr\\.ecr\\..*")),
				}),
			},
			"Environment": map[string]any{
				"Variables": map[string]any{
					"CLUSTER_ARN":  assertions.Match_AnyValue(),
					"SERVICE_NAME": assertions.Match_AnyValue(),
				},
			},
			"FunctionName":                 "personal-platform-teamspeak6-management",
			"Handler":                      assertions.Match_Absent(),
			"MemorySize":                   float64(128),
			"PackageType":                  "Image",
			"ReservedConcurrentExecutions": assertions.Match_Absent(),
			"Runtime":                      assertions.Match_Absent(),
			"Timeout":                      float64(15),
		})
		template.HasResourceProperties(jsii.String("AWS::Logs::LogGroup"), map[string]any{
			"LogGroupName":    "/personal-platform/teamspeak6/management",
			"RetentionInDays": float64(7),
		})
		template.ResourceCountIs(jsii.String("AWS::Lambda::Function"), jsii.Number(2))
		template.HasResourceProperties(jsii.String("AWS::SQS::Queue"), map[string]any{
			"ContentBasedDeduplication": true,
			"FifoQueue":                 true,
			"QueueName":                 commandDLQName,
			"MessageRetentionPeriod":    float64(1_209_600),
			"SqsManagedSseEnabled":      true,
		})
		template.HasResourceProperties(jsii.String("AWS::SQS::Queue"), map[string]any{
			"ContentBasedDeduplication": true,
			"FifoQueue":                 true,
			"QueueName":                 commandQueueName,
			"RedrivePolicy": map[string]any{
				"deadLetterTargetArn": assertions.Match_AnyValue(),
				"maxReceiveCount":     float64(5),
			},
			"SqsManagedSseEnabled": true,
			"VisibilityTimeout":    float64(90),
		})
		template.ResourceCountIs(jsii.String("AWS::SQS::Queue"), jsii.Number(2))
		template.HasResourceProperties(jsii.String("AWS::Lambda::EventSourceMapping"), map[string]any{
			"BatchSize": float64(1),
			"EventSourceArn": map[string]any{
				"Fn::GetAtt": []any{
					assertions.Match_StringLikeRegexp(jsii.String("ManagementCommandQueue.*")),
					"Arn",
				},
			},
			"FunctionName": map[string]any{
				"Ref": assertions.Match_StringLikeRegexp(jsii.String("ManagementFunction.*")),
			},
			"FunctionResponseTypes": []any{"ReportBatchItemFailures"},
		})
		template.ResourceCountIs(jsii.String("AWS::Lambda::EventSourceMapping"), jsii.Number(1))

		template.HasResourceProperties(jsii.String("AWS::ApiGateway::RestApi"), map[string]any{
			"ApiKeySourceType": "HEADER",
			"Body":             assertions.Match_Absent(),
			"EndpointConfiguration": map[string]any{
				"Types": []any{"REGIONAL"},
			},
			"Name": "personal-platform-teamspeak6-management",
		})
		template.HasResourceProperties(jsii.String("AWS::ApiGateway::Stage"), map[string]any{
			"StageName": "prod",
		})
		template.ResourceCountIs(jsii.String("AWS::ApiGateway::Stage"), jsii.Number(1))
		template.HasResourceProperties(jsii.String("AWS::ApiGateway::ApiKey"), map[string]any{
			"Enabled": true,
			"Name":    "personal-platform-teamspeak6-management",
			"Value":   assertions.Match_Absent(),
		})
		template.HasResourceProperties(jsii.String("AWS::ApiGateway::UsagePlan"), map[string]any{
			"ApiStages": assertions.Match_ArrayWith(&[]any{
				assertions.Match_ObjectLike(&map[string]any{
					"ApiId": assertions.Match_AnyValue(),
					"Stage": assertions.Match_AnyValue(),
				}),
			}),
			"Quota": assertions.Match_Absent(),
			"Throttle": map[string]any{
				"BurstLimit": float64(5),
				"RateLimit":  float64(2),
			},
			"UsagePlanName": "personal-platform-teamspeak6-management",
		})
		template.HasResourceProperties(jsii.String("AWS::ApiGateway::UsagePlanKey"), map[string]any{
			"KeyId":       assertions.Match_AnyValue(),
			"KeyType":     "API_KEY",
			"UsagePlanId": assertions.Match_AnyValue(),
		})
		template.ResourceCountIs(jsii.String("AWS::ApiGateway::UsagePlanKey"), jsii.Number(1))

		expectedRoutes := map[string]apiRoute{
			"start":  {method: "POST", apiKeyRequired: true},
			"stop":   {method: "POST", apiKeyRequired: true},
			"status": {method: "GET", apiKeyRequired: true},
		}
		if got := apiRoutes(t, template); !reflect.DeepEqual(got, expectedRoutes) {
			t.Errorf("API routes = %#v, want %#v", got, expectedRoutes)
		}

		expectedActions := []string{
			"ecs:DescribeServices",
			"ecs:DescribeTasks",
			"ecs:ListTasks",
			"ecs:UpdateService",
		}
		if got := ecsIAMActions(t, template); !reflect.DeepEqual(got, expectedActions) {
			t.Errorf("ECS IAM actions = %v, want %v", got, expectedActions)
		}
		expectedSQSActions := []string{
			"sqs:ChangeMessageVisibility",
			"sqs:DeleteMessage",
			"sqs:GetQueueAttributes",
			"sqs:GetQueueUrl",
			"sqs:ReceiveMessage",
		}
		if got := sqsIAMActions(t, template); !reflect.DeepEqual(got, expectedSQSActions) {
			t.Errorf("SQS IAM actions = %v, want %v", got, expectedSQSActions)
		}
		template.HasResourceProperties(jsii.String("AWS::IAM::Policy"), map[string]any{
			"PolicyDocument": map[string]any{
				"Statement": assertions.Match_ArrayWith(&[]any{
					assertions.Match_ObjectLike(&map[string]any{
						"Action": assertions.Match_ArrayWith(&[]any{
							"ecs:DescribeServices",
							"ecs:UpdateService",
						}),
						"Effect":   "Allow",
						"Resource": assertions.Match_AnyValue(),
					}),
					assertions.Match_ObjectLike(&map[string]any{
						"Action":    "ecs:ListTasks",
						"Condition": assertions.Match_AnyValue(),
						"Effect":    "Allow",
						"Resource":  "*",
					}),
					assertions.Match_ObjectLike(&map[string]any{
						"Action":   "ecs:DescribeTasks",
						"Effect":   "Allow",
						"Resource": assertions.Match_AnyValue(),
					}),
				}),
				"Version": "2012-10-17",
			},
		})
		for _, resourceType := range []string{
			"AWS::ApiGateway::BasePathMapping",
			"AWS::ApiGateway::DomainName",
			"AWS::Lambda::Url",
		} {
			template.ResourceCountIs(jsii.String(resourceType), jsii.Number(0))
		}
		template.ResourceCountIs(jsii.String("AWS::Route53::RecordSet"), jsii.Number(0))
		assertDistinctApplicationImageAssets(t, template)
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
			"HostedZoneID",
			"NameServers",
			"TeamSpeakAddress",
			"ManagementAPIURL",
			"ManagementAPIKeyID",
			"ManagementCommandQueueURL",
			"ManagementCommandQueueARN",
			"ManagementCommandDLQURL",
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
			"AWS::RDS::DBCluster",
			"AWS::RDS::DBInstance",
		} {
			template.ResourceCountIs(jsii.String(resourceType), jsii.Number(0))
		}
	})
}

func TestNewStackRequiresImageAssetDirectories(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		props      *StackProps
		panicMatch string
	}{
		{
			name:       "nil props",
			props:      nil,
			panicMatch: "stack props are required",
		},
		{
			name: "missing server image directory",
			props: &StackProps{
				DNSUpdaterImageAssetDirectory: "/tmp/dns-updater",
				ManagementImageAssetDirectory: "/tmp/management",
			},
			panicMatch: "image asset directory is required",
		},
		{
			name: "missing dns updater image directory",
			props: &StackProps{
				ImageAssetDirectory:           "/tmp/teamspeak6",
				ManagementImageAssetDirectory: "/tmp/management",
			},
			panicMatch: "dns updater image asset directory is required",
		},
		{
			name: "missing management image directory",
			props: &StackProps{
				ImageAssetDirectory:           "/tmp/teamspeak6",
				DNSUpdaterImageAssetDirectory: "/tmp/dns-updater",
			},
			panicMatch: "management image asset directory is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			defer func() {
				value := recover()
				if value == nil {
					t.Fatal("NewStack() did not panic")
				}
				if !strings.Contains(fmt.Sprint(value), test.panicMatch) {
					t.Errorf("NewStack() panic = %q, want substring %q", value, test.panicMatch)
				}
			}()

			app := awscdk.NewApp(nil)
			NewStack(app, "TestStack", test.props)
		})
	}
}

func assertCostAllocationTags(
	t *testing.T,
	template assertions.Template,
	application string,
) {
	t.Helper()

	tagPropertyByResourceType := map[string]string{
		"AWS::ApiGateway::ApiKey":         "Tags",
		"AWS::ApiGateway::RestApi":        "Tags",
		"AWS::ApiGateway::Stage":          "Tags",
		"AWS::ApiGateway::UsagePlan":      "Tags",
		"AWS::EC2::InternetGateway":       "Tags",
		"AWS::EC2::RouteTable":            "Tags",
		"AWS::EC2::SecurityGroup":         "Tags",
		"AWS::EC2::Subnet":                "Tags",
		"AWS::EC2::VPC":                   "Tags",
		"AWS::ECS::Cluster":               "Tags",
		"AWS::ECS::Service":               "Tags",
		"AWS::ECS::TaskDefinition":        "Tags",
		"AWS::EFS::AccessPoint":           "AccessPointTags",
		"AWS::EFS::FileSystem":            "FileSystemTags",
		"AWS::Events::Rule":               "Tags",
		"AWS::IAM::Role":                  "Tags",
		"AWS::Lambda::EventSourceMapping": "Tags",
		"AWS::Lambda::Function":           "Tags",
		"AWS::Logs::LogGroup":             "Tags",
		"AWS::Route53::HostedZone":        "HostedZoneTags",
		"AWS::SQS::Queue":                 "Tags",
	}
	expected := map[string]string{
		"Application": application,
		"Environment": "production",
		"Project":     "personal-platform",
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

type apiRoute struct {
	method         string
	apiKeyRequired bool
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
		ImageAssetDirectory:           imageAssetDirectory(t),
		DNSUpdaterImageAssetDirectory: dnsUpdaterImageAssetDirectory(t),
		ManagementImageAssetDirectory: managementImageAssetDirectory(t),
	})

	return assertions.Template_FromStack(stack, nil)
}

func dnsUpdaterImageAssetDirectory(t *testing.T) string {
	t.Helper()

	directory, err := filepath.Abs(
		filepath.Join("..", "..", "..", "apps", "dns-updater"),
	)
	if err != nil {
		t.Fatalf("resolve dns updater image asset directory: %v", err)
	}

	return directory
}

func managementImageAssetDirectory(t *testing.T) string {
	t.Helper()

	directory, err := filepath.Abs(
		filepath.Join("..", "..", "..", "apps", "ts6-management"),
	)
	if err != nil {
		t.Fatalf("resolve management image asset directory: %v", err)
	}

	return directory
}

func assertDistinctApplicationImageAssets(t *testing.T, template assertions.Template) {
	t.Helper()

	resources, ok := (*template.ToJSON())["Resources"].(map[string]any)
	if !ok {
		t.Fatal("template Resources is not an object")
	}

	var serverImage any
	lambdaImages := map[string]any{}
	for _, resourceValue := range resources {
		resource, ok := resourceValue.(map[string]any)
		if !ok {
			continue
		}
		properties, _ := resource["Properties"].(map[string]any)
		switch resource["Type"] {
		case "AWS::ECS::TaskDefinition":
			containers, _ := properties["ContainerDefinitions"].([]any)
			if len(containers) == 1 {
				container, _ := containers[0].(map[string]any)
				serverImage = container["Image"]
			}
		case "AWS::Lambda::Function":
			code, _ := properties["Code"].(map[string]any)
			functionName, _ := properties["FunctionName"].(string)
			lambdaImages[functionName] = code["ImageUri"]
			if _, hasInlineCode := code["ZipFile"]; hasInlineCode {
				t.Error("lambda code contains an inline ZipFile")
			}
		}
	}

	updaterImage := lambdaImages["personal-platform-teamspeak6-dns-updater"]
	managementImage := lambdaImages["personal-platform-teamspeak6-management"]
	if serverImage == nil || updaterImage == nil || managementImage == nil {
		t.Fatalf(
			"image assets = server %#v, updater %#v, management %#v",
			serverImage,
			updaterImage,
			managementImage,
		)
	}
	if reflect.DeepEqual(serverImage, updaterImage) {
		t.Errorf("server and updater unexpectedly use the same image asset: %#v", serverImage)
	}
	if reflect.DeepEqual(serverImage, managementImage) {
		t.Errorf("server and management unexpectedly use the same image asset: %#v", serverImage)
	}
	if reflect.DeepEqual(updaterImage, managementImage) {
		t.Errorf("updater and management unexpectedly use the same image asset: %#v", updaterImage)
	}
}

func apiRoutes(t *testing.T, template assertions.Template) map[string]apiRoute {
	t.Helper()

	resources, ok := (*template.ToJSON())["Resources"].(map[string]any)
	if !ok {
		t.Fatal("template Resources is not an object")
	}

	pathsByLogicalID := map[string]string{}
	for logicalID, resourceValue := range resources {
		resource, ok := resourceValue.(map[string]any)
		if !ok || resource["Type"] != "AWS::ApiGateway::Resource" {
			continue
		}
		properties, _ := resource["Properties"].(map[string]any)
		path, _ := properties["PathPart"].(string)
		pathsByLogicalID[logicalID] = path
	}

	routes := map[string]apiRoute{}
	for _, resourceValue := range resources {
		resource, ok := resourceValue.(map[string]any)
		if !ok || resource["Type"] != "AWS::ApiGateway::Method" {
			continue
		}
		properties, _ := resource["Properties"].(map[string]any)
		resourceID, _ := properties["ResourceId"].(map[string]any)
		logicalID, _ := resourceID["Ref"].(string)
		path := pathsByLogicalID[logicalID]
		method, _ := properties["HttpMethod"].(string)
		apiKeyRequired, _ := properties["ApiKeyRequired"].(bool)
		authorizationType, _ := properties["AuthorizationType"].(string)
		if authorizationType != "NONE" {
			t.Errorf("route %s authorization type = %q, want NONE", path, authorizationType)
		}
		routes[path] = apiRoute{method: method, apiKeyRequired: apiKeyRequired}
	}

	return routes
}

func ecsIAMActions(t *testing.T, template assertions.Template) []string {
	return iamActions(t, template, "ecs:")
}

func sqsIAMActions(t *testing.T, template assertions.Template) []string {
	return iamActions(t, template, "sqs:")
}

func iamActions(t *testing.T, template assertions.Template, prefix string) []string {
	t.Helper()

	resources, ok := (*template.ToJSON())["Resources"].(map[string]any)
	if !ok {
		t.Fatal("template Resources is not an object")
	}

	actions := map[string]struct{}{}
	for _, resourceValue := range resources {
		resource, ok := resourceValue.(map[string]any)
		if !ok || resource["Type"] != "AWS::IAM::Policy" {
			continue
		}
		properties, _ := resource["Properties"].(map[string]any)
		document, _ := properties["PolicyDocument"].(map[string]any)
		statements, _ := document["Statement"].([]any)
		for _, statementValue := range statements {
			statement, _ := statementValue.(map[string]any)
			switch values := statement["Action"].(type) {
			case string:
				if strings.HasPrefix(values, prefix) {
					actions[values] = struct{}{}
				}
			case []any:
				for _, value := range values {
					action, _ := value.(string)
					if strings.HasPrefix(action, prefix) {
						actions[action] = struct{}{}
					}
				}
			}
		}
	}

	result := make([]string, 0, len(actions))
	for action := range actions {
		result = append(result, action)
	}
	sort.Strings(result)

	return result
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
