# s3-client

A CLI for AWS S3 with subcommands for common operations. The **download** subcommand performs fast, parallel chunked downloads with a live progress display (speed, ETA, chunk map). More subcommands can be added over time.

## Features

- **Subcommand-based** — `download` (or `dl`) today; extensible for upload, ls, cp, etc.
- **Parallel chunked downloads** — Splits the object into configurable chunks and downloads them concurrently
- **Progress display** — Live progress bar with speed (MB/s), ETA, elapsed time, and per-chunk status
- **Flexible output** — Save to a custom path or use the object key basename
- **AWS integration** — Uses AWS SDK v2; supports profiles, region override, and standard credential chain

## Prerequisites

- **Go 1.25+** (or the version specified in `go.mod`)
- **AWS credentials** with `s3:GetObject` (and `s3:ListBucket` if needed) on the target bucket/key

## Installation

### From source

```bash
git clone <repo-url>
cd s3-client
make build
# Binary: ./s3-client
```

### Install to `$GOPATH/bin`

```bash
make install
```

## Usage

```text
s3-client <command> [options]
```

### Commands

| Command        | Description                          |
|----------------|--------------------------------------|
| `download`, `dl` | Download an object from S3 (parallel chunked) |

Use `s3-client <command> -h` for command-specific help.

### download

```text
s3-client download [flags] s3://bucket/key/path
```

| Flag           | Default | Description                                      |
|----------------|--------|--------------------------------------------------|
| `-output`      | (key basename) | Output file path                          |
| `-chunk-size`  | 10     | Chunk size in MB                                 |
| `-concurrency` | 5      | Number of parallel chunk downloads               |
| `-region`      | (from env/config) | AWS region                               |
| `-profile`     | (from env) | AWS credentials/config profile name        |

#### Examples

```bash
# Download to current directory (filename = key basename)
s3-client download s3://my-bucket/backups/file.tgz

# Use a specific AWS profile and region
s3-client download -profile prod -region us-west-2 s3://my-bucket/data/dump.tar.gz

# Custom output path and tuning
s3-client download -chunk-size 25 -concurrency 8 -output /tmp/file.tgz s3://my-bucket/file.tgz
```

## AWS credentials

The tool uses the default AWS SDK credential chain:

1. **Profile** — `s3-client download -profile myprofile s3://...` or `AWS_PROFILE=myprofile`
2. **Environment** — `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`
3. **Shared config** — `~/.aws/credentials` and `~/.aws/config`

Ensure the credentials have `s3:GetObject` (and `s3:ListBucket` where applicable) on the bucket and key.

## Build (Makefile)

| Target   | Description                    |
|----------|--------------------------------|
| `make` / `make build` | Build the binary (`s3-client`) |
| `make install`        | Install to `$GOPATH/bin`          |
| `make clean`          | Remove built binary               |
| `make deps`           | Download Go module dependencies   |
| `make test`           | Run tests                         |

## License

See repository license file.
