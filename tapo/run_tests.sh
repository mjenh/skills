#!/bin/bash
# Run all tests for the tapo module
set -e

cd "$(dirname "$0")"

echo "=== Building all packages ==="
go build ./...

echo ""
echo "=== Running go vet ==="
go vet ./...

echo ""
echo "=== Testing internal/crypto ==="
go test ./internal/crypto/... -v

echo ""
echo "=== Testing internal/transport/legacy ==="
go test ./internal/transport/legacy/... -v

echo ""
echo "=== Running all tests with race detector ==="
go test -race ./...

echo ""
echo "=== All tests passed ==="
