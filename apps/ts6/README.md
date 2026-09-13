# TeamSpeak 6 Server beta

This directory is the source of truth for the personal TeamSpeak 6 Server beta
application and its AWS deployment. The server is intended for four
simultaneous users in AWS São Paulo (`sa-east-1`). It uses the pinned
`teamspeaksystems/teamspeak6-server:6.0.0-beta12.1` image, embedded SQLite, one
ECS Fargate task, and retained EFS One Zone storage.

The `production` tag identifies a long-lived personal environment. This design
does not provide production-grade availability or recoverability: it has one
Availability Zone, no stable public address, no backup, and no database
replica.

## Architecture

```text
Internet
  |
  |-- 9987/UDP (voice) -----------+
  `-- 30033/TCP (file transfer) --+--> public Fargate task
                                         TeamSpeak 6 beta
                                         Linux/x86-64
                                              |
                                              | TLS + IAM access point
                                              v
                                      encrypted EFS One Zone
                                      /var/tsserver
```

The CDK stack is `PersonalPlatformTeamspeak6Stack`. It creates:

- A `10.42.0.0/24` VPC, one public `/26` subnet in `sa-east-1a`, an Internet
  Gateway, and no NAT Gateway.
- An ECS cluster named `personal-platform-teamspeak6` and a service named
  `teamspeak6`.
- One Linux/x86-64 Fargate task using `0.25 vCPU`, `1 GiB` of memory, and an
  automatically assigned public IPv4 address.
- An encrypted EFS One Zone file system and an access point enforcing UID/GID
  `9987`, mounted read-write at `/var/tsserver` with TLS and IAM authorization.
- A CloudWatch log group named `/personal-platform/teamspeak6` with seven-day
  retention.

Every taggable resource uses `Project=personal-storage`,
`Application=teamspeak6`, and `Environment=production`. The ECS service
propagates these tags to each Fargate task so compute can be grouped by
application in AWS billing reports.

There is no load balancer, DNS, Elastic IP, NAT Gateway, ECS Exec, SSH, RDS,
external MariaDB, or public query interface. The security group admits only
`9987/UDP` and `30033/TCP` from IPv4 clients. Query SSH (`10022`), HTTP
(`10080`), and HTTPS (`10443`) are disabled in `tsserver.yaml` and have no
ingress rules.

## Application image and local use

`Dockerfile` extends the pinned upstream image and copies the committed
`tsserver.yaml` to `/opt/teamspeak6-config`. Keeping the configuration outside
`/var/tsserver` prevents the persistent volume from hiding it. TeamSpeak state,
logs, crash dumps, SQLite files, and file-transfer data remain under
`/var/tsserver`.

The pinned tag is multi-architecture. Both Compose and CDK explicitly select
`linux/amd64`; the Fargate task definition also declares x86-64. Never replace
the tag with `latest`.

Read the license bundled with the exact image. Compose configures
`TSSERVER_LICENSE_ACCEPTED=accept` automatically:

```bash
cd apps/ts6
docker compose config
docker compose build --pull
docker compose up -d
docker compose logs --follow --tail=100 teamspeak6
```

Stop the local server without deleting the named volume:

```bash
docker compose stop
docker compose down
```

Never run `docker compose down -v` unless permanent deletion of local
TeamSpeak data is intended.

## Persistence and deployment safety

The ECS service starts with desired count `1`. Its deployment limits are
`minimumHealthyPercent=0` and `maximumPercent=100`, so an update stops the old
task before starting a replacement. The container receives a 120-second stop
timeout, and ECS deployment rollback is enabled. These settings prevent two
tasks from intentionally writing the same SQLite database at once; they also
cause downtime during every replacement.

SQLite warns against placing a database on a network filesystem. This design
explicitly accepts the risks described in [SQLite's network-filesystem
guidance](https://www.sqlite.org/useovernet.html) in exchange for storage that
survives Fargate task replacement. Do not run a second task, manually launch a
copy of the task definition, or increase desired count above `1`.

EFS preserves only data TeamSpeak writes below `/var/tsserver`. It cannot make
an application feature persistent when the beta server does not store that
feature there. In particular, validate chat-history behavior after deployment.

The EFS file system has CloudFormation `Retain` and `UpdateReplacePolicy:
Retain`. It continues billing while the ECS service is stopped and after stack
deletion until an operator explicitly deletes it. EFS One Zone is not a backup
and does not protect against Availability Zone loss, data corruption, or
operator deletion.

## Local CDK validation

From `infra/aws`:

```bash
go test ./...
go vet ./...
go build ./...
cdk synth PersonalPlatformStorageStack --no-lookups
cdk synth PersonalPlatformTeamspeak6Stack --no-lookups
```

The stack always configures `TSSERVER_LICENSE_ACCEPTED=accept` in the ECS task;
there is no CDK context flag, administrator CIDR, or EC2 key-pair input. Docker
must be available to the environment that deploys because CDK builds the image
before publishing it to the bootstrapped CDK ECR asset repository.

Only the user runs commands that contact AWS. Agents must not bootstrap, diff,
deploy, scale, or query AWS resources.

## Deployment workflow

Before any AWS change, verify identity and Region, inventory existing manual
TeamSpeak resources, and review cost and blast radius:

```bash
aws sts get-caller-identity
aws configure get region
```

Confirm the intended account and `sa-east-1`. If that account and Region have
not been bootstrapped, review and run:

```bash
cdk bootstrap aws://<ACCOUNT_ID>/sa-east-1
```

Review the proposed change:

```bash
cd infra/aws
cdk diff PersonalPlatformTeamspeak6Stack
```

Deploy only while the ECS service currently has desired count `1` and is
running. A CDK deployment reconciles the template's desired count back to `1`,
so deploying while the service is intentionally stopped would unexpectedly
start it.

After explicit approval, the user deploys:

```bash
cdk deploy PersonalPlatformTeamspeak6Stack
```

The Docker asset is built locally for `linux/amd64` and published through the
CDK bootstrap ECR asset repository. The application does not create a separate
named ECR repository.

## Retrieve the current address

The public IPv4 belongs to the current Fargate task and changes after task
replacement or a scale-to-zero/start cycle. It is deliberately not a
CloudFormation output. Retrieve it from the current task:

```bash
TASK_ARN=$(aws ecs list-tasks \
  --region sa-east-1 \
  --cluster personal-platform-teamspeak6 \
  --service-name teamspeak6 \
  --desired-status RUNNING \
  --query 'taskArns[0]' \
  --output text)

ENI_ID=$(aws ecs describe-tasks \
  --region sa-east-1 \
  --cluster personal-platform-teamspeak6 \
  --tasks "$TASK_ARN" \
  --query 'tasks[0].attachments[0].details[?name==`networkInterfaceId`].value | [0]' \
  --output text)

aws ec2 describe-network-interfaces \
  --region sa-east-1 \
  --network-interface-ids "$ENI_ID" \
  --query 'NetworkInterfaces[0].Association.PublicIp' \
  --output text
```

Do not cache or publish the result as a permanent endpoint. Share the current
address only with intended users.

## First deployment acceptance

After deployment, the user verifies:

1. ECS reports exactly one healthy running task.
2. The task has a public IPv4, and TeamSpeak accepts voice connections on UDP
   9987.
3. File transfer works on TCP 30033 only for groups allowed by TeamSpeak
   permissions and quotas.
4. The initial administrative privilege key is retrieved privately from
   CloudWatch Logs, stored in a password manager, and claimed immediately.
5. A strong server password and conservative group permissions are configured
   outside IaC.
6. Four users can speak with acceptable latency, jitter, packet loss, and
   memory/CPU use.
7. Channels, permissions, files, and any chat history TeamSpeak itself persists
   survive a task replacement and a scale-to-zero/start cycle.

Startup logs can contain the privilege key or other credentials. Treat the
entire log group as secret-bearing, never paste unsanitized logs into issues or
prompts, and remember that seven-day retention limits operational history.

## Start and stop

Stop the service after informing connected users:

```bash
aws ecs update-service \
  --region sa-east-1 \
  --cluster personal-platform-teamspeak6 \
  --service teamspeak6 \
  --desired-count 0

aws ecs wait services-stable \
  --region sa-east-1 \
  --cluster personal-platform-teamspeak6 \
  --services teamspeak6
```

Start it again:

```bash
aws ecs update-service \
  --region sa-east-1 \
  --cluster personal-platform-teamspeak6 \
  --service teamspeak6 \
  --desired-count 1

aws ecs wait services-stable \
  --region sa-east-1 \
  --cluster personal-platform-teamspeak6 \
  --services teamspeak6
```

Retrieve and share the new public IPv4 after every start. Never scale above
`1`. Fargate compute and public-IPv4 charges stop at desired count `0`; EFS,
ECR image storage, and retained CloudWatch log storage continue charging.

Future start/stop Lambdas and CI/CD automation are deferred. Their service
control contract is limited to changing desired count between `0` and `1`.

## Logs and troubleshooting

View recent startup or shutdown logs privately:

```bash
aws logs tail /personal-platform/teamspeak6 \
  --region sa-east-1 \
  --since 30m
```

If users cannot connect, check the service desired count and running task,
retrieve the current public IPv4, inspect stopped-task reasons and CloudWatch
logs, then confirm the subnet route and `9987/UDP` security-group rule. For file
transfer, also verify `30033/TCP`, TeamSpeak permissions and quotas, and free
EFS capacity.

If state appears missing, scale to zero before investigation. Verify that the
task definition mounts the expected file-system and access-point IDs at
`/var/tsserver`; do not start an ad hoc second writer. There is no point-in-time
restore or snapshot in this design.

## Upgrades

TeamSpeak beta upgrades can change configuration and database behavior:

1. Read the official release notes, configuration reference, and bundled
   license for the proposed version.
2. Record the working image tag and explicitly accept that no backup or
   database rollback exists.
3. Change the pinned tag in `Dockerfile`; never use a floating tag.
4. Validate Compose and build for `linux/amd64` locally.
5. Confirm the ECS service is running at desired count `1` before reviewing and
   deploying the CDK change.
6. Let the `0/100` deployment replace the task without overlapping SQLite
   writers.
7. Validate connectivity, state, permissions, file transfer, chat retention,
   logs, and resource use.

Restoring the old image tag may not reverse database migrations. Add and test a
backup strategy before an upgrade where state loss is unacceptable.

## Cost model

Estimate date: 2026-09-13. Prices are public on-demand São Paulo rates before
credits, support, and tax:

- Fargate Linux/x86: USD 0.0696 per vCPU-hour plus USD 0.0076 per GB-hour. The
  selected `0.25 vCPU / 1 GiB` task is USD 0.0250 per running hour.
- One in-use public IPv4: USD 0.005 per running hour.
- EFS One Zone: USD 0.304 per GB-month. The estimate assumes 1 GiB average
  stored; actual storage is elastic. Bursting throughput is selected, and same-
  AZ access has no data-transfer charge.
- Private ECR storage: USD 0.10 per GB-month. The estimate allows 0.1 GiB for
  the current compressed image asset; old assets can increase this.
- CloudWatch Logs: USD 0.90 per GB ingested and USD 0.0408 per GB-month stored.
  The estimate allows 0.1 GiB ingested per month and seven-day retention.
- Internet data transfer out: the first 100 GB per month is free in aggregate
  across eligible AWS services and Regions; São Paulo's first paid tier is USD
  0.15 per GB. The scenarios assume usage remains inside the shared free tier.
- PTAX reference: BRL 5.0918/USD on 2026-09-11. Estimated Brazilian taxes are
  12.15%, using `USD × 5.0918 × 1.1215`. AWS invoice conversion and taxes can
  differ.

| Scenario | Fargate + IPv4 | Persistent EFS/ECR + logs | USD before tax | Estimated BRL after tax |
|---|---:|---:|---:|---:|
| One 4-hour session in a month | 0.1200 | 0.4049 | 0.5249 | R$3.00 |
| 4 hours/day for 30 days | 3.6000 | 0.4049 | 4.0049 | R$22.87 |
| Continuously running for 730 hours | 21.9000 | 0.4049 | 22.3049 | R$127.37 |

The persistent subtotal is `1 GiB EFS + 0.1 GiB ECR + 0.1 GiB log ingestion +
approximately 0.023 GiB-month retained log storage`. It is a planning example,
not a spending cap. File transfers, larger EFS state, accumulated CDK assets,
manual resources, and usage of the shared free egress allowance can dominate
the bill.

Official references:

- [TeamSpeak 6 server repository](https://github.com/teamspeak/teamspeak6-server)
- [TeamSpeak 6 configuration reference](https://github.com/teamspeak/teamspeak6-server/blob/main/CONFIG.md)
- [Pinned Docker image tag](https://hub.docker.com/r/teamspeaksystems/teamspeak6-server/tags)
- [AWS Fargate pricing](https://aws.amazon.com/fargate/pricing/)
- [Amazon EFS pricing](https://aws.amazon.com/efs/pricing/)
- [Amazon ECR pricing](https://aws.amazon.com/ecr/pricing/)
- [Amazon CloudWatch pricing](https://aws.amazon.com/cloudwatch/pricing/)
- [Public IPv4 pricing](https://aws.amazon.com/vpc/pricing/)
- [EC2 internet data-transfer pricing](https://aws.amazon.com/ec2/pricing/on-demand/#Data_Transfer)
- [AWS Brazil taxes](https://aws.amazon.com/tax-help/Brazil/)
- [Banco Central PTAX](https://ptax.bcb.gov.br/ptax_internet/consultarTodasAsMoedas.do?method=consultaTodasMoedas)

Recalculate and date this model whenever rates, exchange rates, taxes, runtime,
stored data, image size, logging, or transfer assumptions change.

## Deferred work and recovery limits

Deferred work includes start/stop Lambdas, CI/CD, a stable address or DNS,
backups and tested restore, monitoring alarms, and automated budget controls.
None is implied by the current stack.

Retained EFS survives service stops, task replacements, and stack deletion, but
recovery is manual and the data has no backup. Existing manually created
TeamSpeak resources are not adopted or deleted by this stack. Retire them only
after the new deployment passes acceptance, after exact resource IDs and data
impact are reviewed, and with separate authorization for each destructive
operation.
