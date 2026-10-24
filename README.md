# XDatabase Proxy

![xdatabase-proxy banner](static/images/banner-en.jpeg)

A small, dependency-light PostgreSQL connection router. Clients connect to one endpoint and the proxy picks the backend from the **username**, terminating TLS itself. It runs in Kubernetes, containers, VMs and bare metal.

```
postgresql://user.<deployment>[.pool]@proxy:5432/db
```

- `user.db-prod` → service of deployment `db-prod` (direct)
- `user.db-prod.pool` → the pooled service (pgbouncer, odyssey, …) of the same deployment

The proxy strips the routing suffix before forwarding, so the backend sees plain `user`. If `database` is empty or equals the full login (psql's default), it becomes `postgres`.

Full documentation: https://hasirciogluhq.github.io/xdatabase-proxy/ (sources in [docs/](docs)).

## Supported databases

| Database | Status | Notes |
| --- | --- | --- |
| PostgreSQL | ✅ Supported | Routing by `user.deployment[.pool]` |
| PostgreSQL-compatible (CockroachDB, YugabyteDB, TimescaleDB, Citus, Greenplum, QuestDB, CrateDB, Aurora/AlloyDB PostgreSQL) | 🧪 Should work, untested | Same wire protocol, use `DATABASE_TYPE=postgresql` |
| MongoDB, Redis, ClickHouse (native) | 📋 Planned | Needs TLS SNI routing, since their login arrives after the connection is open |
| MySQL / MariaDB | 📋 Planned, hardest | The server speaks first and the password scramble is bound to the backend |

### Roadmap, easiest first

1. **PostgreSQL-compatible databases**: no code needed, only test and document them.
2. **TLS SNI routing** (`deployment.proxy.example.com`): one generic handler that reads the SNI from the TLS ClientHello and forwards bytes. It unlocks every TLS-capable protocol (MongoDB, Redis, ClickHouse, …) and also works for PostgreSQL. This is the best next step.
3. **MySQL / MariaDB**: the proxy must answer the greeting itself and handle authentication, so it is a much larger change.

## How it is built

| Concern | Implementation |
| --- | --- |
| Wire protocol | [`pgx/pgproto3`](https://github.com/jackc/pgx) |
| Kubernetes | `client-go` shared informer with a route index (no polling, O(1) lookup) |
| Logging | `log/slog` (JSON) |
| TLS | `crypto/tls`, self-signed ECDSA P-256 generated on demand, TLS ≥ 1.2 |

Layout: `pkg/` holds reusable pieces (`proxy` TCP server/pipe, `tlsutil`); `cmd/proxy/internal/` holds project-specific code (`config`, `discovery`, `tlsstore`, `postgres`, `health`).

## Deployment

All manifests are Kustomize bases under [`kubernetes/`](kubernetes):

| Path | What it deploys |
| --- | --- |
| `kubernetes/base` | Namespace, ServiceAccount, least-privilege RBAC (`list/watch services`, own TLS secret only) |
| `kubernetes/production/nodeport` | 3-replica Deployment, `NodePort` Service (30432, `externalTrafficPolicy: Local`), PDB |
| `kubernetes/production/daemonset-hostport` | DaemonSet, one proxy per node on `hostPort 5432`, no Service hop |
| `kubernetes/development` | 1 replica, debug logs, in-memory TLS, `-development` image |
| `kubernetes/test` | Throw-away PostgreSQL with the discovery labels |

```bash
kubectl apply -k kubernetes/production/nodeport          # or .../daemonset-hostport
kubectl apply -k kubernetes/development && kubectl apply -n xdatabase-proxy -f kubernetes/test/postgresql.yaml
```

Pin the image to a version tag or digest in production. Images are multi-arch, distroless, non-root, and published with SBOM and provenance attestations.

**Container / VM** (static routing, no Kubernetes needed):

```bash
docker run -p 5432:5432 -e STATIC_BACKENDS="db1=10.0.0.5:5432,db1.pool=10.0.0.5:6432" \
  ghcr.io/hasirciogluhq/xdatabase-proxy:latest
```

For Kubernetes discovery outside a cluster set `KUBECONFIG` (and optionally `KUBE_CONTEXT`).

## Service discovery (Kubernetes)

Label a Service and it becomes routable:

| Label | Meaning |
| --- | --- |
| `xdatabase-proxy-enabled: "true"` | opt in |
| `xdatabase-proxy-database-type: postgresql` | protocol |
| `xdatabase-proxy-deployment-id: <id>` | routing key |
| `xdatabase-proxy-pooled: "true"` | pooled endpoint (default `false`) |
| `xdatabase-proxy-destination-port: <port or name>` | optional, defaults to the first port |

If several Services share a route, the one with the lowest `namespace/name` wins.

## Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `PROXY_START_PORT` | `5432` | Listener port |
| `HEALTH_SERVER_PORT` | `8080` | `/health` (liveness), `/ready` (readiness) |
| `DEBUG` | `false` | Debug logging |
| `MAX_CONNECTIONS` | `10000` | Concurrent client limit (`0` = unlimited) |
| `DIAL_TIMEOUT_SECONDS` | `5` | Backend resolve + dial timeout |
| `DISCOVERY_MODE` | auto | `static` if `STATIC_BACKENDS` is set, else `kubernetes` |
| `STATIC_BACKENDS` | – | `deployment[.pool]=host:port,…` |
| `WATCH_NAMESPACE` | all | Restrict discovery to one namespace (then a Role is enough) |
| `CLUSTER_DOMAIN` | `cluster.local` | Used to build service DNS names |
| `KUBECONFIG`, `KUBE_CONTEXT` | – | Kubernetes access outside a pod |
| `NAMESPACE` | pod namespace | Namespace of the TLS secret |
| `TLS_ENABLED` | `true` | Offer TLS to clients (`SSLRequest`) |
| `TLS_MODE` | auto | `file` (if `TLS_CERT_FILE`), `kubernetes` (if `TLS_SECRET_NAME`), else `memory` |
| `TLS_CERT_FILE`, `TLS_KEY_FILE` | – | Certificate files |
| `TLS_SECRET_NAME` | – | `kubernetes.io/tls` Secret (created if missing) |
| `TLS_AUTO_GENERATE` | `true` | Generate a self-signed cert when none exists |
| `TLS_RENEWAL_THRESHOLD_DAYS` | `30` | Re-issue **our own** self-signed cert this long before expiry; certificates you provide are never replaced |

Certificates are re-checked every 6 hours and hot-swapped, so cert-manager rotations need no restart.

## Hardening

- Startup packets are size-limited by `pgproto3` (10 kB), startup has a 10 s deadline, negotiation attempts are capped (no recursion).
- Connection cap, graceful shutdown (drains for 25 s on SIGTERM), health goes unready first.
- Startup parameters and passwords are never logged.
- Minimal dependency set, `go mod verify`, `govulncheck` and Trivy in CI, Dependabot for Go modules, actions and base image.
- Pods run non-root with a read-only filesystem and all capabilities dropped.
- `CancelRequest` is not routable (it carries no user); clients simply fall back to their own timeout.

## Development

```bash
go test -race ./...
scripts/dev-run.sh          # proxy on :6432 → postgres on localhost:5432
scripts/local-deploy.sh     # build, load and deploy to a local cluster
```

CI (`.github/workflows/ci.yaml`): format, vet, race tests, govulncheck, manifest rendering, image build + scan, and an end-to-end `psql` run against a real PostgreSQL. Pushing a `v*` tag runs `publish.yaml`: multi-arch image, SBOM + provenance, GitHub release from `CHANGELOG.md`.

## License

MIT, see [LICENSE](LICENSE). Contributing: [CONTRIBUTING.md](CONTRIBUTING.md). Conduct: [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md). Security: [SECURITY.md](SECURITY.md). Help: [SUPPORT.md](SUPPORT.md). Issues: https://github.com/hasirciogluhq/xdatabase-proxy/issues
