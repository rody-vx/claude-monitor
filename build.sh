#!/bin/bash

# Claude Monitor Build Script
# Builds binaries for macOS (Intel/ARM) and Windows

set -e

VERSION="1.0.0"
OUTPUT_DIR="dist"
APP_NAME="claude-monitor"

echo "Building Claude Monitor v${VERSION}..."
echo ""

# Create output directory
mkdir -p "$OUTPUT_DIR"

# Build for macOS ARM64 (Apple Silicon)
echo "Building for macOS ARM64..."
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o "${OUTPUT_DIR}/${APP_NAME}-darwin-arm64" .
echo "  -> ${OUTPUT_DIR}/${APP_NAME}-darwin-arm64"

# Build for macOS AMD64 (Intel)
echo "Building for macOS AMD64..."
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o "${OUTPUT_DIR}/${APP_NAME}-darwin-amd64" .
echo "  -> ${OUTPUT_DIR}/${APP_NAME}-darwin-amd64"

# Build for Windows AMD64
echo "Building for Windows AMD64..."
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o "${OUTPUT_DIR}/${APP_NAME}-windows-amd64.exe" .
echo "  -> ${OUTPUT_DIR}/${APP_NAME}-windows-amd64.exe"

# Build for Windows ARM64
echo "Building for Windows ARM64..."
GOOS=windows GOARCH=arm64 go build -ldflags="-s -w" -o "${OUTPUT_DIR}/${APP_NAME}-windows-arm64.exe" .
echo "  -> ${OUTPUT_DIR}/${APP_NAME}-windows-arm64.exe"

echo ""
echo "Build complete!"
echo ""

# Show file sizes
echo "Output files:"
ls -lh "$OUTPUT_DIR"
