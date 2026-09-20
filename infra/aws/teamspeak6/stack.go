// Package teamspeak6 provisions the personal TeamSpeak 6 ECS service.
package teamspeak6

import (
	"net/http"
	"strconv"

	"aws/internal/costtags"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigateway"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudfront"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudfrontorigins"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecrassets"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsefs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsevents"
	"github.com/aws/aws-cdk-go/awscdk/v2/awseventstargets"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambdaeventsources"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsroute53"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3deployment"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssqs"
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
	commandQueueName          = "personal-platform-teamspeak6-management-api-commands.fifo"
	commandDLQName            = "personal-platform-teamspeak6-management-api-commands-dlq.fifo"
)

// StackProps configures the TeamSpeak stack and its isolated Docker image assets.
type StackProps struct {
	awscdk.StackProps
	ImageAssetDirectory              string
	DNSUpdaterImageAssetDirectory    string
	ManagementAPIImageAssetDirectory string
	ManagementWebDirectory           string
}

type stackOutputResources struct {
	cluster       awsecs.Cluster
	service       awsecs.FargateService
	fileSystem    awsefs.FileSystem
	accessPoint   awsefs.AccessPoint
	securityGroup awsec2.SecurityGroup
	logGroup      awslogs.LogGroup
	hostedZone    awsroute53.PublicHostedZone
	managementAPI awsapigateway.RestApi
	managementKey awsapigateway.IApiKey
	managementWeb awscloudfront.Distribution
	commandQueue  awssqs.Queue
	commandDLQ    awssqs.Queue
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
	if props.ManagementAPIImageAssetDirectory == "" {
		panic("teamspeak6: management api image asset directory is required")
	}
	if props.ManagementWebDirectory == "" {
		panic("teamspeak6: management web directory is required")
	}

	stackProps := props.StackProps
	stackProps.Description = jsii.String(
		"Personal TeamSpeak 6 beta server on ECS Fargate Spot with retained EFS state",
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
	costtags.ApplyComponent(vpc, costtags.ComponentServer)

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
	costtags.ApplyComponent(taskSecurityGroup, costtags.ComponentServer)

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
	costtags.ApplyComponent(fileSystem, costtags.ComponentServer)
	costtags.ApplyComponent(accessPoint, costtags.ComponentServer)

	logGroup := awslogs.NewLogGroup(stack, jsii.String("LogGroup"), &awslogs.LogGroupProps{
		LogGroupName:  jsii.String("/personal-platform/teamspeak6"),
		RemovalPolicy: awscdk.RemovalPolicy_DESTROY,
		Retention:     awslogs.RetentionDays_ONE_WEEK,
	})
	costtags.ApplyComponent(logGroup, costtags.ComponentServer)

	cluster := awsecs.NewCluster(stack, jsii.String("Cluster"), &awsecs.ClusterProps{
		ClusterName:         jsii.String("personal-platform-teamspeak6"),
		ContainerInsightsV2: awsecs.ContainerInsights_DISABLED,
		Vpc:                 vpc,
	})
	costtags.ApplyComponent(cluster, costtags.ComponentServer)
	hostedZone := awsroute53.NewPublicHostedZone(
		stack,
		jsii.String("HostedZone"),
		&awsroute53.PublicHostedZoneProps{
			Comment:  jsii.String("Personal domain DNS managed by the TeamSpeak infrastructure stack"),
			ZoneName: jsii.String(domainName),
		},
	)
	hostedZone.ApplyRemovalPolicy(awscdk.RemovalPolicy_RETAIN)
	costtags.ApplyComponent(hostedZone, costtags.ComponentServer)

	taskDefinition := awsecs.NewFargateTaskDefinition(
		stack,
		jsii.String("TaskDefinition"),
		&awsecs.FargateTaskDefinitionProps{
			Cpu:            jsii.Number(256),
			Family:         jsii.String("teamspeak6"),
			MemoryLimitMiB: jsii.Number(512),
			RuntimePlatform: &awsecs.RuntimePlatform{
				CpuArchitecture:       awsecs.CpuArchitecture_X86_64(),
				OperatingSystemFamily: awsecs.OperatingSystemFamily_LINUX(),
			},
		},
	)
	costtags.ApplyComponent(taskDefinition, costtags.ComponentServer)
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
		Cluster: cluster,
		CapacityProviderStrategies: &[]*awsecs.CapacityProviderStrategy{
			{
				CapacityProvider: jsii.String("FARGATE_SPOT"),
				Weight:           jsii.Number(1),
			},
		},
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
	costtags.ApplyComponent(service, costtags.ComponentServer)
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
	costtags.ApplyComponent(dnsUpdater, costtags.ComponentServer)
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
	costtags.ApplyComponent(dnsUpdateRule, costtags.ComponentServer)
	service.Node().AddDependency(dnsUpdateRule)
	management := newManagementAPI(
		stack,
		cluster,
		service,
		props.ManagementAPIImageAssetDirectory,
	)
	managementWeb := newManagementWeb(
		stack,
		management.api,
		props.ManagementWebDirectory,
	)

	newOutputs(stack, stackOutputResources{
		cluster:       cluster,
		service:       service,
		fileSystem:    fileSystem,
		accessPoint:   accessPoint,
		securityGroup: taskSecurityGroup,
		logGroup:      logGroup,
		hostedZone:    hostedZone,
		managementAPI: management.api,
		managementKey: management.key,
		managementWeb: managementWeb,
		commandQueue:  management.commandQueue,
		commandDLQ:    management.commandDLQ,
	})

	return stack
}

type managementAPIResources struct {
	api          awsapigateway.RestApi
	key          awsapigateway.IApiKey
	commandQueue awssqs.Queue
	commandDLQ   awssqs.Queue
}

func newManagementAPI(
	stack awscdk.Stack,
	cluster awsecs.Cluster,
	service awsecs.FargateService,
	imageAssetDirectory string,
) managementAPIResources {
	logGroup := awslogs.NewLogGroup(stack, jsii.String("ManagementAPILogGroup"), &awslogs.LogGroupProps{
		LogGroupName:  jsii.String("/personal-platform/teamspeak6/management-api"),
		RemovalPolicy: awscdk.RemovalPolicy_DESTROY,
		Retention:     awslogs.RetentionDays_ONE_WEEK,
	})
	commandDLQ := awssqs.NewQueue(stack, jsii.String("ManagementAPICommandDLQ"), &awssqs.QueueProps{
		ContentBasedDeduplication: jsii.Bool(true),
		Encryption:                awssqs.QueueEncryption_SQS_MANAGED,
		EnforceSSL:                jsii.Bool(true),
		Fifo:                      jsii.Bool(true),
		QueueName:                 jsii.String(commandDLQName),
		RemovalPolicy:             awscdk.RemovalPolicy_DESTROY,
		RetentionPeriod:           awscdk.Duration_Days(jsii.Number(14)),
	})
	commandQueue := awssqs.NewQueue(stack, jsii.String("ManagementAPICommandQueue"), &awssqs.QueueProps{
		ContentBasedDeduplication: jsii.Bool(true),
		DeadLetterQueue: &awssqs.DeadLetterQueue{
			MaxReceiveCount: jsii.Number(5),
			Queue:           commandDLQ,
		},
		Encryption:        awssqs.QueueEncryption_SQS_MANAGED,
		EnforceSSL:        jsii.Bool(true),
		Fifo:              jsii.Bool(true),
		QueueName:         jsii.String(commandQueueName),
		RemovalPolicy:     awscdk.RemovalPolicy_DESTROY,
		VisibilityTimeout: awscdk.Duration_Seconds(jsii.Number(90)),
	})
	function := awslambda.NewDockerImageFunction(
		stack,
		jsii.String("ManagementAPIFunction"),
		&awslambda.DockerImageFunctionProps{
			Architecture: awslambda.Architecture_X86_64(),
			Code: awslambda.DockerImageCode_FromImageAsset(
				jsii.String(imageAssetDirectory),
				&awslambda.AssetImageCodeProps{
					Platform: awsecrassets.Platform_LINUX_AMD64(),
				},
			),
			Description: jsii.String("Start, stop, and report status for the singleton TeamSpeak service"),
			Environment: &map[string]*string{
				"CLUSTER_ARN":  cluster.ClusterArn(),
				"SERVICE_NAME": service.ServiceName(),
			},
			FunctionName: jsii.String("personal-platform-teamspeak6-management-api"),
			LogGroup:     logGroup,
			MemorySize:   jsii.Number(128),
			Timeout:      awscdk.Duration_Seconds(jsii.Number(15)),
		},
	)
	function.AddToRolePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions: &[]*string{
			jsii.String("ecs:DescribeServices"),
			jsii.String("ecs:UpdateService"),
		},
		Resources: &[]*string{service.ServiceArn()},
	}))
	function.AddToRolePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
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
	function.AddToRolePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions:   &[]*string{jsii.String("ecs:DescribeTasks")},
		Resources: &[]*string{taskARN},
	}))
	function.AddEventSource(awslambdaeventsources.NewSqsEventSource(
		commandQueue,
		&awslambdaeventsources.SqsEventSourceProps{
			BatchSize:               jsii.Number(1),
			ReportBatchItemFailures: jsii.Bool(true),
		},
	))

	api := awsapigateway.NewRestApi(stack, jsii.String("ManagementAPI"), &awsapigateway.RestApiProps{
		ApiKeySourceType: awsapigateway.ApiKeySourceType_HEADER,
		CloudWatchRole:   jsii.Bool(false),
		DeployOptions: &awsapigateway.StageOptions{
			DataTraceEnabled: jsii.Bool(false),
			LoggingLevel:     awsapigateway.MethodLoggingLevel_OFF,
			StageName:        jsii.String("prod"),
		},
		Description: jsii.String("TeamSpeak singleton lifecycle management API"),
		EndpointTypes: &[]awsapigateway.EndpointType{
			awsapigateway.EndpointType_REGIONAL,
		},
		RestApiName: jsii.String("personal-platform-teamspeak6-management-api"),
	})
	api.Node().TryRemoveChild(jsii.String("Endpoint"))
	for _, response := range []struct {
		id           string
		responseType awsapigateway.ResponseType
	}{
		{id: "Default4XX", responseType: awsapigateway.ResponseType_DEFAULT_4XX()},
		{id: "Default5XX", responseType: awsapigateway.ResponseType_DEFAULT_5XX()},
	} {
		api.AddGatewayResponse(jsii.String(response.id), &awsapigateway.GatewayResponseOptions{
			Type: response.responseType,
			ResponseHeaders: &map[string]*string{
				"Access-Control-Allow-Origin": jsii.String("'*'"),
			},
		})
	}
	integration := awsapigateway.NewLambdaIntegration(function, nil)
	methodOptions := &awsapigateway.MethodOptions{
		ApiKeyRequired:    jsii.Bool(true),
		AuthorizationType: awsapigateway.AuthorizationType_NONE,
	}
	preflight := &awsapigateway.CorsOptions{
		AllowOrigins:     awsapigateway.Cors_ALL_ORIGINS(),
		AllowMethods:     &[]*string{jsii.String(http.MethodGet), jsii.String(http.MethodPost)},
		AllowHeaders:     &[]*string{jsii.String("X-Api-Key")},
		AllowCredentials: jsii.Bool(false),
	}
	resourceOptions := &awsapigateway.ResourceOptions{DefaultCorsPreflightOptions: preflight}
	api.Root().AddResource(jsii.String("start"), resourceOptions).AddMethod(
		jsii.String("POST"),
		integration,
		methodOptions,
	)
	api.Root().AddResource(jsii.String("stop"), resourceOptions).AddMethod(
		jsii.String("POST"),
		integration,
		methodOptions,
	)
	api.Root().AddResource(jsii.String("status"), resourceOptions).AddMethod(
		jsii.String("GET"),
		integration,
		methodOptions,
	)

	key := api.AddApiKey(jsii.String("ManagementAPIKey"), &awsapigateway.ApiKeyOptions{
		ApiKeyName:  jsii.String("personal-platform-teamspeak6-management-api"),
		Description: jsii.String("Private key for the TeamSpeak lifecycle management API"),
	})
	usagePlan := api.AddUsagePlan(jsii.String("ManagementAPIUsagePlan"), &awsapigateway.UsagePlanProps{
		Description: jsii.String("Low-rate access to TeamSpeak lifecycle operations"),
		Name:        jsii.String("personal-platform-teamspeak6-management-api"),
		Throttle: &awsapigateway.ThrottleSettings{
			BurstLimit: jsii.Number(5),
			RateLimit:  jsii.Number(2),
		},
	})
	usagePlan.AddApiStage(&awsapigateway.UsagePlanPerApiStage{
		Api:   api,
		Stage: api.DeploymentStage(),
	})
	usagePlan.AddApiKey(key, nil)

	costtags.ApplyComponent(logGroup, costtags.ComponentManagementAPI)
	costtags.ApplyComponent(commandDLQ, costtags.ComponentManagementAPI)
	costtags.ApplyComponent(commandQueue, costtags.ComponentManagementAPI)
	costtags.ApplyComponent(function, costtags.ComponentManagementAPI)
	costtags.ApplyComponent(api, costtags.ComponentManagementAPI)
	costtags.ApplyComponent(key, costtags.ComponentManagementAPI)
	costtags.ApplyComponent(usagePlan, costtags.ComponentManagementAPI)

	return managementAPIResources{
		api:          api,
		key:          key,
		commandQueue: commandQueue,
		commandDLQ:   commandDLQ,
	}
}

func newManagementWeb(
	stack awscdk.Stack,
	api awsapigateway.RestApi,
	assetDirectory string,
) awscloudfront.Distribution {
	bucket := awss3.NewBucket(stack, jsii.String("ManagementWebBucket"), &awss3.BucketProps{
		BlockPublicAccess: awss3.BlockPublicAccess_BLOCK_ALL(),
		Encryption:        awss3.BucketEncryption_S3_MANAGED,
		EnforceSSL:        jsii.Bool(true),
		RemovalPolicy:     awscdk.RemovalPolicy_DESTROY,
	})
	origin := awscloudfrontorigins.S3BucketOrigin_WithOriginAccessControl(bucket, nil)
	distribution := awscloudfront.NewDistribution(
		stack,
		jsii.String("ManagementWebDistribution"),
		&awscloudfront.DistributionProps{
			Comment:           jsii.String("Private TeamSpeak lifecycle management SPA"),
			DefaultRootObject: jsii.String("index.html"),
			DefaultBehavior: &awscloudfront.BehaviorOptions{
				AllowedMethods:       awscloudfront.AllowedMethods_ALLOW_GET_HEAD_OPTIONS(),
				Compress:             jsii.Bool(true),
				Origin:               origin,
				ViewerProtocolPolicy: awscloudfront.ViewerProtocolPolicy_REDIRECT_TO_HTTPS,
			},
		},
	)
	runtimeConfig := awscdk.Fn_Join(jsii.String(""), &[]*string{
		jsii.String("window.__TS6_RUNTIME_CONFIG__ = Object.freeze({ apiBaseUrl: \""),
		api.Url(),
		jsii.String("\" });\n"),
	})
	deployment := awss3deployment.NewBucketDeployment(
		stack,
		jsii.String("ManagementWebDeployment"),
		&awss3deployment.BucketDeploymentProps{
			DestinationBucket: bucket,
			Distribution:      distribution,
			DistributionPaths: &[]*string{jsii.String("/*")},
			Prune:             jsii.Bool(true),
			RetainOnDelete:    jsii.Bool(false),
			Sources: &[]awss3deployment.ISource{
				awss3deployment.Source_Asset(jsii.String(assetDirectory), nil),
				awss3deployment.Source_Data(
					jsii.String("runtime-config.js"),
					runtimeConfig,
					&awss3deployment.MarkersConfig{JsonEscape: jsii.Bool(false)},
				),
			},
		},
	)
	costtags.ApplyComponent(bucket, costtags.ComponentManagementWeb)
	costtags.ApplyComponent(distribution, costtags.ComponentManagementWeb)
	costtags.ApplyComponent(deployment, costtags.ComponentManagementWeb)
	deploymentProvider := deployment.HandlerRole().Node().Scope()
	// BucketDeployment's singleton provider is a stack sibling, not a deployment child.
	costtags.ApplyComponent(deploymentProvider, costtags.ComponentManagementWeb)

	return distribution
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
			FunctionName:  jsii.String("personal-platform-teamspeak6-dns-updater"),
			LogGroup:      logGroup,
			MemorySize:    jsii.Number(128),
			RetryAttempts: jsii.Number(2),
			Timeout:       awscdk.Duration_Seconds(jsii.Number(30)),
		},
	)
	costtags.ApplyComponent(logGroup, costtags.ComponentServer)
	costtags.ApplyComponent(updater, costtags.ComponentServer)
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
		{
			id:          "ManagementAPIURL",
			value:       resources.managementAPI.Url(),
			description: "TeamSpeak management API prod-stage URL",
		},
		{
			id:          "ManagementAPIKeyID",
			value:       resources.managementKey.KeyId(),
			description: "API key ID used to retrieve the private key value",
		},
		{
			id: "ManagementWebURL",
			value: awscdk.Fn_Join(jsii.String(""), &[]*string{
				jsii.String("https://"),
				resources.managementWeb.DistributionDomainName(),
			}),
			description: "CloudFront URL for the TeamSpeak management web app",
		},
		{
			id:          "ManagementAPICommandQueueURL",
			value:       resources.commandQueue.QueueUrl(),
			description: "FIFO queue URL for TeamSpeak lifecycle commands",
		},
		{
			id:          "ManagementAPICommandQueueARN",
			value:       resources.commandQueue.QueueArn(),
			description: "FIFO queue ARN for TeamSpeak lifecycle commands",
		},
		{
			id:          "ManagementAPICommandDLQURL",
			value:       resources.commandDLQ.QueueUrl(),
			description: "Dead-letter queue URL for failed lifecycle commands",
		},
	}

	for _, output := range outputs {
		awscdk.NewCfnOutput(stack, jsii.String(output.id), &awscdk.CfnOutputProps{
			Description: jsii.String(output.description),
			Value:       output.value,
		})
	}
}
