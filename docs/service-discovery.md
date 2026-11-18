---
title: Service discovery
nav_order: 8
---

# Service discovery

The resolver turns a route key (deployment, pooled) into `host:port`. There are two: a static table and a live Kubernetes watch.

## Static

```bash
STATIC_BACKENDS="db1=10.0.0.5:5432,db1.pool=10.0.0.5:6432,db2=db2.internal:5432"
```

- Keys are `deployment` for the direct route and `deployment.pool` for the pooled route.
- Values are any `host:port` the proxy can dial; host names are resolved at connect time, so DNS changes are followed.
- Whitespace around entries is ignored, empty entries are skipped. An entry without `=` or with an empty side stops startup.
- The table is read once at start. Restart to change it.
- Every routing strategy is allowed. There is no routing label.
- Keys are matched exactly. SNI host names are lower cased first, so use lower case keys for SNI routing.

Use it for VMs, containers, tests, or databases outside Kubernetes.

## Kubernetes

The proxy watches Services through a client-go informer and keeps an in-memory index. Label the Service of each database endpoint:

| Label | Required | Meaning |
| --- | --- | --- |
| xdatabase-proxy-enabled | yes | Must be `"true"` |
| xdatabase-proxy-database-type | yes | Must equal the proxy's `DATABASE_TYPE`: `postgresql` (also for compatible products), `redis`, `clickhouse`, `mongodb` or `tls` |
| xdatabase-proxy-deployment-id | yes | The deployment part of the login or host name |
| xdatabase-proxy-pooled | no | `"true"` marks the pooled endpoint. Anything else or missing means direct |
| xdatabase-proxy-destination-port | no | Port number or port name of the Service to use. Default is the first port. A value that matches no port is an error |
| xdatabase-proxy-routing | no | `username`, `sni` or `all`. See [Routing strategies](routing-strategies.md#the-routing-label) |

The proxy dials `<service>.<namespace>.svc.<CLUSTER_DOMAIN>:<service port>`, the Service's `port`, not its `targetPort`.

### Example: database and pooler

```yaml
apiVersion: v1
kind: Service
metadata:
  name: orders-db
  namespace: data
  labels:
    xdatabase-proxy-enabled: "true"
    xdatabase-proxy-database-type: postgresql
    xdatabase-proxy-deployment-id: orders
spec:
  selector: { app: orders-db }
  ports:
    - { name: postgresql, port: 5432 }
---
apiVersion: v1
kind: Service
metadata:
  name: orders-pgbouncer
  namespace: data
  labels:
    xdatabase-proxy-enabled: "true"
    xdatabase-proxy-database-type: postgresql
    xdatabase-proxy-deployment-id: orders
    xdatabase-proxy-pooled: "true"
spec:
  selector: { app: orders-pgbouncer }
  ports:
    - { name: metrics, port: 9127 }
    - { name: pgbouncer, port: 6432 }
```

`alice.orders` reaches `orders-db.data.svc.cluster.local:5432`. `alice.orders.pool` would reach port 9127, the first port, so add `xdatabase-proxy-destination-port: pgbouncer` to the pooler.

### Example: labelling a Service created by an operator

Operators often own their Services and overwrite labels. Create your own Service with the same selector instead:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: orders-rw-proxy
  namespace: data
  labels:
    xdatabase-proxy-enabled: "true"
    xdatabase-proxy-database-type: postgresql
    xdatabase-proxy-deployment-id: orders
spec:
  selector:
    cnpg.io/cluster: orders
    cnpg.io/instanceRole: primary
  ports:
    - { port: 5432 }
```

### Example: Redis reachable only by host name

```yaml
metadata:
  labels:
    xdatabase-proxy-enabled: "true"
    xdatabase-proxy-database-type: redis
    xdatabase-proxy-deployment-id: sessions
    xdatabase-proxy-routing: sni
    xdatabase-proxy-destination-port: tls
spec:
  ports:
    - { name: plain, port: 6379 }
    - { name: tls, port: 6380 }
```

## Behaviour

- Changes are picked up as soon as the watch event arrives, normally well under a second. No restart is needed.
- The informer resyncs every 10 minutes as a safety net and reconnects by itself after API server outages; existing routes stay usable meanwhile.
- The proxy only becomes ready after the first full list of Services has synced.
- Services without a deployment id or without ports are ignored.
- Services whose routing label does not allow the client's strategy are skipped.
- If several Services still match one route, the lowest `namespace/name` wins and the proxy logs one warning per distinct set of candidates (`several services match one route …`), naming the chosen one and all candidates.
- A destination-port label that matches no port fails the connection (`resolve failed`) instead of falling back to the first port (since main after 2.1.2; image 2.1.2 still falls back).
- The proxy talks to the Service DNS name, so its pods need working cluster DNS. For other cluster domains set `CLUSTER_DOMAIN`.
- Services in any namespace are found unless `WATCH_NAMESPACE` is set. Deployment ids are global across namespaces, so make them unique.

## Permissions

| Scope | RBAC |
| --- | --- |
| All namespaces (default) | ClusterRole with `list` and `watch` on `services` (shipped in `kubernetes/base/rbac.yaml`) |
| `WATCH_NAMESPACE=data` | Role with `list` and `watch` on `services` in `data`, bound to the proxy's ServiceAccount |

A label routes traffic. Restrict who may create or label Services, see [Security](security.md).
