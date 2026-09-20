package ecs

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsecs "github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/diogo-nb/personal-platform/apps/ts6-management-api/internal/domain/lifecycle"
)

type fakeECS struct {
	describeServicesOutput *awsecs.DescribeServicesOutput
	describeServicesError  error
	updateServiceOutput    *awsecs.UpdateServiceOutput
	updateServiceError     error
	listTasksOutput        *awsecs.ListTasksOutput
	listTasksError         error
	describeTasksOutput    *awsecs.DescribeTasksOutput
	describeTasksError     error
	describeServicesInput  *awsecs.DescribeServicesInput
	updateServiceInput     *awsecs.UpdateServiceInput
	listTasksInput         *awsecs.ListTasksInput
	describeTasksInput     *awsecs.DescribeTasksInput
	contexts               []context.Context
}

func (f *fakeECS) DescribeServices(
	ctx context.Context,
	input *awsecs.DescribeServicesInput,
	_ ...func(*awsecs.Options),
) (*awsecs.DescribeServicesOutput, error) {
	f.contexts = append(f.contexts, ctx)
	f.describeServicesInput = input
	return f.describeServicesOutput, f.describeServicesError
}

func (f *fakeECS) UpdateService(
	ctx context.Context,
	input *awsecs.UpdateServiceInput,
	_ ...func(*awsecs.Options),
) (*awsecs.UpdateServiceOutput, error) {
	f.contexts = append(f.contexts, ctx)
	f.updateServiceInput = input
	return f.updateServiceOutput, f.updateServiceError
}

func (f *fakeECS) ListTasks(
	ctx context.Context,
	input *awsecs.ListTasksInput,
	_ ...func(*awsecs.Options),
) (*awsecs.ListTasksOutput, error) {
	f.contexts = append(f.contexts, ctx)
	f.listTasksInput = input
	return f.listTasksOutput, f.listTasksError
}

func (f *fakeECS) DescribeTasks(
	ctx context.Context,
	input *awsecs.DescribeTasksInput,
	_ ...func(*awsecs.Options),
) (*awsecs.DescribeTasksOutput, error) {
	f.contexts = append(f.contexts, ctx)
	f.describeTasksInput = input
	return f.describeTasksOutput, f.describeTasksError
}

func TestNew(t *testing.T) {
	t.Parallel()

	api := &fakeECS{}
	tests := []struct {
		name        string
		api         ecsAPI
		clusterARN  string
		serviceName string
		wantError   bool
	}{
		{name: "valid dependencies", api: api, clusterARN: "cluster", serviceName: "service"},
		{name: "missing client", clusterARN: "cluster", serviceName: "service", wantError: true},
		{name: "missing cluster arn", api: api, serviceName: "service", wantError: true},
		{name: "missing service name", api: api, clusterARN: "cluster", wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := New(test.api, test.clusterARN, test.serviceName)
			if (err != nil) != test.wantError {
				t.Fatalf("New() error = %v, wantError %t", err, test.wantError)
			}
		})
	}
}

func TestClient_Snapshot(t *testing.T) {
	t.Parallel()

	type contextKey string
	ctx := context.WithValue(t.Context(), contextKey("request"), "value")
	api := &fakeECS{describeServicesOutput: &awsecs.DescribeServicesOutput{Services: []types.Service{{
		ServiceArn:   aws.String("service-arn"),
		ServiceName:  aws.String("service"),
		DesiredCount: 1,
		RunningCount: 1,
		PendingCount: 0,
	}}}}
	client := newClient(t, api)

	got, err := client.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	want := lifecycle.Snapshot{Desired: 1, Running: 1}
	if got != want {
		t.Errorf("Snapshot() = %#v, want %#v", got, want)
	}
	wantInput := &awsecs.DescribeServicesInput{
		Cluster:  aws.String("cluster"),
		Services: []string{"service"},
	}
	if !reflect.DeepEqual(api.describeServicesInput, wantInput) {
		t.Errorf("DescribeServices input = %#v, want %#v", api.describeServicesInput, wantInput)
	}
	assertContexts(t, api.contexts, ctx)
}

func TestClient_SnapshotRejectsMalformedResponses(t *testing.T) {
	t.Parallel()

	validService := types.Service{ServiceArn: aws.String("service-arn"), ServiceName: aws.String("service")}
	tests := []struct {
		name string
		api  *fakeECS
	}{
		{name: "aws failure", api: &fakeECS{describeServicesError: errors.New("unavailable")}},
		{name: "nil output", api: &fakeECS{}},
		{name: "reported failure", api: &fakeECS{describeServicesOutput: &awsecs.DescribeServicesOutput{Failures: []types.Failure{{Arn: aws.String("service")}}}}},
		{name: "missing service", api: &fakeECS{describeServicesOutput: &awsecs.DescribeServicesOutput{}}},
		{name: "multiple services", api: &fakeECS{describeServicesOutput: &awsecs.DescribeServicesOutput{Services: []types.Service{validService, validService}}}},
		{name: "missing service arn", api: &fakeECS{describeServicesOutput: &awsecs.DescribeServicesOutput{Services: []types.Service{{ServiceName: aws.String("service")}}}}},
		{name: "mismatched service name", api: &fakeECS{describeServicesOutput: &awsecs.DescribeServicesOutput{Services: []types.Service{{ServiceArn: aws.String("service-arn"), ServiceName: aws.String("other")}}}}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if _, err := newClient(t, test.api).Snapshot(t.Context()); err == nil {
				t.Fatal("Snapshot() error = nil, want error")
			}
		})
	}
}

func TestClient_SetDesiredCount(t *testing.T) {
	t.Parallel()

	type contextKey string
	ctx := context.WithValue(t.Context(), contextKey("request"), "value")
	api := &fakeECS{updateServiceOutput: &awsecs.UpdateServiceOutput{Service: &types.Service{
		ServiceArn:   aws.String("service-arn"),
		ServiceName:  aws.String("service"),
		DesiredCount: 1,
	}}}
	client := newClient(t, api)

	if err := client.SetDesiredCount(ctx, 1); err != nil {
		t.Fatalf("SetDesiredCount() error = %v", err)
	}
	wantInput := &awsecs.UpdateServiceInput{
		Cluster:      aws.String("cluster"),
		Service:      aws.String("service"),
		DesiredCount: aws.Int32(1),
	}
	if !reflect.DeepEqual(api.updateServiceInput, wantInput) {
		t.Errorf("UpdateService input = %#v, want %#v", api.updateServiceInput, wantInput)
	}
	assertContexts(t, api.contexts, ctx)
}

func TestClient_SetDesiredCountRejectsMalformedResponses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		api  *fakeECS
	}{
		{name: "aws failure", api: &fakeECS{updateServiceError: errors.New("unavailable")}},
		{name: "nil output", api: &fakeECS{}},
		{name: "missing service", api: &fakeECS{updateServiceOutput: &awsecs.UpdateServiceOutput{}}},
		{name: "mismatched desired count", api: &fakeECS{updateServiceOutput: updateOutput("service-arn", "service", 0)}},
		{name: "missing service arn", api: &fakeECS{updateServiceOutput: updateOutput("", "service", 1)}},
		{name: "mismatched service name", api: &fakeECS{updateServiceOutput: updateOutput("service-arn", "other", 1)}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if err := newClient(t, test.api).SetDesiredCount(t.Context(), 1); err == nil {
				t.Fatal("SetDesiredCount() error = nil, want error")
			}
		})
	}
}

func TestClient_RunningTask(t *testing.T) {
	t.Parallel()

	type contextKey string
	ctx := context.WithValue(t.Context(), contextKey("request"), "value")
	startedAt := time.Date(2026, time.September, 19, 15, 30, 0, 0, time.FixedZone("BRT", -3*60*60))
	api := &fakeECS{
		listTasksOutput: &awsecs.ListTasksOutput{TaskArns: []string{"task"}},
		describeTasksOutput: &awsecs.DescribeTasksOutput{Tasks: []types.Task{{
			TaskArn:    aws.String("task"),
			LastStatus: aws.String("RUNNING"),
			StartedAt:  &startedAt,
		}}},
	}
	client := newClient(t, api)

	got, err := client.RunningTask(ctx)
	if err != nil {
		t.Fatalf("RunningTask() error = %v", err)
	}
	if !got.StartedAt.Equal(startedAt) || got.StartedAt.Location() != startedAt.Location() {
		t.Errorf("RunningTask() = %#v, want start time %v", got, startedAt)
	}
	wantListInput := &awsecs.ListTasksInput{
		Cluster:       aws.String("cluster"),
		DesiredStatus: types.DesiredStatusRunning,
		ServiceName:   aws.String("service"),
	}
	if !reflect.DeepEqual(api.listTasksInput, wantListInput) {
		t.Errorf("ListTasks input = %#v, want %#v", api.listTasksInput, wantListInput)
	}
	wantDescribeInput := &awsecs.DescribeTasksInput{
		Cluster: aws.String("cluster"),
		Tasks:   []string{"task"},
	}
	if !reflect.DeepEqual(api.describeTasksInput, wantDescribeInput) {
		t.Errorf("DescribeTasks input = %#v, want %#v", api.describeTasksInput, wantDescribeInput)
	}
	assertContexts(t, api.contexts, ctx, ctx)
}

func TestClient_RunningTaskRejectsMalformedResponses(t *testing.T) {
	t.Parallel()

	startedAt := time.Now().UTC()
	validTask := types.Task{TaskArn: aws.String("task"), LastStatus: aws.String("RUNNING"), StartedAt: &startedAt}
	validList := &awsecs.ListTasksOutput{TaskArns: []string{"task"}}
	tests := []struct {
		name string
		api  *fakeECS
	}{
		{name: "list aws failure", api: &fakeECS{listTasksError: errors.New("unavailable")}},
		{name: "nil list output", api: &fakeECS{}},
		{name: "list pagination", api: &fakeECS{listTasksOutput: &awsecs.ListTasksOutput{NextToken: aws.String("next")}}},
		{name: "no task arn", api: &fakeECS{listTasksOutput: &awsecs.ListTasksOutput{}}},
		{name: "multiple task arns", api: &fakeECS{listTasksOutput: &awsecs.ListTasksOutput{TaskArns: []string{"one", "two"}}}},
		{name: "empty task arn", api: &fakeECS{listTasksOutput: &awsecs.ListTasksOutput{TaskArns: []string{""}}}},
		{name: "describe aws failure", api: &fakeECS{listTasksOutput: validList, describeTasksError: errors.New("unavailable")}},
		{name: "nil describe output", api: &fakeECS{listTasksOutput: validList}},
		{name: "describe reported failure", api: &fakeECS{listTasksOutput: validList, describeTasksOutput: &awsecs.DescribeTasksOutput{Failures: []types.Failure{{Arn: aws.String("task")}}}}},
		{name: "no described task", api: &fakeECS{listTasksOutput: validList, describeTasksOutput: &awsecs.DescribeTasksOutput{}}},
		{name: "multiple described tasks", api: &fakeECS{listTasksOutput: validList, describeTasksOutput: &awsecs.DescribeTasksOutput{Tasks: []types.Task{validTask, validTask}}}},
		{name: "mismatched task arn", api: &fakeECS{listTasksOutput: validList, describeTasksOutput: taskOutput("other", "RUNNING", &startedAt)}},
		{name: "wrong last status", api: &fakeECS{listTasksOutput: validList, describeTasksOutput: taskOutput("task", "PENDING", &startedAt)}},
		{name: "missing start time", api: &fakeECS{listTasksOutput: validList, describeTasksOutput: taskOutput("task", "RUNNING", nil)}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if _, err := newClient(t, test.api).RunningTask(t.Context()); err == nil {
				t.Fatal("RunningTask() error = nil, want error")
			}
		})
	}
}

func newClient(t *testing.T, api *fakeECS) *Client {
	t.Helper()

	client, err := New(api, "cluster", "service")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	return client
}

func updateOutput(serviceARN, serviceName string, desired int32) *awsecs.UpdateServiceOutput {
	return &awsecs.UpdateServiceOutput{Service: &types.Service{
		ServiceArn:   aws.String(serviceARN),
		ServiceName:  aws.String(serviceName),
		DesiredCount: desired,
	}}
}

func taskOutput(taskARN, lastStatus string, startedAt *time.Time) *awsecs.DescribeTasksOutput {
	return &awsecs.DescribeTasksOutput{Tasks: []types.Task{{
		TaskArn:    aws.String(taskARN),
		LastStatus: aws.String(lastStatus),
		StartedAt:  startedAt,
	}}}
}

func assertContexts(t *testing.T, got []context.Context, want ...context.Context) {
	t.Helper()

	if !reflect.DeepEqual(got, want) {
		t.Errorf("contexts = %v, want %v", got, want)
	}
}
