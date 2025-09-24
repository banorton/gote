#!/bin/sh
set -e

mkdir -p builds

echo "Building for macOS (darwin)..."
GOOS=darwin GOARCH=amd64 go build -o builds/gote-mac ./src/main.go
chmod +x builds/gote-mac

echo "Building for Linux..."
GOOS=linux GOARCH=amd64 go build -o builds/gote-linux ./src/main.go
chmod +x builds/gote-linux

echo "Building for Windows..."
GOOS=windows GOARCH=amd64 go build -o builds/gote-win.exe ./src/main.go

echo "All builds complete."
