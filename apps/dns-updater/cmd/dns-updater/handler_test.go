package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	route53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
)

func TestUpdaterHandleSkipsNonSingletonTaskCounts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		taskARNs []string
	}{
		{name: "stopped service", taskARNs: []string{}},
		{name: "multiple running tasks", taskARNs: []string{"task/one", "task/two"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			clients := successfulClients()
			clients.ecs.listTasks = func(
				ctx context.Context,
				input *ecs.ListTasksInput,
			) (*ecs.ListTasksOutput, error) {
				if ctx != t.Context() {
					t.Error("ListTasks() did not receive the invocation context")
				}
				if input.Cluster == nil || *input.Cluster != testConfig().clusterARN {
					t.Errorf("ListTasks() cluster = %v", input.Cluster)
				}
				if input.ServiceName == nil || *input.ServiceName != testConfig().serviceName {
					t.Errorf("ListTasks() service = %v", input.ServiceName)
				}
				if input.MaxResults == nil || *input.MaxResults != 2 {
					t.Errorf("ListTasks() max results = %v, want 2", input.MaxResults)
				}
				if input.DesiredStatus != ecstypes.DesiredStatusRunning {
					t.Errorf("ListTasks() desired status = %q, want RUNNING", input.DesiredStatus)
				}
				return &ecs.ListTasksOutput{TaskArns: test.taskARNs}, nil
			}
			clients.ecs.describeTasks = func(
				context.Context,
				*ecs.DescribeTasksInput,
			) (*ecs.DescribeTasksOutput, error) {
				t.Fatal("DescribeTasks() called for a non-singleton task count")
				return nil, nil
			}

			if err := testUpdater(clients).handle(t.Context()); err != nil {
				t.Fatalf("handle() error = %v", err)
			}
		})
	}
}

func TestUpdaterHandleSkipsTaskThatStoppedAfterListing(t *testing.T) {
	t.Parallel()

	clients := successfulClients()
	clients.ecs.describeTasks = func(
		context.Context,
		*ecs.DescribeTasksInput,
	) (*ecs.DescribeTasksOutput, error) {
		return &ecs.DescribeTasksOutput{
			Tasks: []ecstypes.Task{{LastStatus: stringPointer("STOPPED")}},
		}, nil
	}
	clients.ec2.describeNetworkInterfaces = func(
		context.Context,
		*ec2.DescribeNetworkInterfacesInput,
	) (*ec2.DescribeNetworkInterfacesOutput, error) {
		t.Fatal("DescribeNetworkInterfaces() called for a stopped task")
		return nil, nil
	}

	if err := testUpdater(clients).handle(t.Context()); err != nil {
		t.Fatalf("handle() error = %v", err)
	}
}

func TestUpdaterHandlePropagatesAWSErrors(t *testing.T) {
	t.Parallel()

	awsError := errors.New("aws failure")
	tests := []struct {
		name       string
		configure  func(*fakeClients)
		errorMatch string
	}{
		{
			name: "list tasks",
			configure: func(clients *fakeClients) {
				clients.ecs.listTasks = func(context.Context, *ecs.ListTasksInput) (*ecs.ListTasksOutput, error) {
					return nil, awsError
				}
			},
			errorMatch: "listing running ecs tasks",
		},
		{
			name: "describe tasks",
			configure: func(clients *fakeClients) {
				clients.ecs.describeTasks = func(context.Context, *ecs.DescribeTasksInput) (*ecs.DescribeTasksOutput, error) {
					return nil, awsError
				}
			},
			errorMatch: "describing running ecs task",
		},
		{
			name: "describe network interface",
			configure: func(clients *fakeClients) {
				clients.ec2.describeNetworkInterfaces = func(
					context.Context,
					*ec2.DescribeNetworkInterfacesInput,
				) (*ec2.DescribeNetworkInterfacesOutput, error) {
					return nil, awsError
				}
			},
			errorMatch: "describing network interface",
		},
		{
			name: "list records",
			configure: func(clients *fakeClients) {
				clients.route53.listResourceRecordSets = func(
					context.Context,
					*route53.ListResourceRecordSetsInput,
				) (*route53.ListResourceRecordSetsOutput, error) {
					return nil, awsError
				}
			},
			errorMatch: "listing route 53 records",
		},
		{
			name: "change record",
			configure: func(clients *fakeClients) {
				clients.route53.changeResourceRecordSets = func(
					context.Context,
					*route53.ChangeResourceRecordSetsInput,
				) (*route53.ChangeResourceRecordSetsOutput, error) {
					return nil, awsError
				}
			},
			errorMatch: "upserting route 53 record",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			clients := successfulClients()
			test.configure(&clients)
			err := testUpdater(clients).handle(t.Context())
			if !errors.Is(err, awsError) {
				t.Fatalf("handle() error = %v, want wrapped AWS error", err)
			}
			if !strings.Contains(err.Error(), test.errorMatch) {
				t.Errorf("handle() error = %q, want substring %q", err, test.errorMatch)
			}
		})
	}
}

func TestUpdaterHandleRejectsMalformedAWSResponses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		configure func(*fakeClients)
	}{
		{
			name: "nil list tasks response",
			configure: func(clients *fakeClients) {
				clients.ecs.listTasks = func(context.Context, *ecs.ListTasksInput) (*ecs.ListTasksOutput, error) {
					return nil, nil
				}
			},
		},
		{
			name: "nil describe tasks response",
			configure: func(clients *fakeClients) {
				clients.ecs.describeTasks = func(context.Context, *ecs.DescribeTasksInput) (*ecs.DescribeTasksOutput, error) {
					return nil, nil
				}
			},
		},
		{
			name: "empty describe tasks response",
			configure: func(clients *fakeClients) {
				clients.ecs.describeTasks = func(context.Context, *ecs.DescribeTasksInput) (*ecs.DescribeTasksOutput, error) {
					return &ecs.DescribeTasksOutput{Tasks: []ecstypes.Task{}}, nil
				}
			},
		},
		{
			name: "task without last status",
			configure: func(clients *fakeClients) {
				clients.ecs.describeTasks = func(context.Context, *ecs.DescribeTasksInput) (*ecs.DescribeTasksOutput, error) {
					return &ecs.DescribeTasksOutput{Tasks: []ecstypes.Task{{}}}, nil
				}
			},
		},
		{
			name: "nil network interface response",
			configure: func(clients *fakeClients) {
				clients.ec2.describeNetworkInterfaces = func(
					context.Context,
					*ec2.DescribeNetworkInterfacesInput,
				) (*ec2.DescribeNetworkInterfacesOutput, error) {
					return nil, nil
				}
			},
		},
		{
			name: "multiple network interfaces",
			configure: func(clients *fakeClients) {
				clients.ec2.describeNetworkInterfaces = func(
					context.Context,
					*ec2.DescribeNetworkInterfacesInput,
				) (*ec2.DescribeNetworkInterfacesOutput, error) {
					return &ec2.DescribeNetworkInterfacesOutput{
						NetworkInterfaces: []ec2types.NetworkInterface{{}, {}},
					}, nil
				}
			},
		},
		{
			name: "nil record-list response",
			configure: func(clients *fakeClients) {
				clients.route53.listResourceRecordSets = func(
					context.Context,
					*route53.ListResourceRecordSetsInput,
				) (*route53.ListResourceRecordSetsOutput, error) {
					return nil, nil
				}
			},
		},
		{
			name: "record without name",
			configure: func(clients *fakeClients) {
				clients.route53.listResourceRecordSets = func(
					context.Context,
					*route53.ListResourceRecordSetsInput,
				) (*route53.ListResourceRecordSetsOutput, error) {
					return &route53.ListResourceRecordSetsOutput{
						ResourceRecordSets: []route53types.ResourceRecordSet{{Type: route53types.RRTypeA}},
					}, nil
				}
			},
		},
		{
			name: "record with empty value",
			configure: func(clients *fakeClients) {
				clients.route53.listResourceRecordSets = func(
					context.Context,
					*route53.ListResourceRecordSetsInput,
				) (*route53.ListResourceRecordSetsOutput, error) {
					return &route53.ListResourceRecordSetsOutput{
						ResourceRecordSets: []route53types.ResourceRecordSet{
							{
								Name: stringPointer(testConfig().dnsName),
								Type: route53types.RRTypeA,
								ResourceRecords: []route53types.ResourceRecord{
									{Value: stringPointer("")},
								},
							},
						},
					}, nil
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			clients := successfulClients()
			test.configure(&clients)
			err := testUpdater(clients).handle(t.Context())
			if err == nil {
				t.Fatal("handle() error = nil, want malformed-response error")
			}
		})
	}
}

func TestNetworkInterfaceID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		task      ecstypes.Task
		want      string
		wantError bool
	}{
		{
			name: "elastic network interface",
			task: ecstypes.Task{Attachments: []ecstypes.Attachment{
				{Type: stringPointer("Other")},
				{
					Type: stringPointer("ElasticNetworkInterface"),
					Details: []ecstypes.KeyValuePair{
						{Name: stringPointer("subnetId"), Value: stringPointer("subnet-123")},
						{Name: stringPointer("networkInterfaceId"), Value: stringPointer("eni-123")},
					},
				},
			}},
			want: "eni-123",
		},
		{
			name:      "missing elastic network interface",
			task:      ecstypes.Task{},
			wantError: true,
		},
		{
			name: "empty network interface id",
			task: ecstypes.Task{Attachments: []ecstypes.Attachment{
				{
					Type: stringPointer("ElasticNetworkInterface"),
					Details: []ecstypes.KeyValuePair{
						{Name: stringPointer("networkInterfaceId"), Value: stringPointer("")},
					},
				},
			}},
			wantError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := networkInterfaceID(test.task)
			if (err != nil) != test.wantError {
				t.Fatalf("networkInterfaceID() error = %v, wantError %t", err, test.wantError)
			}
			if got != test.want {
				t.Errorf("networkInterfaceID() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestUpdaterWaitForPublicIPRetriesUntilAddressExists(t *testing.T) {
	t.Parallel()

	clients := successfulClients()
	describeCalls := 0
	waitCalls := 0
	clients.ec2.describeNetworkInterfaces = func(
		ctx context.Context,
		input *ec2.DescribeNetworkInterfacesInput,
	) (*ec2.DescribeNetworkInterfacesOutput, error) {
		describeCalls++
		if ctx != t.Context() {
			t.Error("DescribeNetworkInterfaces() did not receive the invocation context")
		}
		if !reflect.DeepEqual(input.NetworkInterfaceIds, []string{"eni-123"}) {
			t.Errorf("network interface ids = %v, want [eni-123]", input.NetworkInterfaceIds)
		}
		if describeCalls < 3 {
			return &ec2.DescribeNetworkInterfacesOutput{
				NetworkInterfaces: []ec2types.NetworkInterface{{}},
			}, nil
		}
		return networkInterfaceOutput("203.0.113.10"), nil
	}
	clients.wait = func(ctx context.Context, duration time.Duration) error {
		waitCalls++
		if ctx != t.Context() {
			t.Error("wait() did not receive the invocation context")
		}
		if duration != publicIPRetryDelay {
			t.Errorf("wait duration = %s, want %s", duration, publicIPRetryDelay)
		}
		return nil
	}

	got, err := testUpdater(clients).waitForPublicIP(t.Context(), "eni-123")
	if err != nil {
		t.Fatalf("waitForPublicIP() error = %v", err)
	}
	if got != "203.0.113.10" {
		t.Errorf("waitForPublicIP() = %q, want 203.0.113.10", got)
	}
	if describeCalls != 3 || waitCalls != 2 {
		t.Errorf("calls = describe %d, wait %d; want 3 and 2", describeCalls, waitCalls)
	}
}

func TestUpdaterWaitForPublicIPExhaustsRetries(t *testing.T) {
	t.Parallel()

	clients := successfulClients()
	describeCalls := 0
	waitCalls := 0
	clients.ec2.describeNetworkInterfaces = func(
		context.Context,
		*ec2.DescribeNetworkInterfacesInput,
	) (*ec2.DescribeNetworkInterfacesOutput, error) {
		describeCalls++
		return &ec2.DescribeNetworkInterfacesOutput{NetworkInterfaces: []ec2types.NetworkInterface{}}, nil
	}
	clients.wait = func(context.Context, time.Duration) error {
		waitCalls++
		return nil
	}

	_, err := testUpdater(clients).waitForPublicIP(t.Context(), "eni-123")
	if err == nil || !strings.Contains(err.Error(), "after 10 attempts") {
		t.Fatalf("waitForPublicIP() error = %v, want exhausted-retry error", err)
	}
	if describeCalls != maxPublicIPAttempts || waitCalls != maxPublicIPAttempts-1 {
		t.Errorf(
			"calls = describe %d, wait %d; want %d and %d",
			describeCalls,
			waitCalls,
			maxPublicIPAttempts,
			maxPublicIPAttempts-1,
		)
	}
}

func TestUpdaterWaitForPublicIPRejectsInvalidIPv4(t *testing.T) {
	t.Parallel()

	for _, address := range []string{"2001:db8::1", "::ffff:192.0.2.1", "not-an-address"} {
		t.Run(address, func(t *testing.T) {
			t.Parallel()

			clients := successfulClients()
			clients.ec2.describeNetworkInterfaces = func(
				context.Context,
				*ec2.DescribeNetworkInterfacesInput,
			) (*ec2.DescribeNetworkInterfacesOutput, error) {
				return networkInterfaceOutput(address), nil
			}

			_, err := testUpdater(clients).waitForPublicIP(t.Context(), "eni-123")
			if err == nil || !strings.Contains(err.Error(), "not ipv4") {
				t.Fatalf("waitForPublicIP() error = %v, want invalid-ipv4 error", err)
			}
		})
	}
}

func TestUpdaterWaitForPublicIPReturnsCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	clients := successfulClients()
	clients.ec2.describeNetworkInterfaces = func(
		context.Context,
		*ec2.DescribeNetworkInterfacesInput,
	) (*ec2.DescribeNetworkInterfacesOutput, error) {
		return &ec2.DescribeNetworkInterfacesOutput{NetworkInterfaces: []ec2types.NetworkInterface{}}, nil
	}
	clients.wait = func(context.Context, time.Duration) error {
		cancel()
		return ctx.Err()
	}

	_, err := testUpdater(clients).waitForPublicIP(ctx, "eni-123")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("waitForPublicIP() error = %v, want context cancellation", err)
	}
}

func TestUpdaterHandleDoesNotChangeMatchingRecord(t *testing.T) {
	t.Parallel()

	clients := successfulClients()
	clients.route53.listResourceRecordSets = func(
		ctx context.Context,
		input *route53.ListResourceRecordSetsInput,
	) (*route53.ListResourceRecordSetsOutput, error) {
		if ctx != t.Context() {
			t.Error("ListResourceRecordSets() did not receive the invocation context")
		}
		assertRecordListInput(t, input)
		return recordOutput("ts.diogo-nb.com.br.", "203.0.113.10"), nil
	}
	clients.route53.changeResourceRecordSets = func(
		context.Context,
		*route53.ChangeResourceRecordSetsInput,
	) (*route53.ChangeResourceRecordSetsOutput, error) {
		t.Fatal("ChangeResourceRecordSets() called for an unchanged record")
		return nil, nil
	}

	if err := testUpdater(clients).handle(t.Context()); err != nil {
		t.Fatalf("handle() error = %v", err)
	}
}

func TestUpdaterHandleUpsertsSingletonRecordWhenRequired(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		record *route53.ListResourceRecordSetsOutput
	}{
		{name: "record absent", record: &route53.ListResourceRecordSetsOutput{}},
		{name: "record differs", record: recordOutput("ts.diogo-nb.com.br.", "203.0.113.9")},
		{
			name: "record has multiple values",
			record: &route53.ListResourceRecordSetsOutput{
				ResourceRecordSets: []route53types.ResourceRecordSet{
					{
						Name: stringPointer("ts.diogo-nb.com.br."),
						Type: route53types.RRTypeA,
						ResourceRecords: []route53types.ResourceRecord{
							{Value: stringPointer("203.0.113.9")},
							{Value: stringPointer("203.0.113.10")},
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			clients := successfulClients()
			clients.route53.listResourceRecordSets = func(
				context.Context,
				*route53.ListResourceRecordSetsInput,
			) (*route53.ListResourceRecordSetsOutput, error) {
				return test.record, nil
			}
			changeCalls := 0
			clients.route53.changeResourceRecordSets = func(
				ctx context.Context,
				input *route53.ChangeResourceRecordSetsInput,
			) (*route53.ChangeResourceRecordSetsOutput, error) {
				changeCalls++
				if ctx != t.Context() {
					t.Error("ChangeResourceRecordSets() did not receive the invocation context")
				}
				assertChangeInput(t, input)
				return &route53.ChangeResourceRecordSetsOutput{}, nil
			}

			if err := testUpdater(clients).handle(t.Context()); err != nil {
				t.Fatalf("handle() error = %v", err)
			}
			if changeCalls != 1 {
				t.Errorf("ChangeResourceRecordSets() calls = %d, want 1", changeCalls)
			}
		})
	}
}

type fakeECSClient struct {
	listTasks     func(context.Context, *ecs.ListTasksInput) (*ecs.ListTasksOutput, error)
	describeTasks func(context.Context, *ecs.DescribeTasksInput) (*ecs.DescribeTasksOutput, error)
}

func (f *fakeECSClient) ListTasks(
	ctx context.Context,
	input *ecs.ListTasksInput,
	_ ...func(*ecs.Options),
) (*ecs.ListTasksOutput, error) {
	return f.listTasks(ctx, input)
}

func (f *fakeECSClient) DescribeTasks(
	ctx context.Context,
	input *ecs.DescribeTasksInput,
	_ ...func(*ecs.Options),
) (*ecs.DescribeTasksOutput, error) {
	return f.describeTasks(ctx, input)
}

type fakeEC2Client struct {
	describeNetworkInterfaces func(
		context.Context,
		*ec2.DescribeNetworkInterfacesInput,
	) (*ec2.DescribeNetworkInterfacesOutput, error)
}

func (f *fakeEC2Client) DescribeNetworkInterfaces(
	ctx context.Context,
	input *ec2.DescribeNetworkInterfacesInput,
	_ ...func(*ec2.Options),
) (*ec2.DescribeNetworkInterfacesOutput, error) {
	return f.describeNetworkInterfaces(ctx, input)
}

type fakeRoute53Client struct {
	listResourceRecordSets func(
		context.Context,
		*route53.ListResourceRecordSetsInput,
	) (*route53.ListResourceRecordSetsOutput, error)
	changeResourceRecordSets func(
		context.Context,
		*route53.ChangeResourceRecordSetsInput,
	) (*route53.ChangeResourceRecordSetsOutput, error)
}

func (f *fakeRoute53Client) ListResourceRecordSets(
	ctx context.Context,
	input *route53.ListResourceRecordSetsInput,
	_ ...func(*route53.Options),
) (*route53.ListResourceRecordSetsOutput, error) {
	return f.listResourceRecordSets(ctx, input)
}

func (f *fakeRoute53Client) ChangeResourceRecordSets(
	ctx context.Context,
	input *route53.ChangeResourceRecordSetsInput,
	_ ...func(*route53.Options),
) (*route53.ChangeResourceRecordSetsOutput, error) {
	return f.changeResourceRecordSets(ctx, input)
}

type fakeClients struct {
	ecs     *fakeECSClient
	ec2     *fakeEC2Client
	route53 *fakeRoute53Client
	wait    waitFunc
}

func successfulClients() fakeClients {
	return fakeClients{
		ecs: &fakeECSClient{
			listTasks: func(context.Context, *ecs.ListTasksInput) (*ecs.ListTasksOutput, error) {
				return &ecs.ListTasksOutput{TaskArns: []string{"task/one"}}, nil
			},
			describeTasks: func(context.Context, *ecs.DescribeTasksInput) (*ecs.DescribeTasksOutput, error) {
				return &ecs.DescribeTasksOutput{Tasks: []ecstypes.Task{runningTask()}}, nil
			},
		},
		ec2: &fakeEC2Client{
			describeNetworkInterfaces: func(
				context.Context,
				*ec2.DescribeNetworkInterfacesInput,
			) (*ec2.DescribeNetworkInterfacesOutput, error) {
				return networkInterfaceOutput("203.0.113.10"), nil
			},
		},
		route53: &fakeRoute53Client{
			listResourceRecordSets: func(
				context.Context,
				*route53.ListResourceRecordSetsInput,
			) (*route53.ListResourceRecordSetsOutput, error) {
				return recordOutput("ts.diogo-nb.com.br.", "203.0.113.9"), nil
			},
			changeResourceRecordSets: func(
				context.Context,
				*route53.ChangeResourceRecordSetsInput,
			) (*route53.ChangeResourceRecordSetsOutput, error) {
				return &route53.ChangeResourceRecordSetsOutput{}, nil
			},
		},
		wait: func(context.Context, time.Duration) error { return nil },
	}
}

func testUpdater(clients fakeClients) *updater {
	return newUpdater(testConfig(), updaterDependencies{
		ecs:     clients.ecs,
		ec2:     clients.ec2,
		route53: clients.route53,
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		wait:    clients.wait,
	})
}

func testConfig() updaterConfig {
	return updaterConfig{
		clusterARN:    "arn:aws:ecs:sa-east-1:123456789012:cluster/personal-platform-teamspeak6",
		serviceName:   "teamspeak6",
		hostedZoneID:  "Z1234567890",
		dnsName:       "ts.diogo-nb.com.br",
		dnsTTLSeconds: 60,
	}
}

func runningTask() ecstypes.Task {
	return ecstypes.Task{
		LastStatus: stringPointer("RUNNING"),
		Attachments: []ecstypes.Attachment{
			{
				Type: stringPointer("ElasticNetworkInterface"),
				Details: []ecstypes.KeyValuePair{
					{Name: stringPointer("networkInterfaceId"), Value: stringPointer("eni-123")},
				},
			},
		},
	}
}

func networkInterfaceOutput(address string) *ec2.DescribeNetworkInterfacesOutput {
	return &ec2.DescribeNetworkInterfacesOutput{
		NetworkInterfaces: []ec2types.NetworkInterface{
			{
				Association: &ec2types.NetworkInterfaceAssociation{
					PublicIp: stringPointer(address),
				},
			},
		},
	}
}

func recordOutput(name string, addresses ...string) *route53.ListResourceRecordSetsOutput {
	records := make([]route53types.ResourceRecord, 0, len(addresses))
	for _, address := range addresses {
		records = append(records, route53types.ResourceRecord{Value: stringPointer(address)})
	}

	return &route53.ListResourceRecordSetsOutput{
		ResourceRecordSets: []route53types.ResourceRecordSet{
			{
				Name:            stringPointer(name),
				ResourceRecords: records,
				Type:            route53types.RRTypeA,
			},
		},
	}
}

func assertRecordListInput(t *testing.T, input *route53.ListResourceRecordSetsInput) {
	t.Helper()

	config := testConfig()
	if input.HostedZoneId == nil || *input.HostedZoneId != config.hostedZoneID {
		t.Errorf("hosted zone id = %v", input.HostedZoneId)
	}
	if input.StartRecordName == nil || *input.StartRecordName != config.dnsName {
		t.Errorf("start record name = %v", input.StartRecordName)
	}
	if input.StartRecordType != route53types.RRTypeA {
		t.Errorf("start record type = %q, want A", input.StartRecordType)
	}
	if input.MaxItems == nil || *input.MaxItems != 1 {
		t.Errorf("max items = %v, want 1", input.MaxItems)
	}
}

func assertChangeInput(t *testing.T, input *route53.ChangeResourceRecordSetsInput) {
	t.Helper()

	config := testConfig()
	if input.HostedZoneId == nil || *input.HostedZoneId != config.hostedZoneID {
		t.Errorf("hosted zone id = %v", input.HostedZoneId)
	}
	if input.ChangeBatch == nil || len(input.ChangeBatch.Changes) != 1 {
		t.Fatalf("change batch = %#v, want one change", input.ChangeBatch)
	}
	change := input.ChangeBatch.Changes[0]
	if change.Action != route53types.ChangeActionUpsert {
		t.Errorf("change action = %q, want UPSERT", change.Action)
	}
	if change.ResourceRecordSet == nil {
		t.Fatal("resource record set = nil")
	}
	record := change.ResourceRecordSet
	if record.Name == nil || *record.Name != config.dnsName {
		t.Errorf("record name = %v", record.Name)
	}
	if record.Type != route53types.RRTypeA {
		t.Errorf("record type = %q, want A", record.Type)
	}
	if record.TTL == nil || *record.TTL != config.dnsTTLSeconds {
		t.Errorf("record ttl = %v, want %d", record.TTL, config.dnsTTLSeconds)
	}
	if len(record.ResourceRecords) != 1 || record.ResourceRecords[0].Value == nil ||
		*record.ResourceRecords[0].Value != "203.0.113.10" {
		t.Errorf("record values = %#v, want singleton current address", record.ResourceRecords)
	}
}
