---
title: Development
nav_order: 18
---

# Development

## Layout

```
cmd/proxy/main.go            wiring, signals, Kubernetes client, certificate refresh
cmd/proxy/internal/          config, route, postgres, gateway, protocol, discovery, tlsstore, health
pkg/                         reusable packages: proxy (TCP server, Pipe), tlsutil
kubernetes/                  base, production/nodeport, production/daemonset-hostport, development, test
scripts/                     dev-run.sh, local-deploy.sh, e2e.sh, create-release.sh
docs/                        this site (Jekyll, just-the-docs)
static/images/               banners and screenshots, copied into the site at build time
.github/workflows/           ci.yaml, publish.yaml, pages.yaml
```

## Commands

```bash
go test -race ./...                             # unit and in-process tests
gofmt -l . && go vet ./...                      # format and static checks
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
scripts/dev-run.sh                              # proxy on :6432 against PostgreSQL on localhost:5432
scripts/e2e.sh                                  # real databases in Docker
scripts/local-deploy.sh                         # build and deploy to a local cluster
kubectl kustomize kubernetes/production/nodeport
```

The Go version is set in `go.mod` (currently 1.27.4). CI and the Dockerfile follow it.

## Unit tests

Handler tests use real local sockets and fake backends; discovery tests use the client-go fake clientset. No external service is needed.

| Package | Covers |
| --- | --- |
| config | DATABASE_TYPE names, defaults and validation, SNI_TLS_MODE rules |
| route | Login and host name parsing |
| postgres | Plain and TLS routing, declined SSL, unknown deployment error, SNI after SSLRequest, direct TLS and its rejection without TLS, login wins over SNI, extra startup parameters forwarded |
| gateway | SNI passthrough and terminate, unknown or missing SNI, plaintext refused for SNI-only protocols, Redis AUTH and HELLO AUTH (plain and over terminated TLS), ClickHouse Hello, routing label fallbacks and precedence, Redis error replies |
| protocol | Redis and ClickHouse parsing, rejection of unroutable or oversized input, pipelined bytes kept |
| discovery | Static table, informer add and delete, routing label, missing pooled label, ambiguous route warning, unknown destination port |
| tlsstore | Generate once and reuse, renew own expiring certificate, no auto generation, file round trip |
| pkg/proxy | Connection limit, shutdown timeout, accept errors do not stop the server |

## End-to-end tests with real databases

`scripts/e2e.sh` needs only Docker, Go and openssl. It builds the proxy for the Docker host, creates a throw-away network and, per database, starts the server, the proxy (the binary mounted into a `busybox` container) and runs the database's own CLI in a client container.

| Set | Image (default) | Checks |
| --- | --- | --- |
| postgres | postgres:17-alpine | login routing and user rewrite over TLS and plaintext, pooled route, database default, TLS really negotiated, direct TLS, unknown and missing deployment errors, large result |
| redis | redis:7-alpine | AUTH routing and rewrite, SET/GET, pooled route, TLS terminated by the proxy, HELLO 3 AUTH, unknown deployment error, pipelined commands |
| clickhouse | clickhouse/clickhouse-server:latest-alpine | Hello routing and rewrite, query, pooled route, large result, unknown deployment |
| mongodb | mongo:8 | SNI passthrough, pooled host, insert and read, unknown host refused, SNI terminate with a plain mongod |

```bash
scripts/e2e.sh                                  # all sets
E2E_ONLY="postgres redis" scripts/e2e.sh        # a subset
E2E_PG_IMAGE=postgres:18 E2E_REDIS_IMAGE=redis:latest E2E_PROXY_IMAGE=ubuntu:22.04 scripts/e2e.sh
```

Image variables: `E2E_PROXY_IMAGE`, `E2E_PG_IMAGE` (17 or newer for direct TLS), `E2E_REDIS_IMAGE`, `E2E_CH_IMAGE`, `E2E_MONGO_IMAGE`. Everything is removed at the end, also on failure. CI runs each set as its own matrix job.

## CI

`ci.yaml` runs on pushes and pull requests to main and development, and is called by `publish.yaml` for tags. Jobs run in parallel:

| Job | Does |
| --- | --- |
| test | gofmt, `go mod verify`, vet, race tests with coverage |
| vulncheck | govulncheck |
| manifests | Kustomize rendering of every variant |
| image | Binaries built on the runner, packaged with `--build-arg SRC=prebuilt`, Trivy scan |
| e2e | `scripts/e2e.sh`, one job per database |

`docker build .` without arguments compiles inside Docker and is what users run locally; CI does not exercise that path.

## Release

1. Move the Unreleased entries in `CHANGELOG.md` under a new version heading, for example `## [2.2.0] - 2026-12-01`, on main.
2. Run `scripts/create-release.sh 2.2.0` on a clean main, or tag `v2.2.0` and push the tag yourself.
3. `publish.yaml` runs the whole CI first. Only if it passes does it build linux/amd64 and linux/arm64 images, push `ghcr.io/hasirciogluhq/xdatabase-proxy:2.2.0` and `:2.2` (and `latest`) with SBOM and provenance, and create the GitHub release from that CHANGELOG section.

If CI fails nothing is published: fix the problem, delete the tag (`git push --delete origin v2.2.0`) and tag again. Only tags publish images; branch pushes run CI only.

## Branches

`development` is the working branch; `main` is released. Merge development into main for a release.

## Documentation

This site builds from `docs/` with GitHub Pages. `pages.yaml` deploys it on every push to main that touches `docs/`, `static/images/` or the workflow. Pages source must be GitHub Actions (one-time setting).

Rules for every change:

1. Update the page in `docs/` that describes the behaviour.
2. Add a line under Unreleased in `CHANGELOG.md`.
3. Update `AGENTS.md` if layout, commands, variables or rules changed.
4. Add or update tests, and an e2e check when routing behaviour changes.

Preview locally:

```bash
cd docs && cp -r ../static/images assets/images
docker run --rm -p 4000:4000 -v "$PWD:/srv/jekyll" jekyll/jekyll:4 jekyll serve --host 0.0.0.0
```

## Adding a database protocol

If the login arrives in the first client message, implement the gateway `Login` interface in `cmd/proxy/internal/protocol`:

```go
Read(r *bufio.Reader) (user string, forward func(plain string) []byte, err error)
// optional, for protocol-level error replies:
WriteError(w io.Writer, msg string)
```

Bound every size, then plug it into `gateway.Handler` in `main.go`. If the protocol needs its own negotiation, write a handler that implements `pkg/proxy.Handler` next to `postgres`. In both cases use `route.Resolver`, accept the new `DATABASE_TYPE` in `config`, add its default strategies to `defaultRouting` in `main.go`, add unit tests and an `e2e.sh` set, and document it under Databases and Configuration.
