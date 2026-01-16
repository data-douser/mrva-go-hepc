#!/bin/bash
# Build release binaries for multiple platforms

set -e

VERSION=${1:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")}
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS="-X main.Version=$VERSION -X main.Commit=$COMMIT -X main.BuildTime=$BUILD_TIME"

OUTPUT_DIR="dist"
BINARY_NAME="hepc-server"

# Platforms to build for
PLATFORMS=(
    "darwin/amd64"
    "darwin/arm64"
    "linux/amd64"
    "linux/arm64"
    "windows/amd64"
)

echo "Building release version: $VERSION"
echo "Commit: $COMMIT"
echo "Output directory: $OUTPUT_DIR"
echo ""

# Clean and create output directory
rm -rf "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR"

# Build for each platform
for platform in "${PLATFORMS[@]}"; do
    IFS='/' read -r -a array <<< "$platform"
    GOOS="${array[0]}"
    GOARCH="${array[1]}"
    
    output_name="${BINARY_NAME}-${GOOS}-${GOARCH}"
    if [ "$GOOS" = "windows" ]; then
        output_name="${output_name}.exe"
    fi
    
    echo "Building $GOOS/$GOARCH..."
    
    GOOS=$GOOS GOARCH=$GOARCH go build \
        -ldflags "$LDFLAGS" \
        -o "$OUTPUT_DIR/$output_name" \
        ./cmd/hepc-server
    
    # Create tarball for non-Windows platforms
    if [ "$GOOS" != "windows" ]; then
        tar -czf "$OUTPUT_DIR/${BINARY_NAME}-${GOOS}-${GOARCH}-${VERSION}.tar.gz" \
            -C "$OUTPUT_DIR" "$output_name"
        rm "$OUTPUT_DIR/$output_name"
    else
        # Create zip for Windows
        (cd "$OUTPUT_DIR" && zip "${BINARY_NAME}-${GOOS}-${GOARCH}-${VERSION}.zip" "$output_name")
        rm "$OUTPUT_DIR/$output_name"
    fi
    
    echo "✓ Built $GOOS/$GOARCH"
done

echo ""
echo "✓ Release binaries built successfully"
echo "Artifacts in: $OUTPUT_DIR"
ls -lh "$OUTPUT_DIR"
