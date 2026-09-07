# Skycrate

Skycrate is a Go command-line tool that assigns local files to hierarchical
object categories and routes those categories to cloud-storage tiers. It uses
one configured bucket and isolates storage access behind an object repository.

Skycrate currently uses a mocked S3 repository. `lift` validates a local file,
creates its storage metadata, and acknowledges the save without uploading file
contents. `list` summarizes deterministic mocked objects.

## Build

```sh
go build -o skycrate .
```

## Configuration

Skycrate requires a YAML configuration containing one bucket and at least one
category-to-tier mapping:

```yaml
bucket: skycrate-storage

categories:
  documents:
    tier: STANDARD
  photos:
    tier: STANDARD_IA
  backups:
    tier: DEEP_ARCHIVE
  backups/university:
    tier: GLACIER
```

By default, Skycrate reads `skycrate/config.yaml` from the platform user
configuration directory. On Linux this is normally
`$XDG_CONFIG_HOME/skycrate/config.yaml`, falling back to
`~/.config/skycrate/config.yaml`.

Use `--config` to select another file:

```sh
skycrate --config ./skycrate.yaml list
```

Bucket and tier values must not be blank or contain surrounding whitespace.
Their casing is preserved. Category paths are normalized during application
initialization.

## Hierarchical categories

A category is a slash-delimited path. Skycrate selects the tier from the most
specific configured category and falls back through its ancestors.

Given this configuration:

```yaml
categories:
  backups:
    tier: DEEP_ARCHIVE
  backups/university:
    tier: GLACIER
```

the resolution behavior is:

| Requested category | Matched mapping | Tier |
| --- | --- | --- |
| `backups` | `backups` | `DEEP_ARCHIVE` |
| `backups/personal` | `backups` | `DEEP_ARCHIVE` |
| `backups/university` | `backups/university` | `GLACIER` |
| `backups/university/thesis` | `backups/university` | `GLACIER` |

Matching follows complete path segments. A mapping for `backups` does not match
`backups-old`.

Category segments and filenames are trimmed, lowercased, and converted to a
conservative ASCII slug. Whitespace runs become `-`; letters, numbers, `.`, `_`,
and `-` are accepted. Leading, trailing, and repeated category slashes are
rejected.

## Lift objects

```console
skycrate lift <file-path> [object-category]
```

With an explicit category:

```console
$ skycrate lift "./My Thesis.PDF" backups/university/coursework
Lifted metadata (mock): s3://skycrate-storage/backups/university/coursework/my-thesis.pdf
```

The category can be an unconfigured descendant when one of its ancestors is
configured.

When the category is omitted, Skycrate displays the configured mappings as a
numbered list on stderr. After a selection, it asks for an optional descendant
suffix. The combined path is resolved again, so a more-specific configured
mapping can take precedence.

`lift` accepts readable, regular, non-symbolic-link files, including empty
files. It creates an object containing:

- Provider path
- Normalized filename
- Full normalized category
- Size in bytes
- Resolved tier
- UTC creation and update timestamps

The mocked repository receives that complete object and returns success. File
contents and the local source path are intentionally absent from the repository
contract in this phase, so no bytes are uploaded or persisted.

For inspection, the mock S3 repository prints the complete object to stderr
using field names before Lift writes its success result to stdout. The diagnostic
includes `Path`, `Name`, `Category`, `Size`, `Tier`, `CreatedAt`, and `UpdatedAt`.

## List objects

```console
skycrate list
skycrate list --category backups
skycrate list --tier DEEP_ARCHIVE
skycrate list --category backups --tier DEEP_ARCHIVE
```

Filters are case-insensitive and combine with AND semantics. A category filter
matches the exact category and all descendants on segment boundaries.

Output uses decimal gigabytes:

```text
Objects: 3
Total size: 10.000 GB
```

The S3 repository currently returns these fixed objects:

| Path | Category | Tier | Size |
| --- | --- | --- | ---: |
| `documents/report.pdf` | `documents` | `STANDARD` | 1 GB |
| `photos/photo.jpg` | `photos` | `STANDARD_IA` | 2 GB |
| `backups/database.dump` | `backups` | `DEEP_ARCHIVE` | 7 GB |

These records are rehydrated as provider-owned data. Their paths, categories,
tiers, sizes, and timestamps are not recomputed from current configuration.

## Architecture

Skycrate uses a ports-and-adapters layout with explicit driving (`in`) and
driven (`out`) boundaries:

```text
internal/
├── domain/
│   ├── category/              # Category paths, catalog, and tier resolution
│   └── object/                # Object construction and validation
├── port/
│   ├── in/                    # Lifter and Lister application contracts
│   └── out/                   # ObjectRepository storage contract
├── application/
│   ├── lift.go                # LiftService implementation
│   └── list.go                # ListService implementation
├── adapter/
│   ├── in/cli/                # Cobra commands and interactive input
│   └── out/
│       ├── s3/                # Mocked ObjectRepository implementation
│       └── config/            # YAML configuration loader
└── util/                      # Generic string normalization and slugging
```

`LiftService` and `ListService` independently implement the input ports. Both
depend on the combined `ObjectRepository` output port, which exposes
`Save(context.Context, object.Object) error` and
`FindMany(context.Context, FindManyRequest) ([]object.Object, error)`.

The CLI adapter knows only the input ports and domain category catalog. Root
`main.go` is the manual composition root: it loads configuration, constructs the
mocked S3 repository, builds both services, and injects them into the CLI.
Domain packages do not import application, ports, adapters, Cobra, or Viper.

## Roadmap

- Extend the save contract to carry file content without putting transient I/O
  state on the domain object.
- Replace the mocked S3 behavior with AWS SDK upload and listing calls.
- Tag uploaded objects with their complete category path.
- Return provider upload metadata and elapsed milliseconds.
- Add machine-readable output where it benefits automation.

## Development

```sh
gofmt -w main.go internal
gopls check main.go internal/**/*.go
go test ./...
go test -race ./...
go vet ./...
go build .
```

## License

See [LICENSE](./LICENSE).
