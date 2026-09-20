# TeamSpeak management API guidance

## Source of truth

Read `README.md` completely before reviewing, changing, deploying, or operating
this application. Also follow `../ts6/AGENTS.md` for TeamSpeak architecture,
deployment safety, data protection, and AWS restrictions.

Always load and follow the `golang-how-to` skill for Go coding, review,
debugging, or project setup in this directory.

## Fixed decisions

- Keep lifecycle policy in `internal/domain/lifecycle`, independent of Gin and
  AWS SDK types. Use cases belong in `internal/application`, contracts in
  `internal/port/in` and `internal/port/out`, and integrations in
  `internal/adapter/in` and `internal/adapter/out`.
- Dependencies point inward: adapters depend on ports and domain types, the
  application depends on domain types and outbound ports, and the domain does
  not depend on application or infrastructure packages.
- Keep `cmd/ts6-management-api/main.go` as the manual composition root so later
  EventBridge handlers can reuse the same lifecycle application service.
- Use manual constructor injection, Gin 1.12, Go 1.27.1, Linux/x86-64,
  `provided.al2023`, and AWS Lambda Web Adapter v1.0.1.
- Expose only `POST /start`, `POST /stop`, and `GET /status` through API
  Gateway. The internal readiness route is for the web adapter only.
- Treat the ECS service as a singleton. Starting must never raise desired count
  above one. Stopping may reduce an unexpectedly oversized service to zero.
- Propagate request contexts into every AWS SDK call.
- Do not log request headers, bodies, API keys, task identifiers, AWS response
  objects, or secret-bearing details. Public failures remain generic.
- Load `.env` only as local configuration. Existing process variables take
  precedence, and `.env` files must remain ignored and outside Docker builds.

## AWS safety

Agents may run local tests, builds, Docker validation, and offline CDK
synthesis. Only the user runs commands that contact AWS, including CDK diff,
deploy, destroy, resource queries, API-key retrieval, or service scaling.

## Required verification

Run from this directory:

```bash
go mod verify
go test -race ./...
go vet ./...
go build ./...
govulncheck ./...
docker build --platform linux/amd64 .
```
