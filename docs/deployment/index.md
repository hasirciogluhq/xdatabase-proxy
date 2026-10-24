---
title: Deployment
nav_order: 9
has_children: true
---

# Deployment

Pick the target that matches how clients reach your databases.

| Target | Path | Choose it when |
| --- | --- | --- |
| Kubernetes, NodePort | kubernetes/production/nodeport | You have an external load balancer or can use node IPs, and want replicas managed as a normal Deployment |
| Kubernetes, DaemonSet with hostPort | kubernetes/production/daemonset-hostport | You want one proxy per node on a fixed port, no Service hop, and preserved client addresses |
| Kubernetes, development | kubernetes/development | Local or test clusters |
| Container or VM | see Container and VM | Databases are outside Kubernetes, or you do not run Kubernetes |

All Kubernetes variants build on kubernetes/base: namespace, ServiceAccount and RBAC.
