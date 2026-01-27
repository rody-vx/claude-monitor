#!/bin/bash

# Claude Monitor Build Script
# Builds binaries for macOS (Intel/ARM) and Windows

set -e

VERSION="${1:-dev}"
OUTPUT_DIR="dist"
APP_NAME="claude-monitor"
LDFLAGS="-s -w -X main.version=${VERSION}"

echo "Building Claude Monitor ${VERSION}..."
echo ""

# Create output directory
mkdir -p "$OUTPUT_DIR"

# Build for macOS ARM64 (Apple Silicon)
echo "Building for macOS ARM64..."
GOOS=darwin GOARCH=arm64 go build -ldflags="${LDFLAGS}" -o "${OUTPUT_DIR}/${APP_NAME}-darwin-arm64" .
echo "  -> ${OUTPUT_DIR}/${APP_NAME}-darwin-arm64"

# Build for macOS AMD64 (Intel)
echo "Building for macOS AMD64..."
GOOS=darwin GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o "${OUTPUT_DIR}/${APP_NAME}-darwin-amd64" .
echo "  -> ${OUTPUT_DIR}/${APP_NAME}-darwin-amd64"

# Build for Windows AMD64
echo "Building for Windows AMD64..."
GOOS=windows GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o "${OUTPUT_DIR}/${APP_NAME}-windows-amd64.exe" .
echo "  -> ${OUTPUT_DIR}/${APP_NAME}-windows-amd64.exe"

# Build for Windows ARM64
echo "Building for Windows ARM64..."
GOOS=windows GOARCH=arm64 go build -ldflags="${LDFLAGS}" -o "${OUTPUT_DIR}/${APP_NAME}-windows-arm64.exe" .
echo "  -> ${OUTPUT_DIR}/${APP_NAME}-windows-arm64.exe"

# Build for Linux AMD64
echo "Building for Linux AMD64..."
GOOS=linux GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o "${OUTPUT_DIR}/${APP_NAME}-linux-amd64" .
echo "  -> ${OUTPUT_DIR}/${APP_NAME}-linux-amd64"

# Build for Linux ARM64
echo "Building for Linux ARM64..."
GOOS=linux GOARCH=arm64 go build -ldflags="${LDFLAGS}" -o "${OUTPUT_DIR}/${APP_NAME}-linux-arm64" .
echo "  -> ${OUTPUT_DIR}/${APP_NAME}-linux-arm64"

echo ""
echo "Build complete!"
echo ""

# Show file sizes
echo "Output files:"
ls -lh "$OUTPUT_DIR"
