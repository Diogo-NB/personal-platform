# Skycrate AWS infrastructure

This AWS CDK application provisions the `skycrate-storage` S3 bucket in
`us-east-1` for Skycrate.

The bucket uses S3-managed encryption, versioning, blocked public access,
bucket-owner-enforced ownership, and a bucket policy that rejects requests made
without TLS. CloudFormation retains the bucket if the stack is deleted or
replaced.

Objects tagged `storage-tier=cold` transition from S3 Glacier Flexible
Retrieval to S3 Glacier Deep Archive after 365 days. Noncurrent versions expire
365 days after they become noncurrent. Incomplete multipart uploads are aborted
after seven days, and expired delete markers are removed.

## Commands

```sh
go test ./...
go vet ./...
go build ./...
cdk synth
```

`cdk synth` only generates a local CloudFormation template. Review that template
before using `cdk deploy`, which changes the configured AWS account.
