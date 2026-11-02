# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=devel
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
	go build -trimpath -ldflags="-s -w -X github.com/SteamedBread2333/imprint/pkg/imprint.Version=${VERSION}" \
	-o /out/ ./cmd/imprint ./cmd/imprint-mcp

# Default entrypoint is the CLI; run the MCP server with --entrypoint /imprint-mcp.
FROM scratch
COPY --from=build /out/imprint /out/imprint-mcp /
ENTRYPOINT ["/imprint"]
