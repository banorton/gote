#!/bin/sh
set -e

mkdir -p builds

echo "Building for macOS (amd64)..."
GOOS=darwin GOARCH=amd64 go build -o builds/gote-mac-amd64 ./src/main.go
chmod +x builds/gote-mac-amd64

echo "Building for macOS (arm64)..."
GOOS=darwin GOARCH=arm64 go build -o builds/gote-mac-arm64 ./src/main.go
chmod +x builds/gote-mac-arm64

echo "Building for Linux (amd64)..."
GOOS=linux GOARCH=amd64 go build -o builds/gote-linux-amd64 ./src/main.go
chmod +x builds/gote-linux-amd64

echo "Building for Linux (arm64)..."
GOOS=linux GOARCH=arm64 go build -o builds/gote-linux-arm64 ./src/main.go
chmod +x builds/gote-linux-arm64

echo "Building for Windows (amd64)..."
GOOS=windows GOARCH=amd64 go build -o builds/gote-win.exe ./src/main.go

echo "All builds complete."
