---
title: Operations
nav_order: 13
---

# Operations

## Health endpoints

Served on `HEALTH_SERVER_PORT` (default 8080), plain HTTP.

| Path | 200 body | Meaning |
| --- | --- | --- |
| /health | `ok` | The process is alive |
| /ready | `ready` | Startup finished (configuration, Kubernetes informer synced, certificate loaded, listener open). Otherwise `503 not ready` |

The binary has a built-in check: `/xdatabase-proxy -healthcheck` requests `http://127.0.0.1:$HEALTH_SERVER_PORT/health` and exits 0 or 1. It is the Docker HEALTHCHECK, so `docker ps` shows the state.

## Startup sequence

1. Read and validate configuration (exit 1 on error).
2. Start the health server; `/ready` returns 503.
3. Connect to the Kubernetes API if needed and wait for the Service informer to sync.
4. Load or generate the certificate if TLS is enabled.
5. Open the listener, set `/ready` to 200, log `listening`.

If any step fails the process exits with `fatal` in the log, and the orchestrator restarts it.

## Shutdown

On SIGTERM or SIGINT:

1. The listener closes at once; new connections are refused.
2. Running sessions get 25 seconds to finish.
3. Remaining sessions are closed (`shutdown timeout, closing remaining connections`).
4. `/ready` turns to 503, the health server stops, `stopped` is logged.

`/ready` keeps answering 200 during the drain, so take the instance out of rotation by the listener closing (Kubernetes removes the endpoint when the pod starts terminating anyway). Keep the termination grace period above 25 seconds; the manifests use 40.

## Logs

JSON on standard output through `log/slog`, one object per line:

```json
{"time":"2026-11-18T10:00:00Z","level":"WARN","msg":"resolve failed","remote":"10.0.0.7:51234","deployment":"orders","pooled":false,"via":"username","error":"no service for deployment \"orders\" (pooled=false)"}
```

| Field | Meaning |
| --- | --- |
| remote | Client address |
| via | Strategy used: `username` or `sni` |
| deployment, pooled | Route key |
| backend | Resolved address |
| error | Reason |

Levels: INFO for lifecycle, WARN for routing problems and degraded operation, ERROR for backend dial failures, DEBUG for every session start and client side failure. `DEBUG=true` is safe in production for troubleshooting: startup parameters and passwords are never logged.

The ambiguous route warning from Kubernetes discovery is written through the default logger, so it appears on standard error in text form. Full list: [Log messages](reference/log-messages.md).

## Monitoring

There is no metrics endpoint. Useful signals:

- probe `/ready` and the client port from your monitoring system;
- count `resolve failed`, `dial failed` and `connection limit reached` in the logs;
- connection counts and bytes from the load balancer, kube-proxy or node exporter;
- certificate expiry of the Secret or files with your usual tooling.

## Upgrades

- Deployment: rolling update with `maxUnavailable: 0`. Sessions on old pods continue for up to 25 seconds after the pod is told to stop, then are closed. Clients reconnect.
- DaemonSet: one node at a time.
- A new version only needs the new image. Read [Upgrading](upgrading.md) for behaviour changes.
- Long sessions (replication, LISTEN, long analytical queries) are cut at the drain deadline. Schedule upgrades accordingly.

## Sizing

A session costs two sockets, two copy goroutines and small buffers; CPU use is dominated by TLS termination. The manifests request 100m CPU and 64Mi memory and limit memory to 512Mi, which handles thousands of idle sessions. For very large fleets raise `MAX_CONNECTIONS`, the file descriptor limit (at least twice the connection count) and the CPU request together. SNI passthrough costs less CPU than termination.

## Troubleshooting

| Symptom | Likely cause | Check |
| --- | --- | --- |
| `no backend for this user` | Login has no deployment, no matching Service or static entry, or the routing label forbids the strategy | `resolve failed` log line, Service labels, login format |
| `backend unavailable` | Service DNS fails or backend down | `dial failed` shows the address; test it from the proxy pod |
| Wrong backend reached | Several Services share a route, often a pooler without `xdatabase-proxy-pooled` | `several services match one route` lists the candidates |
| `resolve failed … matches no port` | `xdatabase-proxy-destination-port` names a port the Service does not have | Fix the label or the Service ports |
| Pod not ready | API server unreachable, informer not synced, or TLS Secret cannot be read or created | Pod log, RBAC for services and the Secret |
| `sslmode=require` refused | `TLS_ENABLED=false` | Configuration |
| `verify-full` fails | Self-signed certificate or name mismatch | Provide your own certificate, see [TLS](tls.md) |
| SNI client closed at once | No route for the host, route forbids sni, client sent no SNI (connected by IP), or plaintext MongoDB | `DEBUG=true`, `sni route unavailable` |
| Redis `-ERR unsupported first message` | Client sends PING, HELLO without AUTH or password-only AUTH first | Set user name and password in the client, or use SNI routing |
| ClickHouse `Connection reset by peer` | Routing failed (ClickHouse gets no error message) | `resolve failed` or `login read failed` in the log |
| Service added but not routable | Missing `enabled`, `deployment-id` or wrong `database-type` label | Labels must match exactly, `database-type` must equal `DATABASE_TYPE` |
| Services outside one namespace ignored | `WATCH_NAMESPACE` is set | Unset it and use the ClusterRole |
| Connections dropped at start | `MAX_CONNECTIONS` reached | `connection limit reached, rejecting` |
| psql Ctrl+C does nothing | CancelRequest is not routed | See [PostgreSQL limits](databases/postgresql.md#limits) |
| `configuration error: …` at start | Invalid variable combination | [Configuration](configuration.md#validation-rules) |
