# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
	go build -trimpath -ldflags="-s -w" -o /out/imprint ./cmd/imprint

FROM scratch
COPY --from=build /out/imprint /imprint
ENTRYPOINT ["/imprint"]
