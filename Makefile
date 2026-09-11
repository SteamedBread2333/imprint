.PHONY: build test install dist publish clean

CGO_ENABLED ?= 0
GOFLAGS ?= -trimpath
LDFLAGS ?= -s -w
VERSION ?= 1.0.0

build:
	CGO_ENABLED=$(CGO_ENABLED) go build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o imprint ./cmd/imprint

test:
	go test ./...

install:
	CGO_ENABLED=$(CGO_ENABLED) go install $(GOFLAGS) -ldflags="$(LDFLAGS)" ./cmd/imprint

dist:
	VERSION=$(VERSION) LDFLAGS="$(LDFLAGS)" bash scripts/dist.sh

publish:
	bash scripts/publish.sh $(VERSION)

clean:
	rm -rf imprint imprint.exe dist
