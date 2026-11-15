VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/banorton/gote/internal/cli.Version=$(VERSION)

.PHONY: build test test-linux cross web web-check e2e

web:
	cd web && npm ci && node build.mjs

web-check: web
	git diff --exit-code --stat -- web/dist
	git ls-files --others --exclude-standard web/dist | (! grep .)

build:
	CGO_ENABLED=0 go build -ldflags '$(LDFLAGS)' -o gote ./cmd/gote

test:
	CGO_ENABLED=0 go test ./...

e2e: web-deps
	CGO_ENABLED=0 go build -o gote ./cmd/gote
	cd web && npx playwright install chromium && npx playwright test -c e2e/playwright.config.mjs

web-deps:
	cd web && npm ci

cross:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags '$(LDFLAGS)' -o builds/gote-darwin-amd64 ./cmd/gote
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags '$(LDFLAGS)' -o builds/gote-darwin-arm64 ./cmd/gote
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags '$(LDFLAGS)' -o builds/gote-linux-amd64 ./cmd/gote
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags '$(LDFLAGS)' -o builds/gote-linux-arm64 ./cmd/gote

# Full suite on Linux (inotify, xdg paths) with networking disabled.
test-linux:
	docker volume create gote-gomod >/dev/null
	docker run --rm -v gote-gomod:/go/pkg/mod -v "$(CURDIR)":/src:ro -w /src golang:1.26 go mod download
	docker run --rm --network none -e CGO_ENABLED=0 -v gote-gomod:/go/pkg/mod -v "$(CURDIR)":/ro:ro golang:1.26 \
		bash -c 'cp -r /ro /src && cd /src && go test -count=1 -timeout 600s ./...'
