VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/banorton/gote/internal/cli.Version=$(VERSION)

.PHONY: build test cross

build:
	CGO_ENABLED=0 go build -ldflags '$(LDFLAGS)' -o gote ./cmd/gote

test:
	CGO_ENABLED=0 go test ./...

cross:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags '$(LDFLAGS)' -o builds/gote-darwin-amd64 ./cmd/gote
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags '$(LDFLAGS)' -o builds/gote-darwin-arm64 ./cmd/gote
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags '$(LDFLAGS)' -o builds/gote-linux-amd64 ./cmd/gote
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags '$(LDFLAGS)' -o builds/gote-linux-arm64 ./cmd/gote
