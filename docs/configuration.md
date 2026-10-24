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
| DATABASE_TYPE | postgresql | Only postgresql is accepted |

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
| TLS_ENABLED | true | Offer TLS to clients |
| TLS_MODE | automatic | file, kubernetes or memory. Automatic: file if TLS_CERT_FILE is set, kubernetes if TLS_SECRET_NAME is set, else memory |
| TLS_CERT_FILE, TLS_KEY_FILE | empty | Paths for file mode |
| TLS_SECRET_NAME | empty | Secret of type kubernetes.io/tls for kubernetes mode |
| TLS_AUTO_GENERATE | true | Create a self-signed certificate when none exists |
| TLS_RENEWAL_THRESHOLD_DAYS | 30 | Renew the proxy's own self-signed certificate this many days before expiry |

## Validation rules

- file mode needs both TLS_CERT_FILE and TLS_KEY_FILE.
- kubernetes mode needs TLS_SECRET_NAME.
- Kubernetes access is needed when discovery is kubernetes or TLS mode is kubernetes.
- Unknown values for DISCOVERY_MODE, TLS_MODE or DATABASE_TYPE are errors.

## Removed variables

RUNTIME, TLS_AUTO_RENEW, TLS_ENABLE_SELF_SIGNED and POSTGRESQL_PROXY_ENABLED are gone. The runtime is no longer detected, because behaviour depends only on the settings above.
