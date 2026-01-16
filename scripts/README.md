# Development Scripts

This directory contains utility scripts for development and release management.

## Available Scripts

### install-hooks.sh

Installs git pre-commit hooks that automatically run quality checks before each commit.

```bash
./scripts/install-hooks.sh
```

The pre-commit hook will:
- Format code with `go fmt`
- Run `go vet` for static analysis
- Run tests with `go test -short`

To skip hooks for a specific commit:
```bash
git commit --no-verify
```

### release.sh

Builds release binaries for multiple platforms.

```bash
# Build with auto-detected version from git tags
./scripts/release.sh

# Build with specific version
./scripts/release.sh v1.0.0
```

Builds for:
- macOS (Intel and Apple Silicon)
- Linux (amd64 and arm64)
- Windows (amd64)

Output: `dist/` directory with compressed binaries

## Usage with Makefile

Most development tasks should use the Makefile instead:

```bash
make build        # Build binary
make test         # Run tests
make check        # Run all checks
make install      # Install to GOPATH
```

See `make help` for all available targets.
