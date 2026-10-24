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
scripts/              dev-run.sh, local-deploy.sh, create-release.sh
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

- postgres: routing over plain TCP, declined SSL, real TLS, error for unknown deployment, user parsing.
- discovery: static table, Kubernetes informer with a fake clientset including add and delete.
- tlsstore: generate once, renew, no auto generate, file round trip.

Add a test for every behaviour change. Handler tests use real sockets and a fake backend, so no external services are needed.

## CI

ci.yaml runs on every push and pull request, with these jobs in parallel: test (gofmt, go mod verify, vet, race tests with coverage), vulncheck (govulncheck), manifests (Kustomize rendering of all variants), image (binary built on the runner, packaged with `--build-arg SRC=prebuilt`, Trivy scan with cached database) and e2e (psql against a real PostgreSQL service). `docker build .` without arguments still compiles inside Docker and is what users run locally; CI does not exercise that path.

## Release

1. Move the Unreleased entries in CHANGELOG.md under a new version heading.
2. Tag the commit with v and the version, for example v2.1.0, and push the tag.
3. publish.yaml first runs the whole CI (tests, govulncheck, manifests, image scan, end-to-end). Only if it passes does it build linux/amd64 and linux/arm64 images, pushes them to ghcr.io with SBOM and provenance, and creates the GitHub release with the notes of that CHANGELOG section.

If CI fails, nothing is published: fix the problem, delete the tag (git push --delete origin TAG) and tag again.

Branch builds: development publishes xdatabase-proxy-development:development, main publishes xdatabase-proxy:main.

## Documentation

This site builds from docs/ with GitHub Pages. pages.yaml deploys it on every push to main that touches docs, static/images or the workflow. Enable it once under Settings, Pages, Source: GitHub Actions.

Rules for every change:

1. Update the page in docs/ that describes the behaviour.
2. Add a line under Unreleased in CHANGELOG.md.
3. Update AGENTS.md if layout, commands, variables or rules changed.
4. Add or update tests.

## Adding a database protocol

Create a package under cmd/proxy/internal with a handler that implements pkg/proxy Handler, reuse the discovery Resolver shape (deployment and pooled flag to address), wire it in main.go and document it here and in Configuration.
