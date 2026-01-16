# mrva-go-hepc

Go implementation of the MRVA HTTP Endpoint for CodeQL (HEPC) interface.

This is a prototype implementation of the [`hohn/mrvahepc`](https://github.com/hohn/mrvahepc) Python interface, providing a standard HTTP API for serving CodeQL database collections to the Multi-Repository Variant Analysis (MRVA) system.

## Features

- **Multiple Storage Backends**: Support for local filesystem and Google Cloud Storage
- **Dynamic Database Discovery**: Automatically discovers CodeQL databases from directory structures
- **Database File Serving**: Serve CodeQL database `.zip` files or unarchived databases
- **Metadata API**: Query database metadata in JSONL format
- **Standards-Based**: Compatible with the MRVA HEPC interface specification
- **Comprehensive Testing**: 75%+ test coverage using real GCS emulation via [fake-gcs-server](https://github.com/fsouza/fake-gcs-server)
- **GCS Authentication**: Supports service account keys and Application Default Credentials (ADC)

## Installation

```bash
go install github.com/data-douser/mrva-go-hepc/cmd/hepc-server@latest
```

Or build from source:

```bash
git clone https://github.com/data-douser/mrva-go-hepc.git
cd mrva-go-hepc
go build -o hepc-server ./cmd/hepc-server
```

## Usage

```bash
hepc-server --storage <type> [STORAGE OPTIONS] [SERVER OPTIONS]
```

### Storage Backends

The server supports multiple storage backends:

| Backend | Description                          |
|---------|--------------------------------------|
| `local` | Local filesystem storage (default)   |
| `gcs`   | Google Cloud Storage                 |

### Server Options

| Flag     | Default     | Description                      |
|----------|-------------|----------------------------------|
| `--host` | `127.0.0.1` | Host address for the HTTP server |
| `--port` | `8070`      | Port for the HTTP server         |
| `--help` | -           | Show help message                |

### Local Storage Options

| Flag       | Description                                |
|------------|--------------------------------------------|
| `--db-dir` | Directory containing CodeQL database files |

### GCS Storage Options

| Flag              | Description                                      |
|-------------------|--------------------------------------------------|
| `--gcs-bucket`    | GCS bucket name (required)                       |
| `--gcs-prefix`    | Object path prefix within the bucket (optional)  |
| `--gcs-credentials` | Path to service account JSON key file (optional) |
| `--gcs-cache-dir` | Local directory for caching metadata (optional)  |

## Examples

### Local Filesystem Storage

```bash
# Start the server with local storage
hepc-server --storage local --db-dir ./db-collection

# Test endpoints
curl http://127.0.0.1:8070/health
curl http://127.0.0.1:8070/index | head -1
```

### Google Cloud Storage

```bash
# Using Application Default Credentials (ADC)
hepc-server --storage gcs --gcs-bucket my-codeql-databases

# Using service account with object prefix
hepc-server --storage gcs \
    --gcs-bucket my-codeql-databases \
    --gcs-prefix databases/production/ \
    --gcs-credentials /path/to/service-account.json

# With custom server settings
hepc-server --storage gcs \
    --gcs-bucket my-codeql-databases \
    --host 0.0.0.0 \
    --port 8080
```

## GCS Authentication

The GCS backend supports multiple authentication methods:

### 1. Service Account Key File

Specify a JSON key file with `--gcs-credentials`:

```bash
hepc-server --storage gcs \
    --gcs-bucket my-bucket \
    --gcs-credentials /path/to/service-account.json
```

### 2. Application Default Credentials (ADC)

If no credentials file is specified, the client uses ADC in this order:

1. **Environment variable**: `GOOGLE_APPLICATION_CREDENTIALS`
2. **gcloud CLI**: `gcloud auth application-default login`
3. **Compute Engine/GKE**: Automatic service account when running on GCP

```bash
# Set credentials via environment variable
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/credentials.json
hepc-server --storage gcs --gcs-bucket my-bucket

# Or use gcloud for local development
gcloud auth application-default login
hepc-server --storage gcs --gcs-bucket my-bucket
```

### Required GCS Permissions

The service account or authenticated user needs these permissions:

- `storage.objects.get` - Read database files
- `storage.objects.list` - List objects (optional, for future features)

Minimal IAM role: **Storage Object Viewer** (`roles/storage.objectViewer`)

## HTTP Endpoints

| Endpoint                           | Method | Description                              |
|------------------------------------|--------|------------------------------------------|
| `/db/{filename}`                   | GET    | Download a CodeQL database file          |
| `/index`                           | GET    | List all databases (JSONL format)        |
| `/api/v1/latest_results/codeql-all`| GET    | List all databases (JSONL format)        |
| `/health`                          | GET    | Health check endpoint                    |

## Storage Structure

The server dynamically discovers CodeQL databases from the storage backend by scanning for `codeql-database.yml` files or `.zip` archives.

### Local Filesystem

```
db-collection/
├── owner-repo-xxx.zip              # Archived CodeQL database
├── owner-repo-yyy/                 # Unarchived CodeQL database
│   ├── codeql-database.yml         # Database metadata
│   └── db-<language>/              # Language-specific data
└── ...
```

### Google Cloud Storage

```
gs://my-bucket/
├── [prefix/]owner-repo-xxx/        # Unarchived CodeQL database
│   ├── codeql-database.yml         # Database metadata  
│   └── db-<language>/              # Language-specific data
└── ...
```

> **Note**: For GCS, only unarchived databases are supported. Archived `.zip` files require downloading the entire archive to read metadata, which is not acceptable for large databases.

## Testing

The project has comprehensive test coverage (~75%) including:

- **Unit tests** for all packages
- **Integration tests** for GCS using [fake-gcs-server](https://github.com/fsouza/fake-gcs-server)
- **Race detection** enabled in CI

```bash
# Run all tests
go test -v -race ./...

# Run tests with coverage
go test -v -race -coverprofile=coverage.out ./...
go tool cover -func=coverage.out

# Run specific package tests
go test -v ./internal/storage/gcs/...
```

### Test Coverage by Package

| Package | Coverage |
|---------|----------|
| `api/` | 100% |
| `internal/codeql/` | 84% |
| `internal/server/` | 74% |
| `internal/storage/` | 100% |
| `internal/storage/gcs/` | 85% |
| `internal/storage/local/` | 91% |

## Project Structure

```
mrva-go-hepc/
├── api/                        # Public API types
│   ├── types.go                # DatabaseMetadata struct
│   └── types_test.go
├── cmd/
│   └── hepc-server/            # Server executable
│       └── main.go
├── internal/
│   ├── codeql/                 # CodeQL database discovery
│   │   ├── discovery.go
│   │   └── discovery_test.go
│   ├── server/                 # HTTP server implementation
│   │   ├── server.go
│   │   └── server_test.go
│   └── storage/                # Storage backend abstraction
│       ├── storage.go          # Backend interface
│       ├── storage_test.go
│       ├── local/              # Local filesystem backend
│       │   ├── local.go
│       │   └── local_test.go
│       └── gcs/                # Google Cloud Storage backend
│           ├── gcs.go
│           └── gcs_test.go     # Uses fake-gcs-server
├── go.mod
├── go.sum
└── README.md
```

## Project Development

### Quick Start

```sh
# Install development tools
make install-tools

# Run all checks
make check

# Build and test
make test build

# Install git hooks
./scripts/install-hooks.sh
```

## Related Projects

- [`hohn/mrvahepc`](https://github.com/hohn/mrvahepc) - Original Python implementation
- [`hohn/mrva-docker`](https://github.com/hohn/mrva-docker) - Docker deployment for MRVA

## License

MIT License - See [LICENSE](LICENSE) for details.
