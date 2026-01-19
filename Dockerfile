# Multi-stage build for mrva-go-hepc
# Produces a minimal container image for the HEPC server

# ============================================================================
# Stage 1: Build the Go binary
# ============================================================================
FROM golang:1.25-alpine AS builder

# Install git for version information and ca-certificates for HTTPS
RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build arguments for version information
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown

# Build the binary with static linking; let Buildx set GOARCH based on target platform
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-s -w -X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildTime=${BUILD_TIME}" \
    -o /hepc-server \
    ./cmd/hepc-server

# ============================================================================
# Stage 2: Create minimal runtime image
# ============================================================================
FROM alpine:3.20

# Install ca-certificates for HTTPS (required for GCS) and wget for healthcheck
RUN apk --no-cache add ca-certificates tzdata wget

# Create non-root user for security
RUN adduser -D -g '' hepc
USER hepc

# Copy the binary from builder
COPY --from=builder /hepc-server /usr/local/bin/hepc-server

# Expose the default HEPC port
EXPOSE 8070

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8070/health || exit 1

# Default entrypoint
ENTRYPOINT ["hepc-server"]

# Default command (can be overridden)
# For local storage: --storage local --db-dir /data
# For GCS storage: --storage gcs --gcs-bucket <bucket>
CMD ["--storage", "local", "--host", "0.0.0.0", "--port", "8070", "--db-dir", "/data"]
