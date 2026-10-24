# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
ARG TARGETOS TARGETARCH VERSION=dev
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w -X main.version=$VERSION" -o /out/xdatabase-proxy ./cmd/proxy

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/xdatabase-proxy /xdatabase-proxy
EXPOSE 5432 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s CMD ["/xdatabase-proxy", "-healthcheck"]
ENTRYPOINT ["/xdatabase-proxy"]
