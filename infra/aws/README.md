# Personal AWS infrastructure

This directory contains the AWS CDK v2 application for personal infrastructure.
The application is written in Go and can contain independent stacks in
different AWS Regions.

## Stacks

| Stack | Region | Status | Documentation |
|---|---|---|---|
| `PersonalPlatformDnsStack` | `sa-east-1` | Implemented | [DNS](#dns) |
| `PersonalPlatformTeamspeak6Stack` | `sa-east-1` | Implemented | [TeamSpeak 6](../../apps/ts6/README.md) |

The `skycrate-storage` S3 bucket in `us-east-1` is managed manually, outside
this CDK application. Its former CloudFormation stack was deleted with both
the bucket and its HTTPS-enforcement policy retained. Do not recreate a CDK
stack for this bucket.

The TeamSpeak stack provisions a single public on-demand ECS Fargate task in
`sa-east-1`, backed by an encrypted retained EFS One Zone file system. Its
Docker image is built from `apps/ts6` and published as a CDK ECR asset. A
separate reusable Go Lambda container asset is built from `apps/dns-updater`;
an EventBridge rule invokes the TeamSpeak-configured deployment when its ECS
task reaches `RUNNING`, keeping `ts.diogo-nb.com.br` in the retained Route 53
public hosted zone pointed at the current task's dynamic public IPv4. A third
Go Lambda container asset is built from `apps/ts6-management-api` and exposed
through an API-key-protected Regional API Gateway REST API for singleton start,
stop, and status operations. An ACM certificate and Route 53 alias expose its
`prod` stage at `https://ts-management-api.diogo-nb.com.br/`; the generated
execute-api endpoint remains enabled for rollback. The independent Vite app is
deployed from `apps/ts6-management-web/dist` to a private S3 bucket and served
through CloudFront with Origin Access Control and no custom frontend hostname.
The browser calls the API Gateway custom hostname directly.
Read the application runbook before synthesis or
operation; it includes automatic license acceptance, one-time registrar
delegation, DNS updates, management-key retrieval, the scaling contract, and
data-recovery limits.

## DNS

`PersonalPlatformDnsStack` owns the retained Route 53 public hosted zone for
`diogo-nb.com.br`, delegated from Registro.br. `vps.diogo-nb.com.br` is the
only record holding the VPS address; `auth`, `dokploy` and `investflow` are
alias records to it, so moving to another server changes one value in
`dns/stack.go`. Add a name to `vpsServiceNames` for each new service on the
VPS.

Stacks that need the zone reference it by ID instead of a cross-stack export,
so they stay deployable and removable independently. The ID is the
`hostedZoneId` context value in `cdk.json`; synthesis of the TeamSpeak stack
fails without it.

### Moving the zone out of the TeamSpeak stack

The zone was created by `PersonalPlatformTeamspeak6Stack` with a retain
policy. It moves without touching any record:

1. Add `"hostedZoneId": "<zone ID>"` to the `context` object in `cdk.json`.
   The ID is in the TeamSpeak stack's current `HostedZoneID` output.
2. With the TeamSpeak service running (see its runbook), deploy
   `PersonalPlatformTeamspeak6Stack`. CloudFormation removes the zone from the
   stack and keeps it. Review the change set first: the zone must show as
   removed. References to it change from `Ref` to the literal ID with the same
   value; if CloudFormation still plans to replace the management API
   certificate, that is safe (ACM validates the new one through the zone and
   API Gateway switches to it), but nothing else should be replaced.
3. Run `cdk import PersonalPlatformDnsStack` and give the zone ID when asked
   for `HostedZone`. Skip the record sets if it asks about them.
4. Deploy `PersonalPlatformDnsStack` to create the VPS records and apply the
   new tags to the zone.

## Cost allocation tags

Both stacks apply the same tag hierarchy to the CloudFormation stack and every
taggable resource:

| Tag | DNS value | TeamSpeak value | Purpose |
|---|---|---|---|
| `Project` | `personal-platform` | `personal-platform` | Aggregate the complete project cost. |
| `Application` | `dns` | `teamspeak6` | Split costs by application. |
| `Component` | Not set | `server`, `management-api`, or `management-web` | Split TeamSpeak costs by deployable component. |
| `Environment` | `production` | `production` | Separate long-lived and future non-production resources. |

The TeamSpeak service propagates its server tags to Fargate tasks and enables
ECS-managed tags. After deployment, [activate `Project`, `Application`,
`Component`, and `Environment`](https://docs.aws.amazon.com/awsaccountbilling/latest/aboutv2/activating-tags.html)
under Billing and Cost Management > Cost allocation tags. AWS can take up to
24 hours to list a new tag key and another 24 hours to activate it. Cost
allocation is not retroactive.

Some charges cannot inherit application tags. In particular, the CDK bootstrap
ECR repository is shared infrastructure rather than a resource created by
either application stack, and a task's dynamic public IPv4 is not a separately
tagged resource. Review those charges separately when reconciling totals.

## Manually managed Skycrate storage

At detachment, the bucket used S3-managed encryption, versioning, blocked public
access, bucket-owner-enforced ownership, and a bucket policy that rejects
requests made without TLS. Manage its configuration, lifecycle rules, policy,
and tags directly in S3. CDK deployments do not manage this bucket or its policy.

Objects tagged `storage-tier=cold` transition from S3 Glacier Flexible
Retrieval to S3 Glacier Deep Archive after 365 days. Noncurrent versions expire
365 days after they become noncurrent. Incomplete multipart uploads are aborted
after seven days, and expired delete markers are removed.

## Local validation

```sh
cd ../../apps/dns-updater
go mod verify
go test -race ./...
go vet ./...
go build ./...

cd ../ts6-management-api
go mod verify
go test -race ./...
go vet ./...
go build ./...

cd ../ts6-management-web
npm ci
npm test
npm run build

cd ../../../infra/aws
go test ./...
go vet ./...
go build ./...
cdk synth PersonalPlatformDnsStack --no-lookups
cdk synth PersonalPlatformTeamspeak6Stack --no-lookups
```

`cdk synth` generates local CloudFormation templates. Review every synthesized
template and `cdk diff` before deployment.

Only the user runs commands that contact or mutate AWS, including `cdk
bootstrap`, `cdk diff`, `cdk deploy`, and `cdk destroy`. Agents must provide the
exact command for review and wait for sanitized output.

Read the scoped documentation and `AGENTS.md` before implementing or operating
a service-specific stack.
