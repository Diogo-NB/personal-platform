# TeamSpeak 6 management API

This Go service provides the start, stop, and status control plane for the
singleton TeamSpeak ECS service. It runs as a Linux/x86-64 Lambda container
behind an API Gateway REST API.

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

## Architecture and configuration

The service uses domain-driven and hexagonal boundaries. Pure lifecycle types
and singleton rules live in `internal/domain/lifecycle`. The use cases in
`internal/application` implement the driving contract in `internal/port/in`
and depend on the scheduler and clock contracts in `internal/port/out`.
`internal/adapter/in/http` translates Gin requests and responses, while the
configuration, ECS, and system-clock adapters live under
`internal/adapter/out`. Dependencies point toward the domain, and
`cmd/ts6-management` remains the manual composition root.

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
Concurrent conflicting writes use last-accepted-write semantics.

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
