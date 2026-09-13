// Package teamspeak6 provisions the personal TeamSpeak 6 ECS service.
package teamspeak6

import (
	"aws/internal/costtags"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecrassets"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsefs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

const dataVolumeName = "teamspeak6-data"

// StackProps configures the TeamSpeak stack and its local Docker image asset.
type StackProps struct {
	awscdk.StackProps
	ImageAssetDirectory string
}

// NewStack creates the TeamSpeak 6 Fargate service with license acceptance
// configured in the task environment.
func NewStack(scope constructs.Construct, id string, props *StackProps) awscdk.Stack {
	if props == nil {
		panic("teamspeak6: stack props are required")
	}
	if props.ImageAssetDirectory == "" {
		panic("teamspeak6: image asset directory is required")
	}

	stackProps := props.StackProps
	stackProps.Description = jsii.String(
		"Personal TeamSpeak 6 beta server on ECS Fargate with retained EFS state",
	)
	stackProps.Tags = costtags.Merge(stackProps.Tags, costtags.ApplicationTeamSpeak6)

	stack := awscdk.NewStack(scope, jsii.String(id), &stackProps)
	costtags.Apply(stack, costtags.ApplicationTeamSpeak6)

	vpc := awsec2.NewVpc(stack, jsii.String("Vpc"), &awsec2.VpcProps{
		CreateInternetGateway:        jsii.Bool(true),
		AvailabilityZones:            &[]*string{jsii.String("sa-east-1a")},
		IpAddresses:                  awsec2.IpAddresses_Cidr(jsii.String("10.42.0.0/24")),
		IpProtocol:                   awsec2.IpProtocol_IPV4_ONLY,
		NatGateways:                  jsii.Number(0),
		RestrictDefaultSecurityGroup: jsii.Bool(false),
		SubnetConfiguration: &[]*awsec2.SubnetConfiguration{
			{
				Name:                jsii.String("public"),
				SubnetType:          awsec2.SubnetType_PUBLIC,
				CidrMask:            jsii.Number(26),
				MapPublicIpOnLaunch: jsii.Bool(true),
			},
		},
		VpcName: jsii.String("teamspeak6"),
	})

	taskSecurityGroup := awsec2.NewSecurityGroup(
		stack,
		jsii.String("TaskSecurityGroup"),
		&awsec2.SecurityGroupProps{
			Vpc:               vpc,
			AllowAllOutbound:  jsii.Bool(true),
			Description:       jsii.String("TeamSpeak 6 public voice and file transfer"),
			SecurityGroupName: jsii.String("teamspeak6-task"),
		},
	)
	taskSecurityGroup.AddIngressRule(
		awsec2.Peer_AnyIpv4(),
		awsec2.Port_Udp(jsii.Number(9987)),
		jsii.String("TeamSpeak voice"),
		jsii.Bool(false),
	)
	taskSecurityGroup.AddIngressRule(
		awsec2.Peer_AnyIpv4(),
		awsec2.Port_Tcp(jsii.Number(30033)),
		jsii.String("TeamSpeak file transfer"),
		jsii.Bool(false),
	)

	fileSystem := awsefs.NewFileSystem(stack, jsii.String("FileSystem"), &awsefs.FileSystemProps{
		Vpc:             vpc,
		Encrypted:       jsii.Bool(true),
		FileSystemName:  jsii.String("teamspeak6-data"),
		OneZone:         jsii.Bool(true),
		PerformanceMode: awsefs.PerformanceMode_GENERAL_PURPOSE,
		RemovalPolicy:   awscdk.RemovalPolicy_RETAIN,
		ThroughputMode:  awsefs.ThroughputMode_BURSTING,
		VpcSubnets: &awsec2.SubnetSelection{
			AvailabilityZones: &[]*string{jsii.String("sa-east-1a")},
			SubnetGroupName:   jsii.String("public"),
		},
	})
	accessPoint := fileSystem.AddAccessPoint(
		jsii.String("AccessPoint"),
		&awsefs.AccessPointOptions{
			Path: jsii.String("/teamspeak6"),
			CreateAcl: &awsefs.Acl{
				OwnerUid:    jsii.String("9987"),
				OwnerGid:    jsii.String("9987"),
				Permissions: jsii.String("750"),
			},
			PosixUser: &awsefs.PosixUser{
				Uid: jsii.String("9987"),
				Gid: jsii.String("9987"),
			},
		},
	)

	logGroup := awslogs.NewLogGroup(stack, jsii.String("LogGroup"), &awslogs.LogGroupProps{
		LogGroupName:  jsii.String("/personal-platform/teamspeak6"),
		RemovalPolicy: awscdk.RemovalPolicy_DESTROY,
		Retention:     awslogs.RetentionDays_ONE_WEEK,
	})

	cluster := awsecs.NewCluster(stack, jsii.String("Cluster"), &awsecs.ClusterProps{
		ClusterName:         jsii.String("personal-platform-teamspeak6"),
		ContainerInsightsV2: awsecs.ContainerInsights_DISABLED,
		Vpc:                 vpc,
	})

	taskDefinition := awsecs.NewFargateTaskDefinition(
		stack,
		jsii.String("TaskDefinition"),
		&awsecs.FargateTaskDefinitionProps{
			Cpu:            jsii.Number(256),
			Family:         jsii.String("teamspeak6"),
			MemoryLimitMiB: jsii.Number(1024),
			RuntimePlatform: &awsecs.RuntimePlatform{
				CpuArchitecture:       awsecs.CpuArchitecture_X86_64(),
				OperatingSystemFamily: awsecs.OperatingSystemFamily_LINUX(),
			},
		},
	)
	taskDefinition.AddVolume(&awsecs.Volume{
		Name: jsii.String(dataVolumeName),
		EfsVolumeConfiguration: &awsecs.EfsVolumeConfiguration{
			FileSystemId: fileSystem.FileSystemId(),
			AuthorizationConfig: &awsecs.AuthorizationConfig{
				AccessPointId: accessPoint.AccessPointId(),
				Iam:           jsii.String("ENABLED"),
			},
			RootDirectory:     jsii.String("/"),
			TransitEncryption: jsii.String("ENABLED"),
		},
	})

	container := taskDefinition.AddContainer(
		jsii.String("Server"),
		&awsecs.ContainerDefinitionOptions{
			ContainerName: jsii.String("teamspeak6"),
			Environment: &map[string]*string{
				"TSSERVER_LICENSE_ACCEPTED": jsii.String("accept"),
			},
			Image: awsecs.ContainerImage_FromAsset(
				jsii.String(props.ImageAssetDirectory),
				&awsecs.AssetImageProps{
					Platform: awsecrassets.Platform_LINUX_AMD64(),
				},
			),
			Logging: awsecs.LogDriver_AwsLogs(&awsecs.AwsLogDriverProps{
				LogGroup:     logGroup,
				StreamPrefix: jsii.String("teamspeak6"),
			}),
			StopTimeout:      awscdk.Duration_Seconds(jsii.Number(120)),
			User:             jsii.String("9987:9987"),
			WorkingDirectory: jsii.String("/var/tsserver"),
		},
	)
	container.AddPortMappings(
		&awsecs.PortMapping{
			ContainerPort: jsii.Number(9987),
			Protocol:      awsecs.Protocol_UDP,
		},
		&awsecs.PortMapping{
			ContainerPort: jsii.Number(30033),
			Protocol:      awsecs.Protocol_TCP,
		},
	)
	container.AddMountPoints(&awsecs.MountPoint{
		ContainerPath: jsii.String("/var/tsserver"),
		ReadOnly:      jsii.Bool(false),
		SourceVolume:  jsii.String(dataVolumeName),
	})

	fileSystem.GrantReadWrite(taskDefinition.TaskRole())

	service := awsecs.NewFargateService(stack, jsii.String("Service"), &awsecs.FargateServiceProps{
		AssignPublicIp:              jsii.Bool(true),
		AvailabilityZoneRebalancing: awsecs.AvailabilityZoneRebalancing_DISABLED,
		CircuitBreaker: &awsecs.DeploymentCircuitBreaker{
			Enable:   jsii.Bool(true),
			Rollback: jsii.Bool(true),
		},
		Cluster:              cluster,
		DesiredCount:         jsii.Number(1),
		EnableECSManagedTags: jsii.Bool(true),
		MaxHealthyPercent:    jsii.Number(100),
		MinHealthyPercent:    jsii.Number(0),
		PropagateTags:        awsecs.PropagatedTagSource_SERVICE,
		SecurityGroups: &[]awsec2.ISecurityGroup{
			taskSecurityGroup,
		},
		ServiceName:    jsii.String("teamspeak6"),
		TaskDefinition: taskDefinition,
		VpcSubnets: &awsec2.SubnetSelection{
			SubnetGroupName: jsii.String("public"),
		},
	})
	fileSystem.Connections().AllowDefaultPortFrom(
		service,
		jsii.String("TeamSpeak tasks mount EFS"),
	)
	service.Node().AddDependency(fileSystem.MountTargetsAvailable())

	newOutputs(stack, cluster, service, fileSystem, accessPoint, taskSecurityGroup, logGroup)

	return stack
}

func newOutputs(
	stack awscdk.Stack,
	cluster awsecs.Cluster,
	service awsecs.FargateService,
	fileSystem awsefs.FileSystem,
	accessPoint awsefs.AccessPoint,
	taskSecurityGroup awsec2.SecurityGroup,
	logGroup awslogs.LogGroup,
) {
	outputs := []struct {
		id          string
		value       *string
		description string
	}{
		{"ClusterName", cluster.ClusterName(), "ECS cluster name"},
		{"ServiceName", service.ServiceName(), "ECS service name"},
		{"FileSystemID", fileSystem.FileSystemId(), "Retained EFS file system ID"},
		{"AccessPointID", accessPoint.AccessPointId(), "EFS access point ID"},
		{"SecurityGroupID", taskSecurityGroup.SecurityGroupId(), "TeamSpeak task security group ID"},
		{"LogGroupName", logGroup.LogGroupName(), "CloudWatch log group name"},
	}

	for _, output := range outputs {
		awscdk.NewCfnOutput(stack, jsii.String(output.id), &awscdk.CfnOutputProps{
			Description: jsii.String(output.description),
			Value:       output.value,
		})
	}
}
