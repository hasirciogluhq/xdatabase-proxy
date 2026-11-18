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

Straight from GitHub, pinned to a release:

```bash
kubectl apply -k "github.com/hasirciogluhq/xdatabase-proxy/kubernetes/production/nodeport?ref=v2.1.2"
```

## Base

- Namespace `xdatabase-proxy` and ServiceAccount `xdatabase-proxy`.
- ClusterRole `xdatabase-proxy` with `list` and `watch` on `services`, bound to the ServiceAccount.
- Role `xdatabase-proxy-tls` in the proxy namespace: `get` and `update` on the Secret `xdatabase-proxy-tls`, and `create` on Secrets (create cannot be limited by name in Kubernetes RBAC).

## NodePort variant

- Deployment with 3 replicas, spread across nodes (`topologySpreadConstraints`), rolling update with `maxUnavailable: 0`, `maxSurge: 1`.
- Service type NodePort, node port 30432, `externalTrafficPolicy: Local`, so the client address is kept and there is no second hop. Nodes without a ready proxy pod do not answer on the port, so the load balancer must health check it.
- PodDisruptionBudget with `minAvailable: 1`.

Clients connect to any node on 30432, normally through a load balancer.

## DaemonSet with hostPort variant

- One pod per node, tolerating every taint, priority class `system-node-critical`.
- Container port 5432 published as hostPort 5432 on each node. No Service is created.
- Rolling update, one node at a time.

Make sure nothing else on the nodes uses port 5432. Point DNS or an external load balancer at the node addresses.

## Development variant

One replica, ClusterIP Service, debug logs, memory TLS and the image `ghcr.io/hasirciogluhq/xdatabase-proxy-development:development` with `imagePullPolicy: Always`. That image is built locally by `scripts/local-deploy.sh`; it is not published by CI. Use it with `kubernetes/test/postgresql.yaml`, a throw-away PostgreSQL (password `1234`, database `db`) labelled as deployment `test-db`.

## Settings common to all variants

- Pod security: `runAsNonRoot`, read-only root filesystem, no privilege escalation, all capabilities dropped, `RuntimeDefault` seccomp.
- Probes: `/health` (liveness, every 20 s) and `/ready` (readiness, every 5 s) on the container port named `health` (8080).
- Environment: `NAMESPACE` from the downward API, `DISCOVERY_MODE=kubernetes`, `TLS_ENABLED=true`, `TLS_MODE=kubernetes`, `TLS_SECRET_NAME=xdatabase-proxy-tls`. Production variants do not set `DATABASE_TYPE`, so they run PostgreSQL.
- Resources: requests 100m CPU and 64Mi memory, limit 512Mi memory.
- `terminationGracePeriodSeconds: 40`, longer than the 25 second drain.

## Customising

Create your own kustomization that references a variant and patches it:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - github.com/hasirciogluhq/xdatabase-proxy/kubernetes/production/nodeport?ref=v2.1.2
images:
  - name: ghcr.io/hasirciogluhq/xdatabase-proxy
    newTag: 2.1.2              # image tags have no "v"
patches:
  - target: { kind: Deployment, name: xdatabase-proxy }
    patch: |
      - op: add
        path: /spec/template/spec/containers/0/env/-
        value: { name: WATCH_NAMESPACE, value: data }
```

### Running Redis instead of PostgreSQL

```yaml
patches:
  - target: { kind: Deployment, name: xdatabase-proxy }
    patch: |
      - op: add
        path: /spec/template/spec/containers/0/env/-
        value: { name: DATABASE_TYPE, value: redis }
      - op: add
        path: /spec/template/spec/containers/0/env/-
        value: { name: PROXY_START_PORT, value: "6379" }
      - op: replace
        path: /spec/template/spec/containers/0/ports/0/containerPort
        value: 6379
  - target: { kind: Service, name: xdatabase-proxy }
    patch: |
      - op: replace
        path: /spec/ports/0/port
        value: 6379
      - op: replace
        path: /spec/ports/0/nodePort
        value: 30379
```

For a second protocol next to PostgreSQL, use `namePrefix` or `nameSuffix` in a separate kustomization so the Deployments and Services do not collide, and either a separate TLS Secret name (add it to the Role) or the same certificate.

### LoadBalancer instead of NodePort

```yaml
patches:
  - target: { kind: Service, name: xdatabase-proxy }
    patch: |
      - op: replace
        path: /spec/type
        value: LoadBalancer
      - op: remove
        path: /spec/ports/0/nodePort
```

Keep `externalTrafficPolicy: Local` to preserve client addresses.

### Restricting discovery to one namespace

Set `WATCH_NAMESPACE`, delete the ClusterRole and ClusterRoleBinding, and add a Role and RoleBinding with `list` and `watch` on `services` in that namespace.

### Own certificate

Point `TLS_SECRET_NAME` at a cert-manager Secret and add `TLS_AUTO_GENERATE=false`, see [TLS](../tls.md#cert-manager).

## Where backends may live

Anywhere the proxy pods can resolve and reach through Service DNS. Cross namespace is fine. NetworkPolicies must allow egress from the proxy namespace to the database ports, to cluster DNS and to the API server, and ingress to the proxy port from clients.
