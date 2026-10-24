---
title: Service discovery
nav_order: 6
---

# Service discovery

## Static

```
STATIC_BACKENDS="db1=10.0.0.5:5432,db1.pool=10.0.0.5:6432,db2=db2.internal:5432"
```

The table is read once at start. Use it for VMs, containers and tests. Whitespace around entries is ignored. A malformed entry stops startup.

## Kubernetes

Label the Service of each database endpoint:

| Label | Required | Meaning |
| --- | --- | --- |
| xdatabase-proxy-enabled: "true" | yes | Opt in |
| xdatabase-proxy-database-type: postgresql | yes | Protocol |
| xdatabase-proxy-deployment-id | yes | Routing key, the deployment part of the login |
| xdatabase-proxy-pooled: "true" | no | Marks the pooled endpoint. Missing means false |
| xdatabase-proxy-destination-port | no | Port number or port name to use. Default is the first port |
| xdatabase-proxy-routing | no | username, sni or all. Which routing strategies may reach this Service, see Routing strategies. Default depends on the database type |

Example for a database and its pooler:

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
```

The login alice.orders now reaches orders-db.data.svc.cluster.local:5432. Create a second Service with xdatabase-proxy-pooled set to true to serve alice.orders.pool.

## Behaviour

- Changes are picked up within a second through the watch. No restart is needed.
- If several Services share one route, the lowest namespace/name wins.
- Services without ports are ignored.
- The proxy talks to the Service DNS name, so the proxy pods need working cluster DNS. For other cluster domains set CLUSTER_DOMAIN.
- The informer resyncs every 10 minutes as a safety net and reconnects by itself after API server outages.

## Permissions

Cluster wide discovery needs list and watch on services (ClusterRole in kubernetes/base/rbac.yaml). With WATCH_NAMESPACE a Role in that namespace is enough.
