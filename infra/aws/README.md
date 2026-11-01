# Personal AWS infrastructure

This directory contains the Go AWS CDK v2 application. TeamSpeak now runs only
on the VPS; its [runbook](../vps/teamspeak/README.md) owns server operation.

## Stacks

| Stack | Region | Purpose |
|---|---|---|
| `PersonalPlatformDnsStack` | `sa-east-1` | Retained public hosted zone and VPS service records |

The operator confirmed deletion of the former `PersonalPlatformTeamspeak6Stack`.
Retained EFS storage, stale DNS records, and old shared assets require separate
cleanup verification.

## DNS

`PersonalPlatformDnsStack` owns the retained Route 53 public hosted zone for
`diogo-nb.com.br`, delegated from Registro.br. `vps.diogo-nb.com.br` holds the
VPS address; `auth`, `dokploy`, `investflow`, and `teamspeak` alias it. Moving
servers changes only the address in `dns/stack.go`. Add a name to
`vpsServiceNames` for each new VPS service.

`teamspeak.diogo-nb.com.br` is the permanent VPS TeamSpeak address. Retire the
old AWS names `ts.diogo-nb.com.br` and `ts-management-api.diogo-nb.com.br`
if they remain. Do not remove the VPS TeamSpeak alias or the zone.

The shared zone was moved out of the former TeamSpeak stack and is owned by
the DNS stack. The former `hostedZoneId` context is no longer needed.

## Cost allocation tags

The DNS stack applies `Project=personal-platform`, `Application=dns`, and
`Environment=production` to the stack and its taggable resources. Activate
these keys in AWS Billing for cost allocation; allocation is not retroactive.

CDK bootstrap image and file assets are shared infrastructure. Old TeamSpeak
assets may remain there after stack deletion; review them separately rather
than deleting the bootstrap stack or repositories.

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

Run from `infra/aws`:

```sh
go test ./...
go vet ./...
go build ./...
cdk synth PersonalPlatformDnsStack --no-lookups
```

The CDK app no longer needs Docker or the retired management web build.
Synthesis generates local templates. Review `cdk diff PersonalPlatformDnsStack`
before any DNS deployment.

Only the user runs commands that contact AWS, including resource queries,
bootstrap, diff, deploy, and deletion. Confirm the AWS account, `sa-east-1`,
cost, and blast radius first. Preserve the shared hosted zone, VPS records,
CDK bootstrap resources, and manually managed `skycrate-storage` bucket.
