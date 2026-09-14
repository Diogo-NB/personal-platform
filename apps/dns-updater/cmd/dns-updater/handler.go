package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/netip"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	route53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
)

const (
	maxPublicIPAttempts = 10
	publicIPRetryDelay  = 2 * time.Second
)

type ecsClient interface {
	ListTasks(
		context.Context,
		*ecs.ListTasksInput,
		...func(*ecs.Options),
	) (*ecs.ListTasksOutput, error)
	DescribeTasks(
		context.Context,
		*ecs.DescribeTasksInput,
		...func(*ecs.Options),
	) (*ecs.DescribeTasksOutput, error)
}

type ec2Client interface {
	DescribeNetworkInterfaces(
		context.Context,
		*ec2.DescribeNetworkInterfacesInput,
		...func(*ec2.Options),
	) (*ec2.DescribeNetworkInterfacesOutput, error)
}

type route53Client interface {
	ListResourceRecordSets(
		context.Context,
		*route53.ListResourceRecordSetsInput,
		...func(*route53.Options),
	) (*route53.ListResourceRecordSetsOutput, error)
	ChangeResourceRecordSets(
		context.Context,
		*route53.ChangeResourceRecordSetsInput,
		...func(*route53.Options),
	) (*route53.ChangeResourceRecordSetsOutput, error)
}

type waitFunc func(context.Context, time.Duration) error

type updaterDependencies struct {
	ecs     ecsClient
	ec2     ec2Client
	route53 route53Client
	logger  *slog.Logger
	wait    waitFunc
}

type updater struct {
	config  updaterConfig
	clients updaterDependencies
}

func newUpdater(config updaterConfig, dependencies updaterDependencies) *updater {
	return &updater{
		config:  config,
		clients: dependencies,
	}
}

func (u *updater) handle(ctx context.Context) error {
	listOutput, err := u.clients.ecs.ListTasks(ctx, &ecs.ListTasksInput{
		Cluster:       &u.config.clusterARN,
		DesiredStatus: ecstypes.DesiredStatusRunning,
		MaxResults:    int32Pointer(2),
		ServiceName:   &u.config.serviceName,
	})
	if err != nil {
		return fmt.Errorf("listing running ecs tasks: %w", err)
	}
	if listOutput == nil {
		return fmt.Errorf("listing running ecs tasks: malformed response")
	}
	if len(listOutput.TaskArns) != 1 {
		u.clients.logger.InfoContext(
			ctx,
			"dns update skipped",
			"reason",
			"running task count is not one",
			"running_tasks",
			len(listOutput.TaskArns),
		)
		return nil
	}

	describeOutput, err := u.clients.ecs.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: &u.config.clusterARN,
		Tasks:   listOutput.TaskArns,
	})
	if err != nil {
		return fmt.Errorf("describing running ecs task: %w", err)
	}
	if describeOutput == nil || len(describeOutput.Tasks) != 1 {
		return fmt.Errorf("describing running ecs task: malformed response")
	}

	task := describeOutput.Tasks[0]
	if task.LastStatus == nil {
		return fmt.Errorf("describing running ecs task: response has no last status")
	}
	if *task.LastStatus != string(ecstypes.DesiredStatusRunning) {
		u.clients.logger.InfoContext(
			ctx,
			"dns update skipped",
			"reason",
			"listed task is no longer running",
		)
		return nil
	}

	networkInterfaceID, err := networkInterfaceID(task)
	if err != nil {
		return fmt.Errorf("extracting task network interface: %w", err)
	}
	publicIP, err := u.waitForPublicIP(ctx, networkInterfaceID)
	if err != nil {
		return err
	}

	currentIP, hasCurrentIP, err := u.currentIP(ctx)
	if err != nil {
		return err
	}
	if hasCurrentIP && currentIP == publicIP {
		u.clients.logger.InfoContext(
			ctx,
			"dns update skipped",
			"reason",
			"record already matches running task",
			"dns_name",
			u.config.dnsName,
		)
		return nil
	}

	_, err = u.clients.route53.ChangeResourceRecordSets(
		ctx,
		&route53.ChangeResourceRecordSetsInput{
			HostedZoneId: &u.config.hostedZoneID,
			ChangeBatch: &route53types.ChangeBatch{
				Comment: stringPointer("Current singleton ECS service task public IPv4"),
				Changes: []route53types.Change{
					{
						Action: route53types.ChangeActionUpsert,
						ResourceRecordSet: &route53types.ResourceRecordSet{
							Name: &u.config.dnsName,
							ResourceRecords: []route53types.ResourceRecord{
								{Value: &publicIP},
							},
							TTL:  &u.config.dnsTTLSeconds,
							Type: route53types.RRTypeA,
						},
					},
				},
			},
		},
	)
	if err != nil {
		return fmt.Errorf("upserting route 53 record: %w", err)
	}

	u.clients.logger.InfoContext(ctx, "dns record updated", "dns_name", u.config.dnsName)
	return nil
}

func networkInterfaceID(task ecstypes.Task) (string, error) {
	for _, attachment := range task.Attachments {
		if attachment.Type == nil || *attachment.Type != "ElasticNetworkInterface" {
			continue
		}
		for _, detail := range attachment.Details {
			if detail.Name == nil || *detail.Name != "networkInterfaceId" {
				continue
			}
			if detail.Value == nil || *detail.Value == "" {
				return "", fmt.Errorf("network interface id is empty")
			}
			return *detail.Value, nil
		}
	}

	return "", fmt.Errorf("task has no elastic network interface")
}

func (u *updater) waitForPublicIP(ctx context.Context, networkInterfaceID string) (string, error) {
	for attempt := range maxPublicIPAttempts {
		if err := ctx.Err(); err != nil {
			return "", fmt.Errorf("waiting for network interface public ip: %w", err)
		}

		output, err := u.clients.ec2.DescribeNetworkInterfaces(
			ctx,
			&ec2.DescribeNetworkInterfacesInput{
				NetworkInterfaceIds: []string{networkInterfaceID},
			},
		)
		if err != nil {
			return "", fmt.Errorf("describing network interface: %w", err)
		}
		if output == nil || len(output.NetworkInterfaces) > 1 {
			return "", fmt.Errorf("describing network interface: malformed response")
		}
		if len(output.NetworkInterfaces) == 1 {
			networkInterface := output.NetworkInterfaces[0]
			if networkInterface.Association != nil && networkInterface.Association.PublicIp != nil {
				publicIP := *networkInterface.Association.PublicIp
				parsedIP, err := netip.ParseAddr(publicIP)
				if err != nil || !parsedIP.Is4() {
					return "", fmt.Errorf("network interface public ip is not ipv4")
				}
				return parsedIP.String(), nil
			}
		}

		if attempt == maxPublicIPAttempts-1 {
			break
		}
		if err := u.clients.wait(ctx, publicIPRetryDelay); err != nil {
			return "", fmt.Errorf("waiting to retry network interface lookup: %w", err)
		}
	}

	return "", fmt.Errorf("network interface has no public ipv4 after %d attempts", maxPublicIPAttempts)
}

func (u *updater) currentIP(ctx context.Context) (string, bool, error) {
	output, err := u.clients.route53.ListResourceRecordSets(
		ctx,
		&route53.ListResourceRecordSetsInput{
			HostedZoneId:    &u.config.hostedZoneID,
			MaxItems:        int32Pointer(1),
			StartRecordName: &u.config.dnsName,
			StartRecordType: route53types.RRTypeA,
		},
	)
	if err != nil {
		return "", false, fmt.Errorf("listing route 53 records: %w", err)
	}
	if output == nil {
		return "", false, fmt.Errorf("listing route 53 records: malformed response")
	}
	if len(output.ResourceRecordSets) == 0 {
		return "", false, nil
	}

	record := output.ResourceRecordSets[0]
	if record.Name == nil {
		return "", false, fmt.Errorf("listing route 53 records: record has no name")
	}
	recordName := strings.TrimSuffix(strings.ToLower(*record.Name), ".")
	if recordName != u.config.dnsName || record.Type != route53types.RRTypeA {
		return "", false, nil
	}
	if len(record.ResourceRecords) != 1 {
		return "", false, nil
	}
	if record.ResourceRecords[0].Value == nil || *record.ResourceRecords[0].Value == "" {
		return "", false, fmt.Errorf("listing route 53 records: a record value is empty")
	}

	return *record.ResourceRecords[0].Value, true, nil
}

func waitForContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func int32Pointer(value int32) *int32 {
	return &value
}

func stringPointer(value string) *string {
	return &value
}
