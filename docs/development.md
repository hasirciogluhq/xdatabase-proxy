---
title: Development
nav_order: 12
---

# Development

## Layout

```
cmd/proxy/            main and internal packages
pkg/                  reusable packages (proxy, tlsutil)
kubernetes/           base, production variants, development, test
scripts/              dev-run.sh, local-deploy.sh, create-release.sh, e2e.sh
docs/                 this site
static/images/        banners and screenshots
.github/workflows/    ci, publish, pages
```

## Commands

```bash
go test -race ./...            # unit and in-process end-to-end tests
scripts/dev-run.sh             # run against local PostgreSQL
scripts/local-deploy.sh        # build and deploy to a local cluster
kubectl kustomize kubernetes/production/nodeport
```

The Go version is set in go.mod. CI and the Dockerfile follow it.

## Tests

- config: DATABASE_TYPE names, defaults and validation, SNI_TLS_MODE rules.
- route: login and host name parsing.
- postgres: routing over plain TCP, declined SSL, real TLS, error for unknown deployment, SNI after SSLRequest, direct TLS (and its rejection without TLS), login wins over SNI, extra startup parameters forwarded.
- gateway: SNI passthrough and terminate, unknown or missing SNI, plaintext refused for SNI-only protocols, Redis AUTH and HELLO AUTH (plain and over terminated TLS), ClickHouse Hello, routing label fallbacks, Redis error replies.
- protocol: Redis and ClickHouse login parsing, rejection of unroutable or oversized input, pipelined bytes kept.
- discovery: static table, Kubernetes informer with a fake clientset including add and delete, routing label, missing pooled label, ambiguous route warning, unknown destination port.
- tlsstore: generate once, renew, no auto generate, file round trip.
- pkg/proxy: connection limit, shutdown timeout, accept errors.

Add a test for every behaviour change. Handler tests use real sockets and a fake backend, so no external services are needed.

## End-to-end tests with real databases

`scripts/e2e.sh` needs only Docker and Go. It builds the proxy for the Docker host, creates a throw-away network and, per database, starts the server, the proxy (the binary mounted into a tiny container) and runs the database's own CLI in a client container: psql, redis-cli, clickhouse-client, mongosh. It checks routing, user rewriting, TLS, pooled routes and error cases.

```bash
scripts/e2e.sh                                  # all databases
E2E_ONLY="postgres redis" scripts/e2e.sh        # a subset
E2E_PG_IMAGE=postgres:18 E2E_REDIS_IMAGE=redis:latest E2E_PROXY_IMAGE=ubuntu:22.04 scripts/e2e.sh   # reuse local images, no pulls
```

Local results (Docker on macOS arm64, amd64 images under emulation): PostgreSQL 18 9/9, Redis 8/8, ClickHouse 5/5, MongoDB 8 4/4 checks passed, plus unit tests with the race detector and govulncheck.

Image variables: `E2E_PROXY_IMAGE`, `E2E_PG_IMAGE` (17 or newer for direct TLS), `E2E_REDIS_IMAGE`, `E2E_CH_IMAGE`, `E2E_MONGO_IMAGE`. Everything is removed at the end, also on failure. CI runs the same script as a matrix job per database on every push.

## CI

ci.yaml runs on pushes and pull requests to main and development (and is called by publish.yaml for tags), with these jobs in parallel: test (gofmt, go mod verify, vet, race tests with coverage), vulncheck (govulncheck), manifests (Kustomize rendering of all variants), image (binary built on the runner, packaged with `--build-arg SRC=prebuilt`, Trivy scan with cached database) and e2e (scripts/e2e.sh, one parallel job per database: PostgreSQL, Redis, ClickHouse, MongoDB). `docker build .` without arguments still compiles inside Docker and is what users run locally; CI does not exercise that path.

## Release

1. Move the Unreleased entries in CHANGELOG.md under a new version heading.
2. Tag the commit with v and the version, for example v2.1.2, and push the tag.
3. publish.yaml first runs the whole CI (tests, govulncheck, manifests, image scan, end-to-end). Only if it passes does it build linux/amd64 and linux/arm64 images, pushes them to ghcr.io with SBOM and provenance, and creates the GitHub release with the notes of that CHANGELOG section.

If CI fails, nothing is published: fix the problem, delete the tag (git push --delete origin TAG) and tag again.

Only tags publish images. Pushes to main and development run CI but publish nothing.

## Documentation

This site builds from docs/ with GitHub Pages. pages.yaml deploys it on every push to main that touches docs, static/images or the workflow. Enable it once under Settings, Pages, Source: GitHub Actions.

Rules for every change:

1. Update the page in docs/ that describes the behaviour.
2. Add a line under Unreleased in CHANGELOG.md.
3. Update AGENTS.md if layout, commands, variables or rules changed.
4. Add or update tests.

## Adding a database protocol

If the login is in the first client message, implement the gateway Login interface in cmd/proxy/internal/protocol (read the user, return a function that re-encodes the message with the plain user, optionally WriteError) and plug it into gateway.Handler. Otherwise create a package under cmd/proxy/internal with a handler that implements pkg/proxy Handler. Either way use route.Resolver, accept the DATABASE_TYPE in config, add the default strategies in main.go, add a check to scripts/e2e.sh and document it in Routing strategies and Configuration.
