---
title: Use cases
nav_order: 8
---

# Use cases

## One public endpoint for many databases

A platform runs a PostgreSQL per customer, each in its own namespace. Label each Service with a deployment id. Customers connect to one host with user.deployment and never learn internal addresses. New databases are routable as soon as the Service exists.

## Direct and pooled access

Applications use user.deployment.pool, which reaches pgbouncer. Operators use user.deployment for migrations and admin tasks, which reaches the database. Both share one host and one certificate.

## TLS in front of databases without TLS

Backends often run without certificates. The proxy gives clients encrypted connections and renews its own certificate. Keep the proxy and the databases on a trusted network.

## Access from outside the cluster

Expose the proxy and keep the databases private. Two options:

- Deployment with NodePort: clients use any node IP on port 30432. Put a cloud load balancer in front for availability.
- DaemonSet with hostPort: every node serves port 5432. Point DNS or an external load balancer at the nodes. No kube-proxy hop, and the client address is preserved.

## Databases on VMs

Run the container or binary on a VM with static routing:

```
STATIC_BACKENDS="crm=10.1.0.4:5432,billing=10.1.0.9:5432"
```

If the VM can reach the Kubernetes API, set KUBECONFIG and use Kubernetes discovery for in-cluster databases instead.

## Local development

scripts/dev-run.sh points the proxy at a local PostgreSQL so applications can use the same login format as in production.

## Clusters managed by an operator

Patroni, pgpool-II, CloudNativePG and similar tools expose a Service for the primary. Label that Service. Failover stays the operator's job, the proxy follows the Service.
