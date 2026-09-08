# Skycrate

Skycrate is a Go command-line tool that assigns local files and directory trees
to hierarchical object categories and routes those categories to cloud-storage
tiers. It uses one configured bucket and isolates storage access behind an
object repository.

Skycrate uses the AWS SDK for Go v2. `lift` validates a local file or recursively
traverses a directory, then uploads every file with the AWS S3 transfer manager.
`list` summarizes Skycrate-managed objects stored in the configured bucket.

## Install

Install the latest release on Linux amd64 or macOS arm64:

```sh
curl -fsSL https://raw.githubusercontent.com/Diogo-NB/personal-platform/main/tools/skycrate/install.sh | sh
```

To install a specific stable version, pass it after `sh -s --`:

```sh
curl -fsSL https://raw.githubusercontent.com/Diogo-NB/personal-platform/main/tools/skycrate/install.sh | sh -s -- 0.1.0
```

The installer requires `curl`, `tar`, `awk`, and either `sha256sum` or
`shasum`. It verifies the selected release archive, installs the executable at
`~/.local/bin/skycrate`, and warns if that directory is not in `PATH`.

On Linux, the installer creates the initial configuration at
`${XDG_CONFIG_HOME:-$HOME/.config}/skycrate/config.yaml`. On macOS, it uses
`~/Library/Application Support/skycrate/config.yaml`, matching Skycrate's
platform-native default. An existing configuration is never overwritten.

## Build

```sh
go build -o skycrate .
```

## Configuration

Skycrate requires a YAML configuration containing one bucket, one AWS region,
and at least one category-to-tier mapping. The default configuration is:

```yaml
bucket: skycrate-storage
region: us-east-1

categories:
  backup:
    tier: archive
  recordings:
    tier: cold
  documents:
    tier: instant
```

By default, Skycrate reads `skycrate/config.yaml` from the platform user
configuration directory. On Linux this is normally
`$XDG_CONFIG_HOME/skycrate/config.yaml`, falling back to
`~/.config/skycrate/config.yaml`.

Use `--config` to select another file:

```sh
skycrate --config ./skycrate.yaml list
```

Bucket, region, and tier values must not be blank or contain surrounding
whitespace. Tier input is case-insensitive and is normalized to one of three
semantic storage tiers:

| Storage tier | Initial S3 storage class | Lifecycle behavior |
| --- | --- | --- |
| `archive` | S3 Glacier Deep Archive (`DEEP_ARCHIVE`) | None |
| `cold` | S3 Glacier Flexible Retrieval (`GLACIER`) | Transitions to Deep Archive after 365 days |
| `instant` | S3 Glacier Instant Retrieval (`GLACIER_IR`) | None |

These values are intentionally not raw S3 storage-class names. Category paths
are normalized during application initialization.

Skycrate uses the standard AWS SDK credential chain. Configure credentials with
the same supported environment variables, shared AWS config/credentials files,
or workload identity that other AWS SDK applications use.

## Hierarchical categories

A category is a slash-delimited path. Skycrate selects the tier from the most
specific configured category and falls back through its ancestors.

Given this configuration:

```yaml
categories:
  backup:
    tier: archive
  backup/university:
    tier: cold
```

the resolution behavior is:

| Requested category | Matched mapping | Tier |
| --- | --- | --- |
| `backup` | `backup` | `archive` |
| `backup/personal` | `backup` | `archive` |
| `backup/university` | `backup/university` | `cold` |
| `backup/university/thesis` | `backup/university` | `cold` |

Matching follows complete path segments. A mapping for `backup` does not match
`backup-old`.

Category segments and filenames are trimmed, lowercased, and converted to a
conservative ASCII slug. Whitespace runs become `-`; letters, numbers, `.`, `_`,
and `-` are accepted. Leading, trailing, and repeated category slashes are
rejected.

## Lift objects

```console
skycrate lift <source-path> [object-category]
```

With an explicit category:

```console
$ skycrate lift --yes "./My Thesis.PDF" backup/university/coursework
Lifted: s3://skycrate-storage/backup/university/coursework/my-thesis.pdf
```

With a directory:

```console
$ skycrate lift --yes "./Research Notes" documents
Lifted: s3://skycrate-storage/documents/drafts/outline.md
Lifted: s3://skycrate-storage/documents/final-report.pdf
```

Directories are always traversed recursively, matching S3 recursive transfer
semantics; Skycrate does not expose a separate recursive flag. The source
directory itself is not added to the object key. Each path relative to that
directory is preserved and normalized, so
`./Research Notes/Drafts/Outline.md` becomes
`documents/drafts/outline.md` in the example above.

The category can be an unconfigured descendant when one of its ancestors is
configured.

After validating the complete source and creating its objects, `lift` reports
the object count, exact byte total, decimal-gigabyte total, and resolved storage
tier on stderr. It then asks for confirmation:

```text
Objects: 3
Total size: 1500000000 bytes (1.500 GB)
Storage tier: cold
Continue with upload? [y/N]:
```

Enter `y` or `yes`, case-insensitively, to upload. Enter `n`, `no`, or press
Enter to cancel successfully without uploading anything. Other responses retry
the prompt. Use `--yes` or `-y` to approve without printing the summary or
prompt, which is useful for scripts.

When the category is omitted, Skycrate displays the configured mappings as a
numbered list on stderr. After a selection, it asks for an optional descendant
suffix. The combined path is resolved again, so a more-specific configured
mapping can take precedence.

`lift` accepts a readable regular file, including an empty file, or a directory
containing at least one regular file. Symbolic links and other special files are
rejected anywhere in the source tree. It creates an object for each file
containing:

- Provider path
- Normalized filename
- Normalized path relative to the source directory
- Full normalized category
- Size in bytes
- Resolved tier
- UTC creation and update timestamps

The S3 adapter uploads each regular file with a SHA-256 checksum, the storage
class selected from its semantic tier, a `storage-tier` object tag, and
Skycrate category/tier metadata. It also checks that the source size has not
changed between traversal and upload.

## List objects

```console
skycrate list
skycrate list --category backup
skycrate list --tier archive
skycrate list --category backup --tier archive
```

Filters are case-insensitive and combine with AND semantics. A category filter
matches the exact category and all descendants on segment boundaries.

Output uses decimal gigabytes:

```text
Objects: 3
Total size: 10.000 GB
```

Listing is paginated and uses object metadata to distinguish Skycrate-managed
objects from unrelated objects in the same bucket. Unmanaged or malformed
objects are ignored. AWS list or metadata request failures are returned instead
of being silently skipped.

## Architecture

Skycrate uses a ports-and-adapters layout with explicit driving (`in`) and
driven (`out`) boundaries:

```text
internal/
├── domain/
│   ├── category/              # Category paths, catalog, and tier resolution
│   ├── object/                # Object construction and validation
│   └── storage/               # Semantic storage-tier enum and parsing
├── port/
│   ├── in/                    # Lifter and Lister application contracts
│   └── out/                   # ObjectRepository storage contract
├── application/
│   ├── lift.go                # LiftService implementation
│   ├── list.go                # ListService implementation
│   └── source.go              # Recursive local source traversal
├── adapter/
│   ├── in/cli/                # Cobra commands and interactive input
│   └── out/
│       ├── s3/                # AWS SDK ObjectRepository implementation
│       └── config/            # YAML configuration loader
└── util/                      # Generic string normalization and slugging
```

`LiftService` and `ListService` independently implement the input ports. Lift
creates one object for a file or an ordered object slice for a recursive
directory traversal, requests approval through a function supplied by the
driving adapter, and returns a result that distinguishes cancellation from a
completed upload. Both services depend on the combined `ObjectRepository`
output port, which exposes `Save(context.Context, SaveRequest) error` and
`FindMany(context.Context, FindManyRequest) ([]object.Object, error)`.

The CLI adapter knows only the input ports and domain category catalog. Root
`main.go` is the manual composition root: it loads configuration, constructs the
AWS S3 client and transfer manager, builds both services, and injects them into
the CLI.
Domain packages do not import application, ports, adapters, Cobra, or Viper.

## Roadmap

- Return provider upload metadata and elapsed milliseconds.
- Add machine-readable output where it benefits automation.

## Releases

Repository releases provide Skycrate for these platforms:

| Platform | Architecture | Asset |
| --- | --- | --- |
| Linux | amd64 | `skycrate_<version>_linux_amd64.tar.gz` |
| macOS | arm64 | `skycrate_<version>_darwin_arm64.tar.gz` |

Each archive contains the `skycrate` binary, this README, and the MIT license.
Download both archives and `SHA256SUMS` into the same directory, then verify
them on Linux with:

```sh
sha256sum --check SHA256SUMS
```

On macOS, use:

```sh
shasum -a 256 --check SHA256SUMS
```

Maintainers publish a release from the repository's **Actions** tab: select
**Publish release**, choose **Run workflow**, and enter a stable version such as
`0.1.0`. The workflow creates the corresponding `v0.1.0` tag and GitHub release
from the selected commit.

Releases use one repository-wide version. Future tools will be packaged by the
same root workflow and included in the same repository release.

## Development

AI coding agents must read and follow [`AGENTS.md`](./AGENTS.md) before changing
Skycrate. Repository-level instruction routers make this guide available to
Codex and Claude Code.

```sh
gofmt -w main.go internal
gopls check main.go internal/**/*.go
go test ./...
go test -race ./...
go vet ./...
go build .
sh -n install.sh
```

## License

See [LICENSE](./LICENSE).
