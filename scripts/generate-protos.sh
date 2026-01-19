#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "Generating Protocol Buffers..."

cd "$PROJECT_ROOT/protos"

# Install buf if not present
if ! command -v buf &> /dev/null; then
    echo "Installing buf..."
    go install github.com/bufbuild/buf/cmd/buf@latest
fi

# Generate
buf generate

echo "Protocol Buffers generated successfully!"
