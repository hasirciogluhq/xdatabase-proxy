---
title: Deployment
nav_order: 10
has_children: true
---

# Deployment

Pick the target that matches how clients reach your databases.

| Target | Path | Choose it when |
| --- | --- | --- |
| Kubernetes, NodePort | `kubernetes/production/nodeport` | You have an external load balancer or can use node IPs, and want replicas managed as a normal Deployment |
| Kubernetes, DaemonSet with hostPort | `kubernetes/production/daemonset-hostport` | You want one proxy per node on a fixed port, no Service hop, and preserved client addresses |
| Kubernetes, LoadBalancer | NodePort variant with a patched Service type | Your cloud provisions load balancers for Services |
| Kubernetes, development | `kubernetes/development` | Local or test clusters |
| Container or VM | [Container and VM](container-and-vm.md) | Databases are outside Kubernetes, or you do not run Kubernetes |

All Kubernetes variants build on `kubernetes/base`: namespace `xdatabase-proxy`, ServiceAccount and RBAC.

## Images

| Image | Tags | Built from |
| --- | --- | --- |
| `ghcr.io/hasirciogluhq/xdatabase-proxy` | `2.1.2`, `2.1`, `latest` | Every `v*` git tag (git tag `v2.1.2` → image tag `2.1.2`) |

Images are linux/amd64 and linux/arm64, based on `gcr.io/distroless/static-debian12:nonroot`, with SBOM and SLSA provenance attestations. The binary is `/xdatabase-proxy`; ports 5432 and 8080 are exposed; the Docker HEALTHCHECK runs `/xdatabase-proxy -healthcheck`.

Pin a version or digest in production. `latest` follows the newest release.

Verify provenance:

```bash
gh attestation verify oci://ghcr.io/hasirciogluhq/xdatabase-proxy:2.1.2 --owner hasirciogluhq
```

## One instance per database type

A proxy process serves one `DATABASE_TYPE`. For PostgreSQL and Redis, run two Deployments (copy the variant, change name, port and `DATABASE_TYPE`). They can share the namespace, ServiceAccount and RBAC from `kubernetes/base`, but should use different TLS Secret names or the same certificate deliberately.
