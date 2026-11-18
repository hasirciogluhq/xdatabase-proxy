---
title: Configuration
nav_order: 5
---

# Configuration

All settings are environment variables. Invalid values stop the process at start with a message.

## Listener and limits

| Variable | Default | Description |
| --- | --- | --- |
| PROXY_START_PORT | 5432 | TCP port for clients. POSTGRESQL_PROXY_START_PORT is the older name and still works |
| HEALTH_SERVER_PORT | 8080 | HTTP port for /health and /ready |
| MAX_CONNECTIONS | 10000 | Concurrent client connections. 0 means unlimited |
| DIAL_TIMEOUT_SECONDS | 5 | Time allowed for resolving and dialing the backend |
| DEBUG | false | Debug level logs |
| DATABASE_TYPE | postgresql | postgresql; the PostgreSQL-compatible names cockroachdb, yugabytedb, timescaledb, citus, greenplum, questdb, cratedb (treated as postgresql, Services keep the label postgresql); redis and clickhouse (username routing, SNI passthrough); mongodb and tls (SNI routing only; TLS_* settings are used only with SNI_TLS_MODE=terminate). See [Routing strategies](routing-strategies.md) |

## Discovery

| Variable | Default | Description |
| --- | --- | --- |
| DISCOVERY_MODE | automatic | static or kubernetes. Automatic picks static when STATIC_BACKENDS is set |
| STATIC_BACKENDS | empty | Comma separated deployment=host:port. Add .pool to the key for pooled routes |
| WATCH_NAMESPACE | all | Limit Service discovery to one namespace |
| CLUSTER_DOMAIN | cluster.local | Suffix for service DNS names |
| KUBECONFIG | empty | Path to a kubeconfig. Empty means in-cluster |
| KUBE_CONTEXT | empty | Context inside the kubeconfig |
| NAMESPACE | pod namespace | Namespace holding the TLS Secret. POD_NAMESPACE is also read. Falls back to default |

## TLS

| Variable | Default | Description |
| --- | --- | --- |
| SNI_TLS_MODE | passthrough | passthrough or terminate. How SNI routes handle TLS for redis, clickhouse, mongodb and tls: pass the encrypted bytes through (backend terminates) or terminate at the proxy and use plain TCP to the backend. Terminate needs TLS_ENABLED=true. Not applicable to postgresql |
| TLS_ENABLED | true | Offer TLS to clients (proxy-side termination). Forced to false for mongodb and tls with SNI_TLS_MODE=passthrough, since no certificate is needed |
| TLS_MODE | automatic | file, kubernetes or memory. Automatic: file if TLS_CERT_FILE is set, kubernetes if TLS_SECRET_NAME is set, else memory |
| TLS_CERT_FILE, TLS_KEY_FILE | empty | Paths for file mode |
| TLS_SECRET_NAME | empty | Secret of type kubernetes.io/tls for kubernetes mode |
| TLS_AUTO_GENERATE | true | Create a self-signed certificate when none exists |
| TLS_RENEWAL_THRESHOLD_DAYS | 30 | Renew the proxy's own self-signed certificate this many days before expiry |

## Validation rules

- file mode needs both TLS_CERT_FILE and TLS_KEY_FILE (checked only when TLS is enabled).
- kubernetes mode needs TLS_SECRET_NAME (checked only when TLS is enabled).
- SNI_TLS_MODE=terminate needs TLS_ENABLED=true and is rejected for postgresql.
- Kubernetes access is needed when discovery is kubernetes or TLS is enabled in kubernetes mode.
- Unknown values for DATABASE_TYPE, SNI_TLS_MODE, DISCOVERY_MODE or TLS_MODE are errors.
- Boolean and number variables that cannot be parsed fall back to their default.

## Removed variables

RUNTIME, TLS_AUTO_RENEW, TLS_ENABLE_SELF_SIGNED and POSTGRESQL_PROXY_ENABLED are gone. The runtime is no longer detected, because behaviour depends only on the settings above.
