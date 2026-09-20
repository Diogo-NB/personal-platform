# TeamSpeak 6 Server beta

This directory is the source of truth for the personal TeamSpeak 6 Server beta
application and its AWS deployment. The server is intended for four
simultaneous users in AWS São Paulo (`sa-east-1`). It uses the pinned
`teamspeaksystems/teamspeak6-server:6.0.0-beta12.1` image, embedded SQLite, one
ECS Fargate Spot task, and retained EFS One Zone storage.

The `production` tag identifies a long-lived personal environment. This design
does not provide production-grade availability or recoverability: it has one
Availability Zone, a stable DNS name backed by a changing public address, no
backup, and no database replica.

## Architecture

```text
Internet
  |
  |  ts.diogo-nb.com.br (Route 53, TTL 60 seconds)
  |
  |-- 9987/UDP (voice) -----------+
  `-- 30033/TCP (file transfer) --+--> public Fargate Spot task
                                         TeamSpeak 6 beta
                                         Linux/x86-64
                                              |
                                              | TLS + IAM access point
                                              v
                                      encrypted EFS One Zone
                                      /var/tsserver

ECS RUNNING events --+
                      +--> Lambda DNS updater --> Route 53 A record
5-minute schedule ----+

Operator -- x-api-key --> API Gateway REST API (prod) --+
                                                         |
Command sender --> encrypted SQS FIFO queue -------------+
                    group: teamspeak6                    |
                                                         v
                                              Lambda management service
                                               | start / stop / status
                                               v
                                            ECS service

Browser --> CloudFront -- OAC --> private S3 web app
   |
   `------ x-api-key -------> API Gateway REST API (prod)
```

The CDK stack is `PersonalPlatformTeamspeak6Stack`. It creates:

- A `10.42.0.0/24` VPC, one public `/26` subnet in `sa-east-1a`, an Internet
  Gateway, and no NAT Gateway.
- An ECS cluster named `personal-platform-teamspeak6` and a service named
  `teamspeak6`.
- One Linux/x86-64 Fargate Spot task using `0.25 vCPU`, `512 MiB` of memory,
  and an automatically assigned public IPv4 address.
- An encrypted EFS One Zone file system and an access point enforcing UID/GID
  `9987`, mounted read-write at `/var/tsserver` with TLS and IAM authorization.
- A CloudWatch log group named `/personal-platform/teamspeak6` with seven-day
  retention.
- A retained Route 53 public hosted zone for `diogo-nb.com.br`, an EventBridge
  rule, and a small Go Lambda container that keeps `ts.diogo-nb.com.br` pointed
  at the current running task.
- A regional API Gateway REST API and 128 MiB Go Lambda container for starting,
  stopping, and reporting the status of the singleton ECS service. All three
  public methods require a generated API key and share a two-request/second,
  burst-five usage plan.
- A private, encrypted, public-access-blocked S3 bucket and CloudFront
  distribution for the management SPA. CloudFront uses Origin Access Control,
  redirects viewers to HTTPS, and serves no API routes.
- An encrypted FIFO lifecycle-command queue and encrypted FIFO dead-letter
  queue. The command queue directly invokes the same management Lambda with a
  batch size of one and partial-batch failure reporting.

Every taggable resource uses `Project=personal-platform`,
`Application=teamspeak6`, and `Environment=production`. A `Component` tag
separates `server`, `management-api`, and `management-web` costs while retaining
the shared TeamSpeak application family. The ECS service propagates the server
tags to each Fargate task.

There is no load balancer, Elastic IP, NAT Gateway, ECS Exec, SSH, RDS,
external MariaDB, or public query interface. DNS changes only how clients find
the task; it does not proxy traffic or make the public IPv4 static. The
security group admits only `9987/UDP` and `30033/TCP` from IPv4 clients. Query
SSH (`10022`), HTTP (`10080`), and HTTPS (`10443`) are disabled in
`tsserver.yaml` and have no ingress rules.

The management API uses the default API Gateway execute-api hostname. It has no
custom domain, Route 53 record, Lambda Function URL, or CloudFront proxy. The
browser client calls that hostname directly. Gin and API Gateway preflight
handlers allow all browser origins for v1 without credentials, and gateway
4xx/5xx responses include the wildcard origin so the SPA can read failures.
An API key is lightweight access control and request metering, not
strong authentication or authorization. AWS recommends stronger authorization
for sensitive APIs; this limitation is accepted for the initial private v1.
The Lambda Web Adapter's private `POST /internal/events` pass-through route is
not an API Gateway resource and cannot be reached through the management URL.

## Application images and local use

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

The DNS updater is the independent, reusable Go 1.27.1 application and Docker
asset under [`../dns-updater`](../dns-updater/README.md). Its multi-stage build
produces a static Linux/x86-64
`bootstrap` executable with the `lambda.norpc` build tag and runs it on the
official AWS Lambda `provided.al2023` base image. Its build context is separate
from the TeamSpeak server context, so updater source or dependency changes do
not replace the ECS task. The current Lambda deployment is configured for
TeamSpeak; future ECS services can deploy the same image with their own scoped
environment, EventBridge filter, and IAM permissions.

The updater reads `CLUSTER_ARN`, `SERVICE_NAME`, `HOSTED_ZONE_ID`, `DNS_NAME`,
and `DNS_TTL` from the Lambda environment. CDK supplies all five values.

The management API is the independent Go 1.27.1 application and Docker asset
under [`../ts6-management-api`](../ts6-management-api/README.md). It uses Gin
1.12 and AWS Lambda Web Adapter v1.0.1 on `provided.al2023`, listens on port
8080, and receives `CLUSTER_ARN` and `SERVICE_NAME` from CDK. Its hexagonal
dependency direction keeps the lifecycle domain and application service
independent of Gin and the AWS SDK. Both the HTTP and SQS inbound adapters call
the same lifecycle application service.

For local interactive use, both management applications provide
`.env.example` files. The API example configures the AWS profile, Region,
cluster ARN, and service name. The web example configures only the local API
URL. Neither file contains or accepts the management API key as persisted
configuration.

`POST /start` changes desired count from zero to one and otherwise does
nothing. `POST /stop` changes a positive desired count to zero, including an
unexpected value above one as a safety action. Both return `202` as soon as ECS
accepts the request and do not wait for stability. `GET /status` maps desired,
running, and pending counts to `stopped`, `starting`, `running`, or `stopping`.
Only `running` includes a UTC start time and whole-second uptime. Missing or
malformed ECS data and singleton violations return a generic `500` response.

The SQS adapter requires an `action` value of `start` or `stop`. Lambda Web
Adapter receives the native SQS invocation and posts it to `/internal/events`
on localhost inside the execution environment; SQS does not call the route over
the network. Failed records are returned through `batchItemFailures` for retry.

The TeamSpeak deployment uses the account's shared regional Lambda concurrency
instead of reserving capacity for the updater. Duplicate EventBridge deliveries
can therefore overlap, but the updater reconciles current ECS state and uses an
idempotent Route 53 `UPSERT`; the five-minute trigger provides eventual repair.

To validate the module and its image locally:

```bash
cd apps/dns-updater
go mod verify
go test -race ./...
go vet ./...
go build ./...
govulncheck ./... # when installed
docker build --platform linux/amd64 .

cd ../ts6-management-api
go mod verify
go test -race ./...
go vet ./...
go build ./...
govulncheck ./... # when installed
docker build --platform linux/amd64 .

cd ../ts6-management-web
npm ci
npm test
npm run build
```

## Persistence and deployment safety

The ECS service starts with desired count `1` and uses only the
`FARGATE_SPOT` capacity provider; it does not fall back to on-demand Fargate.
AWS can interrupt the task with approximately two minutes of warning, and Spot
capacity can be temporarily unavailable. ECS keeps requesting a replacement,
but the singleton server remains offline until a Spot task reaches `RUNNING`.
The existing 120-second container stop timeout gives TeamSpeak the full Spot
warning window to shut down after receiving `SIGTERM`.

The deployment limits are `minimumHealthyPercent=0` and `maximumPercent=100`,
so an update stops the old task before starting a replacement. ECS deployment
rollback is enabled. These settings prevent two tasks from intentionally
writing the same SQLite database at once; they also cause downtime during every
replacement.

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
cd ../../apps/ts6-management-web
npm ci
npm test
npm run build

cd ../../../infra/aws
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
cd apps/ts6-management-web
npm ci
npm test
npm run build

cd ../../../infra/aws
cdk diff PersonalPlatformTeamspeak6Stack
```

The API/web naming migration intentionally replaces the named management
Lambda, log group, API key, FIFO queues, frontend bucket, and CloudFront
distribution. Before deploying it over an existing stack, inspect and drain
the old command queue and DLQ, and export any management logs that must be
retained. The old key and CloudFront URL stop working after replacement; use
the new `ManagementAPIKeyID` and `ManagementWebURL` outputs after deployment.

Deploy only while the ECS service currently has desired count `1` and is
running. A CDK deployment reconciles the template's desired count back to `1`,
so deploying while the service is intentionally stopped would unexpectedly
start it. This warning still applies when the service was stopped through the
management API.

After explicit approval, the user deploys:

```bash
cdk deploy PersonalPlatformTeamspeak6Stack
```

All three Docker assets are built locally for `linux/amd64` and published through
the CDK bootstrap ECR asset repository. The application does not create a
separate named ECR repository.

## Management console, API, and command queue operation

The deployment outputs `ManagementWebURL`, `ManagementAPIURL`, and
`ManagementAPIKeyID`. Open the CloudFront URL and enter the retrieved key to
authenticate the SPA. The key remains only in that tab's memory: it is not put
in browser storage, the URL, logs, or runtime configuration. The SPA polls
every two seconds while visible, displays start timestamps as Brasília time,
and calls API Gateway directly rather than through CloudFront.

The key value is intentionally absent from CloudFormation outputs and source
control.
After deployment, retrieve it privately using the output key ID:

```bash
aws apigateway get-api-key \
  --region sa-east-1 \
  --api-key <ManagementAPIKeyID> \
  --include-value \
  --query value \
  --output text
```

Store the value in a password manager. Do not paste it into logs, issues,
shell history, or committed environment files. The following examples use a
temporary shell variable; enter its value without sharing terminal output:

```bash
export TS6_MANAGEMENT_URL='<ManagementAPIURL>'
read -rs TS6_MANAGEMENT_API_KEY
export TS6_MANAGEMENT_API_KEY

curl --fail-with-body \
  -X POST \
  -H "x-api-key: ${TS6_MANAGEMENT_API_KEY}" \
  "${TS6_MANAGEMENT_URL}start"

curl --fail-with-body \
  -H "x-api-key: ${TS6_MANAGEMENT_API_KEY}" \
  "${TS6_MANAGEMENT_URL}status"

curl --fail-with-body \
  -X POST \
  -H "x-api-key: ${TS6_MANAGEMENT_API_KEY}" \
  "${TS6_MANAGEMENT_URL}stop"

unset TS6_MANAGEMENT_API_KEY
```

The URL output ends in `/prod/`. Missing or invalid keys receive API Gateway's
`403` response. Successful start and stop responses are empty `202` responses.
Conflicting concurrent start and stop requests use last-accepted-write
semantics. The API has no durable lifecycle history.

The deployment also outputs `ManagementAPICommandQueueURL`,
`ManagementAPICommandQueueARN`, and `ManagementAPICommandDLQURL`. The command
queue invokes the management API Lambda directly; it does not call API Gateway
or require the management API key. Every sender must use FIFO message-group ID
`teamspeak6`. For example:

```bash
aws sqs send-message \
  --region sa-east-1 \
  --queue-url '<ManagementAPICommandQueueURL>' \
  --message-group-id teamspeak6 \
  --message-body '{"action":"start"}'

aws sqs send-message \
  --region sa-east-1 \
  --queue-url '<ManagementAPICommandQueueURL>' \
  --message-group-id teamspeak6 \
  --message-body '{"action":"stop"}'
```

The body must contain the case-sensitive `action` field with value `start` or
`stop`. Additional fields are ignored. Content-based deduplication is enabled.
Identical bodies sent within SQS's deduplication interval are accepted as
duplicates but delivered once unless the sender supplies explicit deduplication
IDs with different semantics. To express a new repeated command within the
five-minute interval, provide a new explicit message-deduplication ID.

The Lambda event-source mapping reads one message per invocation to preserve
order and isolate failures. The queue visibility timeout is 90 seconds for the
15-second Lambda timeout. Failed messages are retried and move to the encrypted
FIFO DLQ after five receives; the DLQ retains them for 14 days. Inspect and
redrive DLQ messages deliberately because the DLQ is operational evidence, not
durable lifecycle history.

SQS delivery is at least once. Duplicate start and stop commands are safe
because the lifecycle service reconciles the current desired count. Ordering
applies only inside the `teamspeak6` SQS message group. API requests can
interleave with queue commands under the existing last-accepted-write behavior.

## DNS setup and automatic updates

The client address is `ts.diogo-nb.com.br`. The stack creates a Route 53 public
hosted zone for `diogo-nb.com.br`, but Registro.br remains the domain
registrar. Route 53 becomes authoritative only after this one-time delegation:

1. Deploy the stack and copy the four values from its `NameServers` output.
2. If the domain already has records for a website, email, or another service,
   reproduce them in Route 53 before continuing.
3. Registro.br enables DNSSEC automatically when its own DNS servers are in
   use. Remove the existing DS delegation or disable DNSSEC there before moving
   to the unsigned Route 53 zone. Leaving the old DS record can make validating
   resolvers return `SERVFAIL`.
4. In the Registro.br domain settings, replace its authoritative DNS servers
   with all four Route 53 nameservers.
5. Wait for delegation caches to expire, then verify:

```bash
dig NS diogo-nb.com.br +short
dig A ts.diogo-nb.com.br +short
```

Do not create or update the `A` record manually. The updater runs when the ECS
task reaches `RUNNING` and every five minutes as reconciliation. It finds the
singleton running task, resolves its current public IPv4, and performs a Route
53 `UPSERT` only when the value changed. The record TTL is 60 seconds, so a
replacement can take roughly the task startup time plus DNS-cache expiry to
become reachable everywhere. The periodic run also creates the first record
for a task that was already running when DNS was deployed.

The hosted zone has a retain policy. Deleting the CloudFormation stack does
not delete the zone, its DNS records, or its ongoing hosted-zone charge. If the
zone is ever intentionally replaced, Registro.br must be delegated to the new
four nameservers.

## First deployment acceptance

After deployment, the user verifies:

1. ECS reports exactly one healthy running task.
2. `ts.diogo-nb.com.br` resolves to the task's public IPv4, and TeamSpeak
   accepts voice connections through that name on UDP 9987.
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
8. `ManagementWebURL` loads over HTTPS, accepts the private key, reports
   status, pauses polling in a hidden tab, and successfully starts and stops
   through the direct API Gateway URL.

Startup logs can contain the privilege key or other credentials. Treat the
entire log group as secret-bearing, never paste unsanitized logs into issues or
prompts, and remember that seven-day retention limits operational history.

## Start and stop

Prefer the management API commands above. For break-glass operation, the user
can still call ECS directly after confirming the account, Region, cluster, and
service. Stop the service only after informing connected users:

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

Keep using `ts.diogo-nb.com.br` after every start; the updater replaces the DNS
value automatically. A start can remain pending while Fargate Spot capacity is
unavailable. Never scale above `1`. Fargate compute and public-IPv4 charges stop
at desired count `0`; Route 53, EFS, ECR image storage, and retained CloudWatch
log storage continue charging.

Stronger authentication, exact-origin CORS after a stable custom frontend
domain, durable history, sender roles and automations, and CI/CD automation
remain deferred.

## Logs and troubleshooting

View recent startup or shutdown logs privately:

```bash
aws logs tail /personal-platform/teamspeak6 \
  --region sa-east-1 \
  --since 30m
```

If users cannot connect, check the service desired count and running task, then
inspect stopped-task reasons, pending-task events, and CloudWatch logs. A Spot
interruption or unavailable Spot capacity can leave the desired task pending.
Compare the task's public IPv4 with `dig A ts.diogo-nb.com.br +short`; DNS
updater logs are in `/personal-platform/teamspeak6/dns-updater`. Also confirm
the subnet route and `9987/UDP` security-group rule. For file transfer, verify
`30033/TCP`, TeamSpeak permissions and quotas, and free EFS capacity.

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

Estimate date: 2026-09-19. Regional services use public São Paulo rates;
Route 53 authoritative DNS uses its global rate. Values are before credits,
support, and tax:

- Fargate Spot Linux/x86: USD 0.02199658 per vCPU-hour plus USD 0.00240193 per
  GB-hour. The selected `0.25 vCPU / 0.5 GiB` task is approximately USD
  0.00670011 per running hour. Spot rates vary with long-term supply and demand;
  recalculate from the live AWS price table before relying on this estimate.
- One in-use public IPv4: USD 0.005 per running hour.
- EFS One Zone: USD 0.304 per GB-month. The estimate assumes 1 GiB average
  stored; actual storage is elastic. Bursting throughput is selected, and same-
  AZ access has no data-transfer charge.
- Private ECR storage: USD 0.10 per GB-month. The estimate allows 0.3 GiB for
  the current compressed TeamSpeak, DNS updater, and management image assets;
  old assets can increase this.
- CloudWatch Logs: USD 0.90 per GB ingested and USD 0.0408 per GB-month stored.
  The estimate allows 0.1 GiB ingested per month and seven-day retention.
- Route 53 authoritative DNS: USD 0.50 per hosted zone per month and USD 0.40
  per million standard queries. Four-user query volume is treated as zero at
  this estimate's precision.
- The EventBridge rule invokes the 128 MiB updater Lambda every five minutes
  and on matching ECS task events. Its approximately 8,640 scheduled monthly
  invocations fit within Lambda's monthly 1 million-request and 400,000
  GB-second free tier if those allowances are not consumed elsewhere. Its log
  volume is included in the CloudWatch allowance above.
- The REST management API estimate assumes 300 calls per month. At the first
  REST API tier's USD 3.50 per million requests, that is about USD 0.0011. The
  128 MiB management Lambda's request and duration usage is expected to fit
  within Lambda's shared monthly free tier when that allowance is available.
- The two SSE-SQS encrypted FIFO queues have no minimum fee. Send, receive,
  delete, visibility-change, payload-size, and retained-message usage is
  metered. This low-volume estimate treats the added SQS request and DLQ storage
  cost as zero at its precision when the shared monthly SQS free tier is
  available; recalculate if other workloads consume that allowance or failed
  messages accumulate.
- The management SPA stores only a small static build in S3 and serves it
  through CloudFront. At personal-use volume its S3 storage and request costs
  round to zero at this model's precision, and its CloudFront requests and
  transfer are expected to remain within the account's shared free allowance.
  Recalculate if other workloads consume that allowance.
- Internet data transfer out: the first 100 GB per month is free in aggregate
  across eligible AWS services and Regions; São Paulo's first paid tier is USD
  0.15 per GB. The scenarios assume usage remains inside the shared free tier.
- PTAX reference: BRL 5.0918/USD on 2026-09-11. Estimated Brazilian taxes are
  12.15%, using `USD × 5.0918 × 1.1215`. AWS invoice conversion and taxes can
  differ.

| Scenario | Fargate Spot + IPv4 | Persistent EFS/ECR + logs/DNS | USD before tax | Estimated BRL after tax |
|---|---:|---:|---:|---:|
| One 4-hour session in a month | 0.0468 | 0.9260 | 0.9728 | R$5.56 |
| 4 hours/day for 30 days | 1.4040 | 0.9260 | 2.3300 | R$13.31 |
| Continuously running for 730 hours | 8.5411 | 0.9260 | 9.4671 | R$54.06 |

The non-compute subtotal is `1 GiB EFS + 0.3 GiB ECR + 0.1 GiB log ingestion +
approximately 0.023 GiB-month retained log storage + one Route 53 hosted zone
+ 300 REST API calls`. It is a planning example, not a spending cap. File
transfers, larger EFS state, accumulated CDK assets, manual resources, and usage
of shared free allowances can dominate the bill.

Official references:

- [TeamSpeak 6 server repository](https://github.com/teamspeak/teamspeak6-server)
- [TeamSpeak 6 configuration reference](https://github.com/teamspeak/teamspeak6-server/blob/main/CONFIG.md)
- [Pinned Docker image tag](https://hub.docker.com/r/teamspeaksystems/teamspeak6-server/tags)
- [AWS Fargate pricing](https://aws.amazon.com/fargate/pricing/)
- [AWS Fargate capacity providers and Spot interruptions](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/fargate-capacity-providers.html)
- [Amazon EFS pricing](https://aws.amazon.com/efs/pricing/)
- [Amazon ECR pricing](https://aws.amazon.com/ecr/pricing/)
- [Amazon CloudWatch pricing](https://aws.amazon.com/cloudwatch/pricing/)
- [Amazon Route 53 pricing](https://aws.amazon.com/route53/pricing/)
- [Route 53 DNS service for an existing domain](https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/MigratingDNS.html)
- [Route 53 DNSSEC migration guidance](https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/hosted-zones-migrating.html)
- [Registro.br domain and DNS guidance](https://registro.br/ajuda/registro-de-novos-dominios/)
- [AWS Lambda pricing](https://aws.amazon.com/lambda/pricing/)
- [Amazon API Gateway pricing](https://aws.amazon.com/api-gateway/pricing/)
- [Amazon SQS pricing](https://aws.amazon.com/sqs/pricing/)
- [Amazon S3 pricing](https://aws.amazon.com/s3/pricing/)
- [Amazon CloudFront pricing](https://aws.amazon.com/cloudfront/pricing/)
- [API Gateway usage-plan and API-key guidance](https://docs.aws.amazon.com/apigateway/latest/developerguide/api-gateway-api-usage-plans.html)
- [Amazon EventBridge pricing](https://aws.amazon.com/eventbridge/pricing/)
- [Public IPv4 pricing](https://aws.amazon.com/vpc/pricing/)
- [EC2 internet data-transfer pricing](https://aws.amazon.com/ec2/pricing/on-demand/#Data_Transfer)
- [AWS Brazil taxes](https://aws.amazon.com/tax-help/Brazil/)
- [Banco Central PTAX](https://ptax.bcb.gov.br/ptax_internet/consultarTodasAsMoedas.do?method=consultaTodasMoedas)

Recalculate and date this model whenever rates, exchange rates, taxes, runtime,
stored data, image size, logging, or transfer assumptions change.

## Deferred work and recovery limits

Deferred work includes stronger management authorization, a stable frontend
domain with exact-origin CORS, command sender roles and automations, durable
lifecycle history, CI/CD,
Route 53 DNSSEC signing, backups and tested restore, monitoring alarms, and
automated budget controls. None is implied by the current stack.

Retained EFS survives service stops, task replacements, and stack deletion, but
recovery is manual and the data has no backup. Existing manually created
TeamSpeak resources are not adopted or deleted by this stack. Retire them only
after the new deployment passes acceptance, after exact resource IDs and data
impact are reviewed, and with separate authorization for each destructive
operation.
