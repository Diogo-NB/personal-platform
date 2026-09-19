package ecs

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsecs "github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/diogo-nb/personal-platform/apps/ts6-management/internal/domain/lifecycle"
	portout "github.com/diogo-nb/personal-platform/apps/ts6-management/internal/port/out"
)

type ecsAPI interface {
	DescribeServices(
		context.Context,
		*awsecs.DescribeServicesInput,
		...func(*awsecs.Options),
	) (*awsecs.DescribeServicesOutput, error)
	UpdateService(
		context.Context,
		*awsecs.UpdateServiceInput,
		...func(*awsecs.Options),
	) (*awsecs.UpdateServiceOutput, error)
	ListTasks(
		context.Context,
		*awsecs.ListTasksInput,
		...func(*awsecs.Options),
	) (*awsecs.ListTasksOutput, error)
	DescribeTasks(
		context.Context,
		*awsecs.DescribeTasksInput,
		...func(*awsecs.Options),
	) (*awsecs.DescribeTasksOutput, error)
}

type Client struct {
	ecs         ecsAPI
	clusterARN  string
	serviceName string
}

func New(client ecsAPI, clusterARN, serviceName string) (*Client, error) {
	if client == nil {
		return nil, errors.New("ecs: client is required")
	}
	if clusterARN == "" {
		return nil, errors.New("ecs: cluster arn is required")
	}
	if serviceName == "" {
		return nil, errors.New("ecs: service name is required")
	}

	return &Client{ecs: client, clusterARN: clusterARN, serviceName: serviceName}, nil
}

func (c *Client) Snapshot(ctx context.Context) (lifecycle.Snapshot, error) {
	output, err := c.ecs.DescribeServices(ctx, &awsecs.DescribeServicesInput{
		Cluster:  aws.String(c.clusterARN),
		Services: []string{c.serviceName},
	})
	if err != nil {
		return lifecycle.Snapshot{}, fmt.Errorf("ecs describe services: %w", err)
	}
	if output == nil {
		return lifecycle.Snapshot{}, errors.New("ecs describe services returned an invalid response")
	}

	hasFailures := len(output.Failures) != 0
	hasUnexpectedServiceCount := len(output.Services) != 1
	if hasFailures || hasUnexpectedServiceCount {
		return lifecycle.Snapshot{}, errors.New("ecs describe services returned an invalid response")
	}

	service := output.Services[0]
	if aws.ToString(service.ServiceArn) == "" || aws.ToString(service.ServiceName) != c.serviceName {
		return lifecycle.Snapshot{}, errors.New("ecs describe services returned an invalid service")
	}

	return lifecycle.Snapshot{
		Desired: service.DesiredCount,
		Running: service.RunningCount,
		Pending: service.PendingCount,
	}, nil
}

func (c *Client) SetDesiredCount(ctx context.Context, desired int32) error {
	output, err := c.ecs.UpdateService(ctx, &awsecs.UpdateServiceInput{
		Cluster:      aws.String(c.clusterARN),
		Service:      aws.String(c.serviceName),
		DesiredCount: aws.Int32(desired),
	})
	if err != nil {
		return fmt.Errorf("ecs update service: %w", err)
	}
	if output == nil || output.Service == nil {
		return errors.New("ecs update service returned an invalid response")
	}

	service := output.Service
	if service.DesiredCount != desired ||
		aws.ToString(service.ServiceArn) == "" ||
		aws.ToString(service.ServiceName) != c.serviceName {
		return errors.New("ecs update service returned an invalid response")
	}

	return nil
}

func (c *Client) RunningTask(ctx context.Context) (lifecycle.RunningTask, error) {
	listOutput, err := c.ecs.ListTasks(ctx, &awsecs.ListTasksInput{
		Cluster:       aws.String(c.clusterARN),
		DesiredStatus: types.DesiredStatusRunning,
		ServiceName:   aws.String(c.serviceName),
	})
	if err != nil {
		return lifecycle.RunningTask{}, fmt.Errorf("ecs list tasks: %w", err)
	}
	if listOutput == nil || listOutput.NextToken != nil {
		return lifecycle.RunningTask{}, errors.New("ecs list tasks returned an invalid response")
	}
	if len(listOutput.TaskArns) != 1 || listOutput.TaskArns[0] == "" {
		return lifecycle.RunningTask{}, errors.New("ecs running service does not have exactly one task")
	}

	taskARN := listOutput.TaskArns[0]
	describeOutput, err := c.ecs.DescribeTasks(ctx, &awsecs.DescribeTasksInput{
		Cluster: aws.String(c.clusterARN),
		Tasks:   []string{taskARN},
	})
	if err != nil {
		return lifecycle.RunningTask{}, fmt.Errorf("ecs describe tasks: %w", err)
	}
	if describeOutput == nil || len(describeOutput.Failures) != 0 || len(describeOutput.Tasks) != 1 {
		return lifecycle.RunningTask{}, errors.New("ecs describe tasks returned an invalid response")
	}

	task := describeOutput.Tasks[0]
	isMismatchedTask := aws.ToString(task.TaskArn) != taskARN
	isNotRunning := aws.ToString(task.LastStatus) != "RUNNING"
	hasNoStartTime := task.StartedAt == nil
	if isMismatchedTask || isNotRunning || hasNoStartTime {
		return lifecycle.RunningTask{}, errors.New("ecs describe tasks returned an invalid task")
	}

	return lifecycle.RunningTask{StartedAt: *task.StartedAt}, nil
}

var _ portout.Scheduler = (*Client)(nil)
