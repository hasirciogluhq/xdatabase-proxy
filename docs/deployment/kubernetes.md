---
title: Kubernetes
parent: Deployment
nav_order: 1
---

# Kubernetes

Apply with Kustomize:

```bash
kubectl apply -k kubernetes/production/nodeport
# or
kubectl apply -k kubernetes/production/daemonset-hostport
```

## Base

- Namespace xdatabase-proxy and ServiceAccount xdatabase-proxy.
- ClusterRole with list and watch on services, bound to the ServiceAccount.
- Role for the TLS Secret xdatabase-proxy-tls (get, update, create) in the proxy namespace.

## NodePort variant

- Deployment with 3 replicas, spread across nodes, rolling update without downtime.
- Service type NodePort, port 30432, externalTrafficPolicy Local so the client address is kept and no second hop happens. Nodes without a ready proxy pod do not answer on the port.
- PodDisruptionBudget with minAvailable 1.

Clients connect to any node IP on 30432, normally through a load balancer that health checks the port.

## DaemonSet with hostPort variant

- One pod per node, tolerating every taint, with priority class system-node-critical.
- Container port 5432 published as hostPort 5432 on each node. No Service is created.
- Rolling update, one node at a time.

Make sure nothing else on the nodes uses port 5432. Point DNS or an external load balancer at the node addresses.

## Development variant

One replica, ClusterIP Service, debug logs, memory TLS and the development image. Use with kubernetes/test/postgresql.yaml, a PostgreSQL labelled for discovery (login postgres.test-db).

## Settings common to all variants

- Pod security: non-root, read-only filesystem, no capabilities, RuntimeDefault seccomp.
- Probes: /health for liveness, /ready for readiness, on container port 8080.
- Environment: NAMESPACE from the downward API, DISCOVERY_MODE kubernetes, TLS in kubernetes mode with Secret xdatabase-proxy-tls.
- terminationGracePeriodSeconds 40, longer than the 25 second drain.

## Customising

Create your own kustomization that references a variant and patches it:

```yaml
resources:
  - github.com/hasirciogluhq/xdatabase-proxy/kubernetes/production/nodeport?ref=v2.1.0
images:
  - name: ghcr.io/hasirciogluhq/xdatabase-proxy
    newTag: v2.1.0
```

Typical patches: image tag or digest, replicas, resource limits, WATCH_NAMESPACE with a Role in place of the ClusterRole, TLS_SECRET_NAME for a cert-manager certificate.

## Where backends may live

Anywhere the proxy pods can resolve and reach through Service DNS. Cross namespace is fine. Network policies must allow egress from the proxy namespace to the database ports and to the API server.
