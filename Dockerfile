# syntax=docker/dockerfile:1
#
# Default: compile inside Docker (docker build .).
# CI: pass --build-arg SRC=prebuilt with binaries in bin/linux-<arch>/xdatabase-proxy
# (built on the runner with a warm Go cache, much faster than compiling here).
ARG SRC=build

FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download && go mod verify
COPY . .
ARG TARGETOS TARGETARCH VERSION=dev
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w -X main.version=$VERSION" -o /out/xdatabase-proxy ./cmd/proxy

FROM scratch AS prebuilt
ARG TARGETARCH
COPY bin/linux-${TARGETARCH}/xdatabase-proxy /out/xdatabase-proxy

FROM ${SRC} AS bin

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=bin /out/xdatabase-proxy /xdatabase-proxy
EXPOSE 5432 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s CMD ["/xdatabase-proxy", "-healthcheck"]
ENTRYPOINT ["/xdatabase-proxy"]
