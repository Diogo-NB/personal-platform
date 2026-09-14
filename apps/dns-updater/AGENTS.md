# DNS updater application guidance

## Source of truth

Read `README.md` completely before reviewing, changing, deploying, or operating
the DNS updater.

## Fixed decisions

- Keep this application independent from any individual service directory.
- Use the same container image for service-specific deployments. Configuration
  remains the `CLUSTER_ARN`, `SERVICE_NAME`, `HOSTED_ZONE_ID`, `DNS_NAME`, and
  `DNS_TTL` environment-variable contract.
- Trigger updates from matching ECS task `RUNNING` events through EventBridge.
  A scheduled reconciliation trigger may supplement, but not replace, the
  event-driven path.
- Require exactly one running ECS service task before changing DNS. Never clear
  a record merely because its service is stopped.
- Build and deploy Linux/x86-64 only with Go 1.27.1, `lambda.norpc`, and the AWS
  Lambda `provided.al2023` base image.
- Do not log events, credentials, public addresses, or secret-bearing data.

## AWS safety

Agents may run local tests, builds, Docker validation, and offline CDK
synthesis. Only the user runs commands that contact AWS, including CDK diff,
deploy, destroy, resource queries, or service scaling.

## Required verification

Run from `apps/dns-updater`:

```bash
go mod verify
go test -race ./...
go vet ./...
go build ./...
govulncheck ./...
docker build --platform linux/amd64 .
```
