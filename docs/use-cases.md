---
title: Use cases
nav_order: 11
---

# Use cases

## One public endpoint for many databases

A platform runs one database per customer, each in its own namespace. Label each Service with a deployment id such as `customer-42`. Customers connect to `db.example.com` with `alice.customer-42` and never learn internal addresses. A new database is routable as soon as its Service exists; deleting the Service removes the route.

## Database as a service with host names

Give every customer a host name instead of a login suffix: `customer-42.db.example.com`. Point `*.db.example.com` at the proxy, put a wildcard certificate on it (or pass TLS through) and set `xdatabase-proxy-routing: sni` or `all` on the Services. Clients keep their plain user names, which suits BI tools and managed connectors that cannot change the user.

## Direct and pooled access

Applications use `user.deployment.pool`, which reaches pgbouncer. Operators use `user.deployment` for migrations, long transactions and admin tasks, which reaches the database. Both share one host and one certificate.

## TLS in front of databases without TLS

Backends often run without certificates. The proxy gives clients encrypted connections and manages its own certificate, or one from cert-manager. Keep the proxy and the databases on a trusted network.

## One certificate instead of hundreds

MongoDB or Redis with `SNI_TLS_MODE=terminate`: the proxy holds one wildcard certificate, the databases run plain TCP inside the cluster.

## End-to-end TLS without the proxy seeing data

With SNI passthrough the proxy never decrypts traffic. It only reads the server name from the ClientHello. Use this when compliance requires TLS up to the database, or when clients authenticate with certificates.

## Access from outside the cluster

Expose the proxy and keep the databases private:

- Deployment with NodePort: clients use any node IP on port 30432. Put a load balancer in front for availability.
- DaemonSet with hostPort: every node serves port 5432 directly. No kube-proxy hop, and the client address is preserved.
- LoadBalancer Service: patch the NodePort variant, see [Kubernetes](deployment/kubernetes.md#loadbalancer-instead-of-nodeport).

## Several database engines

Run one proxy per engine on its own port, all fed by the same labelled Services:

| Proxy | DATABASE_TYPE | Port |
| --- | --- | --- |
| xdatabase-proxy-pg | postgresql | 5432 |
| xdatabase-proxy-redis | redis | 6379 |
| xdatabase-proxy-ch | clickhouse | 9000 |
| xdatabase-proxy-mongo | mongodb | 27017 |

## PostgreSQL-compatible databases

CockroachDB, YugabyteDB, TimescaleDB, Citus, Greenplum, QuestDB and CrateDB speak the PostgreSQL protocol. Set `DATABASE_TYPE` to the product name (or postgresql) and label their Services with `xdatabase-proxy-database-type: postgresql`. Startup options such as `--cluster=…` are forwarded unchanged.

## Databases on VMs

Run the container or binary on a VM with static routing:

```bash
STATIC_BACKENDS="crm=10.1.0.4:5432,billing=10.1.0.9:5432"
```

If the VM can reach the Kubernetes API and cluster DNS, set `KUBECONFIG` and use Kubernetes discovery for in-cluster databases instead.

## Migrating a database without changing clients

Clients use `alice.orders`. Move the label `xdatabase-proxy-deployment-id: orders` from the old Service to the new one. New connections go to the new backend immediately; existing sessions stay on the old one until they reconnect.

## Local development

`scripts/dev-run.sh` points the proxy at a local PostgreSQL, so applications use the same login format as in production.

## Clusters managed by an operator

Patroni, pgpool-II, CloudNativePG, Zalando, Crunchy PGO, Redis operators and the MongoDB community operator expose a Service for the primary. Label that Service, or create your own with the same selector. Failover stays the operator's job; the proxy follows the Service.
