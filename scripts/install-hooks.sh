#!/bin/bash
# Install git pre-commit hooks

set -e

HOOK_DIR=".git/hooks"
HOOK_FILE="$HOOK_DIR/pre-commit"

# Ensure we're in a git repository
if [ ! -d ".git" ]; then
    echo "Error: Not in a git repository"
    exit 1
fi

# Create hooks directory if it doesn't exist
mkdir -p "$HOOK_DIR"

# Create pre-commit hook
cat > "$HOOK_FILE" << 'EOF'
#!/bin/bash
# Pre-commit hook for mrva-go-hepc

set -e

echo "Running pre-commit checks..."

# Format code
echo "→ Formatting code..."
go fmt ./...

# Run go vet
echo "→ Running go vet..."
go vet ./...

# Run tests
echo "→ Running tests..."
go test -short ./...

echo "✓ Pre-commit checks passed"
EOF

# Make hook executable
chmod +x "$HOOK_FILE"

echo "✓ Git hooks installed successfully"
echo ""
echo "To run checks manually:"
echo "  make check"
echo ""
echo "To skip hooks for a commit:"
echo "  git commit --no-verify"
