# Personal AWS infrastructure

This directory contains the Go AWS CDK v2 application. TeamSpeak now runs only
on the VPS; its [runbook](../vps/teamspeak/README.md) owns server operation.

## Stacks

| Stack | Region | Purpose |
|---|---|---|
| `PersonalPlatformDnsStack` | `sa-east-1` | Retained public hosted zone, VPS service records, and hosted addy.io email records |

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

### Hosted addy.io email

`dns/addy.go` prepares `diogo-nb.com.br` for hosted addy.io. No mail server runs
on the VPS. These records do not change the domain's web service records.

| Type | Name | Value |
|---|---|---|
| MX | `@` | Priority 10: `mail.anonaddy.me`; priority 20: `mail2.anonaddy.me` |
| TXT | `@` | `v=spf1 include:spf.anonaddy.me -all` |
| TXT | `@` | Account ownership value beginning with `aa-verify=` in `dns/addy.go` |
| CNAME | `dk1._domainkey` | `dk1._domainkey.anonaddy.me` |
| CNAME | `dk2._domainkey` | `dk2._domainkey.anonaddy.me` |
| TXT | `_dmarc` | `v=DMARC1; p=quarantine; adkim=s` |

The two apex TXT values share one Route 53 record set. The ownership value is
public DNS data, not an API credential. Deployment and addy.io verification
are not yet confirmed.

After local validation, the operator confirms the AWS account, `sa-east-1`,
cost, and blast radius, then runs from `infra/aws`:

```sh
cdk diff PersonalPlatformDnsStack
cdk deploy PersonalPlatformDnsStack
```

The diff must add only the five mail record sets; preserve the hosted zone
and all five existing VPS record sets. Extra DNS records do not require a
second hosted zone. Route 53 DNS query charges still apply.

Once DNS propagates, enter `diogo-nb.com.br` in addy.io's **Domains > Add Domain**
dialog and submit. Open **Check Records** and verify receiving and sending.
Choose a verified recipient at another domain as the default inbox. Create an
alias, send a test email from a different inbox, and test a reply through the
alias. Catch-all is enabled by default; disable it if only explicitly created
aliases should receive mail.

These records authorize addy.io sending. If SES is later used for Keycloak,
configure its sender authentication before sending under this domain.

Reference: [addy.io custom domain setup](https://addy.io/help/adding-a-custom-domain/).

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
