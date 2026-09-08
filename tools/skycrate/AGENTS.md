# Skycrate agent instructions

These instructions apply to every task under `tools/skycrate/`.

## Start here

- Read `README.md` before designing or implementing a change. It defines the
  current CLI behavior, configuration, architecture, and development commands.
- Inspect the relevant ports and their implementations before changing a
  contract. Update every adapter and test affected by a contract change.
- Treat the current implementation as a real AWS S3 workflow. Keep AWS SDK
  concerns inside the S3 adapter and source-file transfer in the output port.

## Architecture

- Preserve the ports-and-adapters dependency direction:
  - `internal/domain/` owns business concepts and imports no application, port,
    adapter, Cobra, or Viper package.
  - `internal/port/in/` defines use-case contracts consumed by driving adapters.
  - `internal/port/out/` defines technology-neutral repository contracts used by
    the application layer.
  - `internal/application/` implements use cases and depends on domain and ports.
  - `internal/adapter/in/cli/` owns Cobra commands, prompts, and terminal I/O.
  - `internal/adapter/out/s3/` and `internal/adapter/out/config/` own external
    technology details.
  - `main.go` is the manual composition root.
- Keep distinct responsibilities in distinct files. Lift and List services,
  ports, command implementations, filesystem traversal, and path normalization
  must not be collapsed into catch-all files.
- Keep tool-specific concepts in domain or application packages. Generic
  utilities must not know about categories, objects, tiers, S3, or CLI behavior.
- Keep the established directory names `in`, `out`, `s3`, and `util`. Do not
  restore `inbound`, `outbound`, `aws`, or `stringutil`.
- Prefer explicit constructor injection. Avoid package globals and implicit
  initialization.

## Behavior invariants

- `lift` accepts one regular file or one directory. Directory traversal is
  always recursive; do not add a user-facing recursive flag.
- Recursive lifts preserve each file's normalized path relative to the source
  directory. The source directory name itself is not part of the object key.
- Reject symbolic links, special files, directory inputs with no regular files,
  and object-key collisions caused by normalization before saving any object.
- Create and summarize every object before requesting lift approval. A rejected
  approval must return successfully without saving any object.
- Resolve storage tiers from the most specific configured category, falling
  back through complete ancestor segments and then to the `default` tier.
- Keep domain timestamps in UTC. Preserve provider-owned values when
  rehydrating listed objects.
- Write command results to stdout. Write prompts, diagnostics, and errors to
  stderr. Use Cobra's command readers and writers so tests can capture I/O.
- The S3 repository must upload file contents with Skycrate metadata and the
  storage class selected from the semantic storage tier. Listings must ignore
  objects that do not have valid Skycrate metadata.

## Go implementation rules

- Follow idiomatic Go naming and error wrapping. Keep error messages lowercase
  and preserve causes with `%w`.
- Pass `context.Context` as the first parameter and propagate it through ports,
  services, and adapters. Check cancellation in recursive or long-running work.
- Define interfaces at architectural boundaries and keep them focused. Add a
  compile-time implementation check when a concrete type implements a port.
- Add dependencies only when the standard library or current dependencies do
  not provide a clear solution.
- Update `README.md` in the same change when CLI behavior, configuration,
  architecture, output, or development commands change.

## Verification

Run commands from `tools/skycrate/`. Format changed Go files, then run:

```sh
gofmt -w <changed-go-files>
gopls check main.go internal/**/*.go
go test ./...
go test -race ./...
go vet ./...
go build .
```

Add or update tests for observable success and failure behavior. For CLI changes,
test command arguments, stdout, stderr, and help text. For recursive filesystem
behavior, cover nested paths, invalid entries, cancellation, and normalization
collisions when relevant.
