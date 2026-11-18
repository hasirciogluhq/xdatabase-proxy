---
title: Configuration
nav_order: 7
---

# Configuration

All settings are environment variables; there are no configuration files and no command line options except `-healthcheck`. Invalid values stop the process at start with `configuration error: …` and exit code 1. Empty variables count as unset. Boolean and number values that cannot be parsed silently fall back to their default, so double check spelling (`true`, `false`, `1`, `0` are accepted).

## Protocol

| Variable | Default | Description |
| --- | --- | --- |
| DATABASE_TYPE | postgresql | `postgresql`, `redis`, `clickhouse`, `mongodb` or `tls`. The PostgreSQL-compatible names `cockroachdb`, `yugabytedb`, `timescaledb`, `citus`, `greenplum`, `questdb`, `cratedb` are accepted and treated as `postgresql` (Services keep the label `postgresql`). Case insensitive. See [Databases](databases/index.md) |
| SNI_TLS_MODE | passthrough | `passthrough` or `terminate`. How SNI routes handle TLS for redis, clickhouse, mongodb and tls. See [Routing strategies](routing-strategies.md#tls-terminate-or-pass-through) |

## Listener and limits

| Variable | Default | Description |
| --- | --- | --- |
| PROXY_START_PORT | 5432 | TCP port for clients, whatever the database type. `POSTGRESQL_PROXY_START_PORT` is the older name and still works |
| HEALTH_SERVER_PORT | 8080 | HTTP port for `/health` and `/ready`. Also used by `-healthcheck` |
| MAX_CONNECTIONS | 10000 | Concurrent client connections. Further connections are closed at once. `0` means unlimited |
| DIAL_TIMEOUT_SECONDS | 5 | Time allowed for resolving and dialling the backend |
| DEBUG | false | Debug level logs: session starts, startup and handshake failures, unusable SNI routes |

## Discovery

| Variable | Default | Description |
| --- | --- | --- |
| DISCOVERY_MODE | automatic | `static` or `kubernetes`. Automatic: `static` when `STATIC_BACKENDS` is set, otherwise `kubernetes` |
| STATIC_BACKENDS | empty | `deployment=host:port` pairs separated by commas. Add `.pool` to the key for pooled routes. See [Service discovery](service-discovery.md#static) |
| WATCH_NAMESPACE | all namespaces | Limit Service discovery to one namespace (a Role is then enough) |
| CLUSTER_DOMAIN | cluster.local | Suffix for Service DNS names |
| KUBECONFIG | empty | Path to a kubeconfig. Empty means in-cluster service account |
| KUBE_CONTEXT | empty | Context inside the kubeconfig. Empty means its current context |
| NAMESPACE | pod namespace | Namespace holding the TLS Secret. Read in order: `NAMESPACE`, `POD_NAMESPACE`, the service account namespace file, then `default` |

## TLS

| Variable | Default | Description |
| --- | --- | --- |
| TLS_ENABLED | true | Terminate TLS for clients. Forced to `false` for mongodb and tls with `SNI_TLS_MODE=passthrough`, where no certificate is needed |
| TLS_MODE | automatic | `file`, `kubernetes` or `memory`. Automatic: `file` if `TLS_CERT_FILE` is set, `kubernetes` if `TLS_SECRET_NAME` is set, else `memory` |
| TLS_CERT_FILE, TLS_KEY_FILE | empty | PEM files for file mode. The certificate file may contain the chain |
| TLS_SECRET_NAME | empty | Secret of type `kubernetes.io/tls` in `NAMESPACE`, for kubernetes mode. Created if missing and auto generation is on |
| TLS_AUTO_GENERATE | true | Create a self-signed certificate when none exists |
| TLS_RENEWAL_THRESHOLD_DAYS | 30 | Re-issue the proxy's **own** self-signed certificate this many days before expiry. Certificates you provide are never replaced |

See [TLS](tls.md).

## Validation rules

- `DATABASE_TYPE`, `SNI_TLS_MODE`, `DISCOVERY_MODE` and `TLS_MODE` must have a known value.
- `SNI_TLS_MODE=terminate` needs `TLS_ENABLED=true` and is rejected for postgresql, which always terminates TLS.
- With TLS enabled, file mode needs both `TLS_CERT_FILE` and `TLS_KEY_FILE`, and kubernetes mode needs `TLS_SECRET_NAME`.
- A malformed `STATIC_BACKENDS` entry stops startup.
- Kubernetes API access is needed when discovery is kubernetes or TLS is enabled in kubernetes mode. Outside a pod set `KUBECONFIG`, otherwise startup fails with `kubernetes client: … (outside a pod set KUBECONFIG)`.

## Examples

PostgreSQL in Kubernetes, certificate managed by cert-manager:

```bash
DISCOVERY_MODE=kubernetes
TLS_MODE=kubernetes
TLS_SECRET_NAME=db-proxy-tls
TLS_AUTO_GENERATE=false
```

Redis on a VM, own certificate files:

```bash
DATABASE_TYPE=redis
PROXY_START_PORT=6379
STATIC_BACKENDS="cache1=10.0.1.10:6379,cache2=10.0.1.11:6379"
TLS_CERT_FILE=/etc/xdatabase/tls.crt
TLS_KEY_FILE=/etc/xdatabase/tls.key
```

MongoDB, TLS passed through to mongod, no certificate on the proxy:

```bash
DATABASE_TYPE=mongodb
PROXY_START_PORT=27017
STATIC_BACKENDS="m1=10.0.2.5:27017"
```

MongoDB, TLS terminated at the proxy, plain mongod:

```bash
DATABASE_TYPE=mongodb
SNI_TLS_MODE=terminate
TLS_CERT_FILE=/tls/wildcard.crt
TLS_KEY_FILE=/tls/wildcard.key
```

## Removed variables

`RUNTIME`, `TLS_AUTO_RENEW`, `TLS_ENABLE_SELF_SIGNED` and `POSTGRESQL_PROXY_ENABLED` were removed in v2.1.0 and are ignored. The runtime is no longer detected, because behaviour depends only on the settings above. See [Upgrading](upgrading.md).
