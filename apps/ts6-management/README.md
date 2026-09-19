# TeamSpeak 6 management API

This Go service provides the start, stop, and status control plane for the
singleton TeamSpeak ECS service. It runs as a Linux/x86-64 Lambda container
invoked by an API Gateway REST API and a FIFO SQS command queue.

## API

API Gateway requires `x-api-key` on every public method:

- `POST /start` requests desired count `1` only when it is currently `0`.
- `POST /stop` requests desired count `0`; repeated requests are no-ops.
- `GET /status` returns `stopped`, `starting`, `running`, or `stopping`.

Successful start and stop requests return `202 Accepted` with an empty body.
Only a running service includes `startedAt` and `uptimeSeconds`; the other
states return both fields as `null`. The internal `GET /internal/readiness`
route is used by AWS Lambda Web Adapter and is not exposed by API Gateway.

The API key is lightweight access control, not user authentication or
authorization. Keep the key private and rotate it after suspected disclosure.
AWS recommends stronger authorization for sensitive APIs.

## SQS commands

The FIFO command queue invokes the management Lambda directly through a Lambda
event-source mapping. SQS does not send an HTTP request and does not pass
through API Gateway. AWS Lambda Web Adapter forwards the native event inside
the Lambda execution environment as `POST /internal/events`; that route is not
an API Gateway resource or a public endpoint.

Each message body must be exactly one of these JSON objects:

```json
{"action":"start"}
```

```json
{"action":"stop"}
```

The `action` field and its values are case-sensitive. Malformed JSON, a missing
or non-string action, trailing JSON values, and unknown actions fail the
message. Additional fields are ignored for forward compatibility. Every sender
must use the FIFO message-group ID `teamspeak6` so all
lifecycle commands share one ordered stream. Content-based deduplication is
enabled, so a sender does not need a message-deduplication ID unless it needs
deduplication semantics different from the exact message body. To express a
new repeated command within SQS's five-minute deduplication interval, provide a
new explicit message-deduplication ID.

The stack outputs `ManagementCommandQueueURL`, `ManagementCommandQueueARN`,
and `ManagementCommandDLQURL`. After deployment, a user can submit commands:

```bash
aws sqs send-message \
  --region sa-east-1 \
  --queue-url '<ManagementCommandQueueURL>' \
  --message-group-id teamspeak6 \
  --message-body '{"action":"start"}'
```

Use `{"action":"stop"}` to stop the service. Do not put credentials or other
metadata in a command body.

The event-source batch size is one, which preserves command order and isolates
failures. A failed command becomes visible again after the 90-second visibility
timeout. After five receives, SQS moves it to the encrypted FIFO dead-letter
queue, where it is retained for 14 days. The adapter returns
`batchItemFailures`, so only unsuccessful records are retried. Delivery remains
at least once; duplicate start and stop deliveries are safe because both
operations reconcile the current ECS desired count.

## Architecture and configuration

The service uses domain-driven and hexagonal boundaries. Pure lifecycle types
and singleton rules live in `internal/domain/lifecycle`. The use cases in
`internal/application` implement the driving contract in `internal/port/in`
and depend on the scheduler and clock contracts in `internal/port/out`.
`internal/adapter/in/http` translates public Gin requests and responses, and
`internal/adapter/in/sqs` translates native SQS events delivered through the
Web Adapter pass-through route. The configuration, ECS, and system-clock
adapters live under `internal/adapter/out`. Dependencies point toward the
domain, and `cmd/ts6-management` remains the manual composition root.

The process listens on port `8080` and requires:

| Variable | Purpose |
|---|---|
| `CLUSTER_ARN` | ECS cluster containing the TeamSpeak service. |
| `SERVICE_NAME` | ECS service to control. |

The Docker image pins Go 1.27.1, Gin 1.12, AWS Lambda Web Adapter v1.0.1, the
AWS `provided.al2023` base, and a static Linux/x86-64 binary.

## Safety behavior

The service rejects missing services, malformed ECS responses, desired counts
above one, and running/pending singleton violations. The stop operation is the
one exception: it can reduce an unexpectedly oversized desired count to zero.
Concurrent conflicting writes use last-accepted-write semantics. FIFO ordering
applies only to SQS commands in the `teamspeak6` message group; HTTP requests
can interleave with them under the same last-accepted-write behavior. The DLQ
is operational evidence, not durable lifecycle history.

The two encrypted queues follow the stack lifecycle. Their low request and
storage volume add no material amount at this application's estimate
precision when the shared SQS free tier is available. SQS usage is still
metered, so recalculate from the live AWS pricing page if other workloads use
that allowance.

The API never waits for ECS stability. Deploy the CDK stack only while the ECS
service is already running at desired count `1`; the stack template declares
that desired count and a deployment while intentionally stopped can restart
TeamSpeak. Never run a second task because the server uses singleton SQLite.

## Local validation

```bash
go mod verify
go test -race ./...
go vet ./...
go build ./...
govulncheck ./...
docker build --platform linux/amd64 .
```

Agents must not contact AWS. Deployment and API-key retrieval are private user
operations documented in [`../ts6/README.md`](../ts6/README.md).
