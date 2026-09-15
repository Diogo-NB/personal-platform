# Personal AWS infrastructure

This directory contains the AWS CDK v2 application for personal infrastructure.
The application is written in Go and can contain independent stacks in
different AWS Regions.

## Stacks

| Stack | Region | Status | Documentation |
|---|---|---|---|
| `PersonalPlatformStorageStack` | `us-east-1` | Implemented | This document |
| `PersonalPlatformTeamspeak6Stack` | `sa-east-1` | Implemented | [TeamSpeak 6](../../apps/ts6/README.md) |

The existing storage stack provisions the `skycrate-storage` S3 bucket in
`us-east-1`. Do not change its Region when adding regional stacks to this CDK
application.

The TeamSpeak stack provisions a single public ECS Fargate Spot task in
`sa-east-1`, backed by an encrypted retained EFS One Zone file system. Its
Docker image is built from `apps/ts6` and published as a CDK ECR asset. A
separate reusable Go Lambda container asset is built from `apps/dns-updater`;
an EventBridge rule invokes the TeamSpeak-configured deployment when its ECS
task reaches `RUNNING`, keeping `ts.diogo-nb.com.br` in the retained Route 53
public hosted zone pointed at the current task's dynamic public IPv4. Read the
application runbook before synthesis or operation; it includes automatic
license acceptance, one-time registrar delegation, DNS updates, the scaling
contract, and data-recovery limits.

## Cost allocation tags

Both stacks apply the same tag hierarchy to the CloudFormation stack and every
taggable resource:

| Tag | Storage value | TeamSpeak value | Purpose |
|---|---|---|---|
| `Project` | `personal-platform` | `personal-platform` | Aggregate the complete project cost. |
| `Application` | `skycrate` | `teamspeak6` | Split costs by application. |
| `Environment` | `production` | `production` | Separate long-lived and future non-production resources. |

The TeamSpeak service propagates its tags to Fargate tasks and enables
ECS-managed tags. After deployment, [activate at least `Project`, `Application`,
and `Environment`](https://docs.aws.amazon.com/awsaccountbilling/latest/aboutv2/activating-tags.html)
under Billing and Cost Management > Cost allocation tags. AWS can take up to
24 hours to list a new tag key and another 24 hours to activate it. Cost
allocation is not retroactive.

Some charges cannot inherit application tags. In particular, the CDK bootstrap
ECR repository is shared infrastructure rather than a resource created by
either application stack, and a task's dynamic public IPv4 is not a separately
tagged resource. Review those charges separately when reconciling totals.

The bucket uses S3-managed encryption, versioning, blocked public access,
bucket-owner-enforced ownership, and a bucket policy that rejects requests made
without TLS. CloudFormation retains the bucket if the stack is deleted or
replaced.

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

cd ../../infra/aws
go test ./...
go vet ./...
go build ./...
cdk synth PersonalPlatformStorageStack --no-lookups
cdk synth PersonalPlatformTeamspeak6Stack --no-lookups
```

`cdk synth` generates local CloudFormation templates. Review every synthesized
template and `cdk diff` before deployment.

Only the user runs commands that contact or mutate AWS, including `cdk
bootstrap`, `cdk diff`, `cdk deploy`, and `cdk destroy`. Agents must provide the
exact command for review and wait for sanitized output.

Read the scoped documentation and `AGENTS.md` before implementing or operating
a service-specific stack.
