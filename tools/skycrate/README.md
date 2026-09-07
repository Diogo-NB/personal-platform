# Skycrate

Skycrate is a Go command-line tool for moving local files into named cloud
storage buckets and, eventually, listing the files stored there.

## Current behavior

The `lift` command is preview-only. It reads a local file, prints its contents
and the requested bucket name, and does not contact AWS or upload anything.

```console
$ skycrate lift ./file.extension bucket-name
File contents:
<file contents>
Bucket: bucket-name
```

The file path must point to a readable file. Skycrate preserves the file
contents verbatim and adds a newline before the bucket line only when the file
does not already end with one. The bucket name is displayed without validation.

## Build and run

Build the binary from this directory:

```sh
go build -o skycrate .
```

Run the built binary:

```sh
./skycrate lift ./file.extension bucket-name
```

You can also run Skycrate without creating a binary:

```sh
go run . lift ./file.extension bucket-name
```

## Intended experience

Skycrate is intended to provide a small file-oriented workflow:

- `skycrate lift <file-path> <bucket-name>` will upload one local file to a
  named bucket.
- A future list command will show the files stored in a named bucket.
- Command output and errors will remain suitable for terminal use and scripts.

## Roadmap

- Validate bucket names.
- Configure AWS credentials and region selection.
- Upload file contents to object storage.
- Add a command for listing objects in a bucket.
- Add machine-readable output where it benefits automation.

## Development

Format, analyze, test, and build the module with:

```sh
gofmt -w main.go cmd/*.go
gopls check main.go cmd/root.go cmd/lift.go cmd/root_test.go cmd/lift_test.go
go test ./...
go test -race ./...
go vet ./...
go build ./...
```

## License

See [LICENSE](./LICENSE).
