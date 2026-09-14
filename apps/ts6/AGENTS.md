# TeamSpeak 6 application and AWS guidance

## Source of truth

Read `README.md` completely before reviewing, changing, deploying, or operating
TeamSpeak. This directory owns the container image, committed configuration,
local Compose workflow, architecture decisions, cost model, and operations
runbook. The CDK implementation is in `../../infra/aws/teamspeak6`.

## Fixed decisions

- Use AWS CDK v2 with Go and the separate
  `PersonalPlatformTeamspeak6Stack` in `sa-east-1`.
- Keep the storage stack in `us-east-1` unchanged.
- Pin `teamspeaksystems/teamspeak6-server:6.0.0-beta12.1`; never use `latest`.
- Build and run Linux/x86-64 only.
- Use one ECS Fargate task at `0.25 vCPU / 1 GiB`, desired count `1` while
  running and `0` while stopped.
- Use embedded SQLite and prevent overlapping writers with ECS deployment
  limits `minimumHealthyPercent=0` and `maximumPercent=100`.
- Persist `/var/tsserver` on encrypted EFS One Zone through an access point
  enforcing UID/GID `9987`, IAM authorization, and encryption in transit.
- Retain EFS on stack deletion. There is no backup or Availability Zone loss
  protection.
- Use a `10.42.0.0/24` VPC with one public `/26` subnet in `sa-east-1a`, an
  Internet Gateway, and no NAT Gateway.
- Give each running task a dynamic public IPv4 and publish it as
  `ts.diogo-nb.com.br`. Do not add an Elastic IP or load balancer.
- Host `diogo-nb.com.br` in a retained Route 53 public hosted zone. Keep the
  `A` record synchronized through the scoped ECS/EventBridge/Lambda updater;
  do not manage the record manually.
- Admit only public `9987/UDP` and `30033/TCP`. Do not expose SSH, ECS Exec, or
  TeamSpeak query ports.
- Send container logs to `/personal-platform/teamspeak6` with seven-day
  retention and treat the logs as potentially secret-bearing.
- Always configure `TSSERVER_LICENSE_ACCEPTED=accept` in the ECS task and local
  Compose service. Do not require a CDK context flag or shell environment
  variable for license acceptance.
- Tag every taggable resource with `Project=personal-storage`,
  `Application=teamspeak6`, and `Environment=production`. Propagate the service
  tags to Fargate tasks.

Do not change these decisions silently. Update the README and obtain user
agreement before a material architecture, persistence, security, recovery, or
cost change.

## AWS safety

Agents may run local tests, builds, Docker validation, and offline CDK
synthesis. Agents must not run AWS CLI commands or CDK commands that contact
AWS, including bootstrap, diff, deploy, destroy, resource queries, or service
scaling. Provide commands for the user to review and run after they confirm the
account, `sa-east-1`, cost, and blast radius.

CDK deployments are allowed only while the ECS service is running at desired
count `1`. The template declares desired count `1`, so a deployment while the
service is intentionally at `0` can start it unexpectedly.

Never scale above `1`, run an ad hoc copy of the task definition, or otherwise
create two SQLite writers. Stop the service before copying, moving, or
investigating persistent state.

## Data and secret safety

- Never run `docker compose down -v` without explicit authorization to destroy
  local data.
- Never delete the EFS file system or access point without an exact ID, impact
  review, and explicit destructive authorization.
- Treat TeamSpeak privilege keys, passwords, query credentials, AWS
  credentials, and startup logs as secrets. Do not commit or paste them into
  prompts, issues, diffs, or documentation.
- EFS is retained but is not a backup. State may be lost through AZ failure,
  corruption, application behavior, or operator deletion.
- EFS preserves only files TeamSpeak writes under `/var/tsserver`; validate
  chat retention rather than assuming it.
- SQLite on EFS accepts the network-filesystem risks documented in the README.

## Change discipline

- Inspect the worktree and preserve unrelated changes.
- Keep reusable examples free of account IDs, credentials, real public
  addresses, and resource IDs. The committed `diogo-nb.com.br` application
  domain is the only allowed real domain.
- Re-check official TeamSpeak notes, image tags, configuration, and license
  before any upgrade.
- Re-check official AWS prices and date the cost model when relevant inputs
  change.
- Keep the TeamSpeak Docker build context limited to `Dockerfile` and
  `tsserver.yaml`.
- Keep the DNS updater in the separate `../dns-updater/` application and Docker
  context so its changes cannot alter the TeamSpeak image asset hash.
- Validate Compose and build the image explicitly for `linux/amd64` when Docker
  is available.
- Update `README.md` whenever architecture, ports, scaling, persistence,
  security, operating procedures, costs, or recovery assumptions change.

## Required verification

For code changes, run from `infra/aws`:

```bash
go test ./...
go vet ./...
go build ./...
cdk synth PersonalPlatformStorageStack --no-lookups
cdk synth PersonalPlatformTeamspeak6Stack --no-lookups
```

For DNS updater changes, also run from `apps/dns-updater`:

```bash
go mod verify
go test -race ./...
go vet ./...
go build ./...
govulncheck ./...
docker build --platform linux/amd64 .
```

When Docker is available, also run from `apps/ts6`:

```bash
docker compose config
docker build --platform linux/amd64 .
```

Template tests must cover automatic license acceptance, image asset, Fargate
runtime and sizing, public IP, allowed ingress, encrypted retained EFS,
access-point identity, TLS/IAM mount, stop-before-start deployment, rollback,
stop timeout, automatic Route 53 updates, tags, outputs, and absence of EC2
instances, EBS, NAT, load balancers, RDS, SSH, and query ingress.
