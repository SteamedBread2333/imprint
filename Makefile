.PHONY: build test install dist publish clean

CGO_ENABLED ?= 0
GOFLAGS ?= -trimpath
MODULE := github.com/SteamedBread2333/imprint
# Git tag is the single source of truth. Override with VERSION=X.Y.Z.
VERSION ?= $(shell bash scripts/version.sh)
LDFLAGS ?= -s -w -X $(MODULE)/pkg/imprint.Version=$(VERSION)

build:
	CGO_ENABLED=$(CGO_ENABLED) go build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o imprint ./cmd/imprint

test:
	go test ./...

install:
	CGO_ENABLED=$(CGO_ENABLED) go install $(GOFLAGS) -ldflags="$(LDFLAGS)" ./cmd/imprint

dist:
	VERSION=$(VERSION) LDFLAGS="$(LDFLAGS)" bash scripts/dist.sh

publish:
	@if [ -z "$(V)" ]; then echo "usage: make publish V=X.Y.Z"; exit 2; fi
	bash scripts/publish.sh $(V)

clean:
	rm -rf imprint imprint.exe dist
