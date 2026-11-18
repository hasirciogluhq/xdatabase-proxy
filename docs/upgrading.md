---
title: Upgrading
nav_order: 14
---

# Upgrading

Releases follow semantic versioning. Image tags have no `v`: git tag `v2.1.2` is image `ghcr.io/hasirciogluhq/xdatabase-proxy:2.1.2`. The full history is in the [changelog](https://github.com/hasirciogluhq/xdatabase-proxy/blob/main/CHANGELOG.md).

## After 2.1.2 (main, not yet released)

Merged on `main` after the v2.1.2 tag (commits eeac149 and cd7fc65). Image `2.1.2` does not contain them yet; they ship with the next release, or build the image from `main` (`docker build .`).


- **Breaking:** an `xdatabase-proxy-destination-port` label that matches no port of the Service now fails the connection instead of silently using the first port. Check your labels before upgrading:

  ```bash
  kubectl get svc -A -l xdatabase-proxy-enabled=true \
    -o custom-columns='NS:.metadata.namespace,NAME:.metadata.name,DEST:.metadata.labels.xdatabase-proxy-destination-port,PORTS:.spec.ports[*].name,NUMS:.spec.ports[*].port'
  ```

- Ambiguous routes (several Services on one route key) are now logged once with every candidate.
- Built with Go 1.27.4 and `golang.org/x/net` v0.62.0.

## 2.1.1 and 2.1.2

License changes only (plain Apache 2.0, NOTICE file). No behaviour change.

## 2.0.x to 2.1.0

2.1.0 rewrote the proxy. Behaviour changes to check:

| Area | 2.0 | 2.1 |
| --- | --- | --- |
| `xdatabase-proxy-pooled` missing | undefined | means `false` (direct) |
| `xdatabase-proxy-destination-port` | ignored | honoured |
| Variables | `RUNTIME`, `TLS_AUTO_RENEW`, `TLS_ENABLE_SELF_SIGNED`, `POSTGRESQL_PROXY_ENABLED` | removed and ignored |
| Port variable | `POSTGRESQL_PROXY_START_PORT` | `PROXY_START_PORT` (old name still read) |
| Self-signed certificate | `TLS_AUTO_RENEW` had no effect | ECDSA P-256, renewed `TLS_RENEWAL_THRESHOLD_DAYS` before expiry |
| Image | | distroless, non-root, multi-arch |
| Manifests | | split into `kubernetes/base`, `production/nodeport`, `production/daemonset-hostport`, `development`, `test` |

New in 2.1.0: Redis, ClickHouse, MongoDB and generic TLS support, SNI routing, the `xdatabase-proxy-routing` label, `SNI_TLS_MODE`, PostgreSQL direct TLS, `WATCH_NAMESPACE`, `CLUSTER_DOMAIN`, `MAX_CONNECTIONS`, `DIAL_TIMEOUT_SECONDS`, graceful shutdown and certificate hot reload.

### The pooler trap

If a pooler Service (pgbouncer) has no `xdatabase-proxy-pooled` label, it now shares the direct route with the database Service, and the first by `namespace/name` wins. Connections still succeed, but may reach the wrong backend. Label every pooler:

```bash
kubectl label svc -n data orders-pgbouncer xdatabase-proxy-pooled=true
```

### Verify routes before switching

A successful connection does not prove the route. In staging:

1. Run the new version with `DEBUG=true`.
2. Replay your real connection strings, both direct and `.pool`.
3. Check the `backend` field of each `session started` line, and that no `several services match one route` warning appears.

### Manifests

Re-apply from the new paths (`kubernetes/production/nodeport` or `kubernetes/production/daemonset-hostport`) and delete objects from the old manifests that no longer exist. RBAC now grants only `list`/`watch` on Services and access to the one TLS Secret.

## 1.x to 2.x

Upgrade to 2.0.0 first, following its release notes, then to 2.1.
