.PHONY: build test install dist publish demo-seed clean

CGO_ENABLED ?= 0
GOFLAGS ?= -trimpath
MODULE := github.com/SteamedBread2333/imprint
# Git tag is the single source of truth. Override with VERSION=X.Y.Z.
VERSION ?= $(shell bash scripts/release/version.sh)
LDFLAGS ?= -s -w -X $(MODULE)/pkg/imprint.Version=$(VERSION)

build:
	CGO_ENABLED=$(CGO_ENABLED) go build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o imprint ./cmd/imprint
	CGO_ENABLED=$(CGO_ENABLED) go build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o imprint-mcp ./cmd/imprint-mcp

test:
	go test ./...

install:
	CGO_ENABLED=$(CGO_ENABLED) go install $(GOFLAGS) -ldflags="$(LDFLAGS)" ./cmd/imprint ./cmd/imprint-mcp

dist:
	VERSION=$(VERSION) LDFLAGS="$(LDFLAGS)" bash scripts/release/dist.sh

publish:
	@if [ -z "$(V)" ]; then echo "usage: make publish V=X.Y.Z"; exit 2; fi
	bash scripts/release/publish.sh $(V)

demo-seed:
	go run ./scripts/demo/seed-vault

clean:
	rm -rf imprint imprint.exe imprint-mcp imprint-mcp.exe dist
