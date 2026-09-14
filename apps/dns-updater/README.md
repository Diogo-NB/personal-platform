# DNS updater

This application provides the reusable Lambda container that updates Route 53
`A` records for services running as singleton ECS tasks. The current deployment
serves TeamSpeak 6, but the image contains no TeamSpeak-specific configuration.

## Event flow

```text
ECS task reaches RUNNING
          |
          v
     EventBridge  ---- five-minute reconciliation fallback
          |                         |
          +------------+------------+
                       v
              DNS updater Lambda
                       |
                       v
             Route 53 A-record UPSERT
```

Each deployment supplies `CLUSTER_ARN`, `SERVICE_NAME`, `HOSTED_ZONE_ID`,
`DNS_NAME`, and a positive `DNS_TTL`. The updater lists at most two running
tasks and changes DNS only when exactly one task exists and its public IPv4
differs from the current singleton record. A stopped service leaves its last
record unchanged.

The Lambda reuses AWS SDK clients across invocations. It propagates invocation
cancellation through ECS, EC2, Route 53, and the bounded public-address retry
wait. Normal skips and successful updates use structured logs without logging
the triggering event or public address.

## Build and test

The module targets Go 1.27.1. Its multi-stage Docker build creates a static
Linux/x86-64 `bootstrap` executable with `lambda.norpc`, then copies it into the
official AWS Lambda `provided.al2023` base image.

```bash
cd apps/dns-updater
go mod verify
go test -race ./...
go vet ./...
go build ./...
govulncheck ./... # when installed
docker build --platform linux/amd64 .
```

Consuming infrastructure owns the EventBridge filters, environment values,
least-privilege IAM permissions, log group, and Route 53 hosted zone. Adding a
service means configuring another deployment of this application for that
service; it does not require service-specific source code here.
