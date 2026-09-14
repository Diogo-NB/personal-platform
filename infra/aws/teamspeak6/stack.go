// Package teamspeak6 provisions the personal TeamSpeak 6 ECS service.
package teamspeak6

import (
	"strconv"

	"aws/internal/costtags"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecrassets"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsefs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsevents"
	"github.com/aws/aws-cdk-go/awscdk/v2/awseventstargets"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsroute53"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

const (
	dataVolumeName            = "teamspeak6-data"
	domainName                = "diogo-nb.com.br"
	dnsRecordName             = "ts.diogo-nb.com.br"
	dnsRecordTTL              = 60
	dnsUpdaterScheduleMinutes = 5
	serviceName               = "teamspeak6"
)

// StackProps configures the TeamSpeak stack and its isolated Docker image assets.
type StackProps struct {
	awscdk.StackProps
	ImageAssetDirectory           string
	DNSUpdaterImageAssetDirectory string
}

type stackOutputResources struct {
	cluster       awsecs.Cluster
	service       awsecs.FargateService
	fileSystem    awsefs.FileSystem
	accessPoint   awsefs.AccessPoint
	securityGroup awsec2.SecurityGroup
	logGroup      awslogs.LogGroup
	hostedZone    awsroute53.PublicHostedZone
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
	if props.DNSUpdaterImageAssetDirectory == "" {
		panic("teamspeak6: dns updater image asset directory is required")
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
	hostedZone := awsroute53.NewPublicHostedZone(
		stack,
		jsii.String("HostedZone"),
		&awsroute53.PublicHostedZoneProps{
			Comment:  jsii.String("Personal domain DNS managed by the TeamSpeak infrastructure stack"),
			ZoneName: jsii.String(domainName),
		},
	)
	hostedZone.ApplyRemovalPolicy(awscdk.RemovalPolicy_RETAIN)

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
			ContainerName: jsii.String(serviceName),
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
		ServiceName:    jsii.String(serviceName),
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

	dnsUpdater := newDNSUpdater(
		stack,
		cluster,
		hostedZone,
		props.DNSUpdaterImageAssetDirectory,
	)
	dnsUpdateRule := awsevents.NewRule(stack, jsii.String("DNSUpdateRule"), &awsevents.RuleProps{
		Description: jsii.String("Keep the TeamSpeak DNS record synchronized with the running Fargate task"),
		EventPattern: &awsevents.EventPattern{
			Detail: &map[string]any{
				"clusterArn": []string{*cluster.ClusterArn()},
				"group":      []string{"service:" + serviceName},
				"lastStatus": []string{"RUNNING"},
			},
			DetailType: &[]*string{jsii.String("ECS Task State Change")},
			Source:     &[]*string{jsii.String("aws.ecs")},
		},
		RuleName: jsii.String("personal-platform-teamspeak6-dns-updater"),
		Schedule: awsevents.Schedule_Rate(
			awscdk.Duration_Minutes(jsii.Number(dnsUpdaterScheduleMinutes)),
		),
	})
	dnsUpdateRule.AddTarget(awseventstargets.NewLambdaFunction(
		dnsUpdater,
		&awseventstargets.LambdaFunctionProps{
			MaxEventAge:   awscdk.Duration_Hours(jsii.Number(1)),
			RetryAttempts: jsii.Number(10),
		},
	))
	service.Node().AddDependency(dnsUpdateRule)

	newOutputs(stack, stackOutputResources{
		cluster:       cluster,
		service:       service,
		fileSystem:    fileSystem,
		accessPoint:   accessPoint,
		securityGroup: taskSecurityGroup,
		logGroup:      logGroup,
		hostedZone:    hostedZone,
	})

	return stack
}

func newDNSUpdater(
	stack awscdk.Stack,
	cluster awsecs.Cluster,
	hostedZone awsroute53.PublicHostedZone,
	imageAssetDirectory string,
) awslambda.DockerImageFunction {
	logGroup := awslogs.NewLogGroup(stack, jsii.String("DNSUpdaterLogGroup"), &awslogs.LogGroupProps{
		LogGroupName:  jsii.String("/personal-platform/teamspeak6/dns-updater"),
		RemovalPolicy: awscdk.RemovalPolicy_DESTROY,
		Retention:     awslogs.RetentionDays_ONE_WEEK,
	})
	updater := awslambda.NewDockerImageFunction(
		stack,
		jsii.String("DNSUpdater"),
		&awslambda.DockerImageFunctionProps{
			Architecture: awslambda.Architecture_X86_64(),
			Code: awslambda.DockerImageCode_FromImageAsset(
				jsii.String(imageAssetDirectory),
				&awslambda.AssetImageCodeProps{
					Platform: awsecrassets.Platform_LINUX_AMD64(),
				},
			),
			Description: jsii.String("Update TeamSpeak DNS when the singleton Fargate task address changes"),
			Environment: &map[string]*string{
				"CLUSTER_ARN":    cluster.ClusterArn(),
				"DNS_NAME":       jsii.String(dnsRecordName),
				"DNS_TTL":        jsii.String(strconv.Itoa(dnsRecordTTL)),
				"HOSTED_ZONE_ID": hostedZone.HostedZoneId(),
				"SERVICE_NAME":   jsii.String(serviceName),
			},
			FunctionName:                 jsii.String("personal-platform-teamspeak6-dns-updater"),
			LogGroup:                     logGroup,
			MemorySize:                   jsii.Number(128),
			ReservedConcurrentExecutions: jsii.Number(1),
			RetryAttempts:                jsii.Number(2),
			Timeout:                      awscdk.Duration_Seconds(jsii.Number(30)),
		},
	)
	updater.AddToRolePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions: &[]*string{jsii.String("ecs:ListTasks")},
		Conditions: &map[string]interface{}{
			"ArnEquals": map[string]interface{}{
				"ecs:cluster": cluster.ClusterArn(),
			},
		},
		Resources: &[]*string{jsii.String("*")},
	}))
	taskARN := stack.FormatArn(&awscdk.ArnComponents{
		ArnFormat:    awscdk.ArnFormat_SLASH_RESOURCE_NAME,
		Resource:     jsii.String("task"),
		ResourceName: awscdk.Fn_Join(jsii.String(""), &[]*string{cluster.ClusterName(), jsii.String("/*")}),
		Service:      jsii.String("ecs"),
	})
	updater.AddToRolePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions: &[]*string{
			jsii.String("ecs:DescribeTasks"),
		},
		Resources: &[]*string{taskARN},
	}))
	updater.AddToRolePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions: &[]*string{
			jsii.String("ec2:DescribeNetworkInterfaces"),
		},
		Resources: &[]*string{jsii.String("*")},
	}))
	updater.AddToRolePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions: &[]*string{
			jsii.String("route53:ChangeResourceRecordSets"),
			jsii.String("route53:ListResourceRecordSets"),
		},
		Resources: &[]*string{hostedZone.HostedZoneArn()},
	}))

	return updater
}

func newOutputs(stack awscdk.Stack, resources stackOutputResources) {
	outputs := []struct {
		id          string
		value       *string
		description string
	}{
		{
			id:          "ClusterName",
			value:       resources.cluster.ClusterName(),
			description: "ECS cluster name",
		},
		{
			id:          "ServiceName",
			value:       resources.service.ServiceName(),
			description: "ECS service name",
		},
		{
			id:          "FileSystemID",
			value:       resources.fileSystem.FileSystemId(),
			description: "Retained EFS file system ID",
		},
		{
			id:          "AccessPointID",
			value:       resources.accessPoint.AccessPointId(),
			description: "EFS access point ID",
		},
		{
			id:          "SecurityGroupID",
			value:       resources.securityGroup.SecurityGroupId(),
			description: "TeamSpeak task security group ID",
		},
		{
			id:          "LogGroupName",
			value:       resources.logGroup.LogGroupName(),
			description: "CloudWatch log group name",
		},
		{
			id:          "HostedZoneID",
			value:       resources.hostedZone.HostedZoneId(),
			description: "Route 53 public hosted zone ID",
		},
		{
			id: "NameServers",
			value: awscdk.Fn_Join(
				jsii.String(","),
				resources.hostedZone.HostedZoneNameServers(),
			),
			description: "Name servers to configure once at the registrar",
		},
		{
			id:          "TeamSpeakAddress",
			value:       jsii.String(dnsRecordName),
			description: "Friendly TeamSpeak server address",
		},
	}

	for _, output := range outputs {
		awscdk.NewCfnOutput(stack, jsii.String(output.id), &awscdk.CfnOutputProps{
			Description: jsii.String(output.description),
			Value:       output.value,
		})
	}
}
