# mrva-go-hepc

Go implementation of the MRVA HTTP Endpoint for CodeQL (HEPC) interface.

This is a prototype implementation of the [`hohn/mrvahepc`](https://github.com/hohn/mrvahepc) Python interface, providing a standard HTTP API for serving CodeQL database collections to the Multi-Repository Variant Analysis (MRVA) system.

## Features

- **Multiple Storage Backends**: Support for local filesystem and Google Cloud Storage
- **Database File Serving**: Serve CodeQL database `.zip` files
- **Metadata API**: Query database metadata from SQLite in JSONL format
- **Standards-Based**: Compatible with the MRVA HEPC interface specification
- **Pure Go**: Uses `modernc.org/sqlite` for CGO-free SQLite support
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

### Local Filesystem

```
db-collection/
├── metadata.sql           # SQLite database with metadata table
├── owner-repo-xxx.zip     # CodeQL database files
└── ...
```

### Google Cloud Storage

```
gs://my-bucket/
├── [prefix/]metadata.sql  # SQLite database with metadata table
├── [prefix/]owner-repo-xxx.zip
└── ...
```

### Metadata Schema

The `metadata.sql` SQLite database should have a `metadata` table with these columns:

| Column                  | Type    | Description                            |
|-------------------------|---------|----------------------------------------|
| `content_hash`          | TEXT    | SHA-256 hash of database file (PK)     |
| `build_cid`             | TEXT    | Build context identifier               |
| `git_branch`            | TEXT    | Git branch name                        |
| `git_commit_id`         | TEXT    | Git commit SHA                         |
| `git_owner`             | TEXT    | Repository owner                       |
| `git_repo`              | TEXT    | Repository name                        |
| `ingestion_datetime_utc`| TEXT    | Database creation timestamp            |
| `primary_language`      | TEXT    | Primary programming language           |
| `result_url`            | TEXT    | Download URL for the database          |
| `tool_name`             | TEXT    | CodeQL tool name                       |
| `tool_version`          | TEXT    | CodeQL CLI version                     |
| `projname`              | TEXT    | Project name (owner/repo)              |
| `db_file_size`          | INTEGER | Database file size in bytes            |

## Project Structure

```
mrva-go-hepc/
├── api/                        # Public API types
│   └── types.go                # DatabaseMetadata struct
├── cmd/
│   └── hepc-server/            # Server executable
│       └── main.go
├── internal/
│   ├── server/                 # HTTP server implementation
│   │   └── server.go
│   └── storage/                # Storage backend abstraction
│       ├── storage.go          # Backend interface
│       ├── local/              # Local filesystem backend
│       │   └── local.go
│       └── gcs/                # Google Cloud Storage backend
│           └── gcs.go
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
