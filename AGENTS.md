AGENTS.md

Guide for AI coding agents working on this repository. Read this first. It is short on purpose. Details live in the docs folder.

PROJECT

XDatabase Proxy is a PostgreSQL connection router written in Go. A client logs in as user.deployment or user.deployment.pool. The proxy reads that name, finds the backend address, terminates TLS, rewrites the startup message to the plain user name and then copies bytes both ways. It does not parse SQL and does not authenticate users. Login based routing supports only PostgreSQL. Other TLS-first databases use SNI routing. MySQL is not supported. PostgreSQL-compatible products use the PostgreSQL handler.

Module path: github.com/hasirciogluhq/xdatabase-proxy. The Go version is the one in go.mod.

LAYOUT

cmd/proxy/main.go wires everything, handles signals, builds the Kubernetes client.
cmd/proxy/internal/config reads environment variables and validates them.
cmd/proxy/internal/postgres is the connection handler: startup negotiation, user parsing, dial, pipe.
cmd/proxy/internal/route holds the routing key, strategies (username, sni) and the resolver contract.
cmd/proxy/internal/gateway routes Redis, ClickHouse, MongoDB and generic TLS connections: SNI passthrough or username routing.
cmd/proxy/internal/protocol reads the login of Redis and ClickHouse from the first client message.
cmd/proxy/internal/discovery has the static table and the Kubernetes informer resolver.
cmd/proxy/internal/tlsstore stores certificates in a file, a Kubernetes Secret or memory.
cmd/proxy/internal/health serves health and ready.
pkg/proxy is a generic TCP server with a connection limit, graceful shutdown and a pipe helper.
pkg/tlsutil creates and inspects self-signed certificates.
kubernetes holds Kustomize bases: base, production/nodeport, production/daemonset-hostport, development, test.
scripts holds dev-run.sh, local-deploy.sh, create-release.sh and e2e.sh (real databases in Docker, queried with their own CLIs).
docs is the documentation site, built with Jekyll and just-the-docs.
static/images holds banners and a screenshot.
.github/workflows holds ci.yaml, publish.yaml and pages.yaml.

Rule: pkg contains code that knows nothing about this product. cmd/proxy/internal contains product logic. Do not import internal from pkg.

WHERE TO READ MORE

Overview: docs/index.md
Routing strategies per database and the Service label: docs/routing-strategies.md
How a connection flows: docs/architecture.md
Login format and errors: docs/connection-routing.md
Every environment variable: docs/configuration.md
Labels and discovery rules: docs/service-discovery.md
Certificates: docs/tls.md
Deploy targets: docs/deployment/index.md, docs/deployment/kubernetes.md, docs/deployment/container-and-vm.md
Typical setups: docs/use-cases.md
Threat model and hardening: docs/security.md
Health, logs, troubleshooting: docs/operations.md
Tests, CI, release, doc rules: docs/development.md
History of changes: CHANGELOG.md
Contribution process: CONTRIBUTING.md. Conduct: CODE_OF_CONDUCT.md. Vulnerability reporting: SECURITY.md. Help: SUPPORT.md. License: LICENSE (MIT).

COMMANDS

Run all tests with the race detector: go test -race ./...
Format check: gofmt -l .
Static checks: go vet ./...
Run locally: scripts/dev-run.sh
End-to-end tests with real databases in Docker: scripts/e2e.sh (E2E_ONLY selects postgres, redis, clickhouse, mongodb; see docs/development.md for image variables)
Render a manifest variant: kubectl kustomize kubernetes/production/nodeport
Vulnerability scan: go run golang.org/x/vuln/cmd/govulncheck@latest ./...

CODING RULES

Write the simplest code that works. Prefer the standard library. Use maintained libraries for protocols and Kubernetes instead of writing them again: pgx pgproto3 for the PostgreSQL wire format, client-go informers for watching Services. Do not write watch loops, wire parsers or YAML parsers by hand.
Add a dependency only when it removes real code and has a good maintenance record. Keep go.mod small and run go mod tidy.
No factory or strategy layers without a second implementation that needs them. Interfaces are defined by the consumer.
Log with log/slog. Never log startup parameters, passwords or backend addresses to clients.
Treat everything a client sends before login as hostile: bound sizes, set deadlines, cap loops.
Match the style of the surrounding code and keep comments short and useful.

BEHAVIOUR TO PRESERVE

Routing strategies: username and sni. The Service label xdatabase-proxy-routing (username, sni or all) decides which may reach a Service; without it the defaults in main.go apply (postgresql username, redis and clickhouse both, mongodb and tls sni). PostgreSQL always reads the username; the TLS server name is used only if the login has no deployment and the label enables sni. Gateway order for TLS: enabled SNI route first (passthrough), else terminate TLS and route by username. Redis needs AUTH or HELLO AUTH as first command. Direct TLS (first byte 0x16, ALPN postgresql) is supported. DATABASE_TYPE names of PostgreSQL-compatible products map to postgresql.

Last dot separated part of the user is the deployment, unless it is the word pool, then the part before it is. The proxy sends the plain user to the backend.
If database is empty or equals the full login, the backend gets postgres.
Kubernetes Services are routable only with labels xdatabase-proxy-enabled true, xdatabase-proxy-database-type postgresql and xdatabase-proxy-deployment-id. Label xdatabase-proxy-pooled defaults to false.
The proxy never overwrites a certificate it did not create.
Shutdown drains for 25 seconds. Pod grace period must stay longer.
Configuration is environment variables only. Removed variables are listed in docs/configuration.md.

TESTING

Every behaviour change needs a test. Handler tests use real local sockets and a fake backend. Discovery tests use the client-go fake clientset. Do not add tests that need external services to go test. Real database tests live in scripts/e2e.sh and run in the e2e matrix job of ci.yaml. Add a check there when routing behaviour changes. Use the latest major versions of GitHub Actions.

WORKFLOW RULES FOR EVERY CHANGE

1. Update the matching page in docs.
2. Add an entry under Unreleased in CHANGELOG.md in the right section: Added, Changed, Fixed, Removed or Security. Mention breaking changes clearly.
3. Update this file when layout, commands, variables or rules change.
4. Run tests, vet and gofmt before finishing.
5. Do not commit or push unless the user asks.

Branches: development is the working branch, main is released. Tags starting with v publish a release image and a GitHub release.
Image names in static/images are banner-en.jpeg, banner-tr.jpeg, banner-default.jpeg and psql-tls-connection.png. Use descriptive lowercase names with hyphens for new files.

PUBLISHING

docs are deployed to GitHub Pages by pages.yaml on pushes to main. The repository setting Pages source must be GitHub Actions. This is a one-time manual step.

KNOWN LIMITS

CancelRequest is not routed. The proxy to backend hop is plain TCP. Self-signed certificates do not pass verify-full.
