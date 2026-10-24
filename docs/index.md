---
title: Home
nav_order: 1
permalink: /
---

# XDatabase Proxy

XDatabase Proxy is a small PostgreSQL connection router. Clients connect to one endpoint. The proxy reads the username, finds the matching backend and forwards the connection. It terminates TLS itself, so a backend does not need its own certificate setup.

![psql connected through the proxy with TLS]({{ site.baseurl }}/assets/images/psql-tls-connection.png)

## What it does

- Routes by username. The login `alice.db-prod.pool` goes to the pooled endpoint of deployment `db-prod`. The backend sees the user `alice`.
- Finds backends from labelled Kubernetes Services (live, through an informer) or from a static list.
- Offers TLS to clients and can create and renew its own self-signed certificate.
- Runs as a Deployment behind a NodePort, as a DaemonSet on host ports, as a plain container, or on a VM.
- Ships as a distroless, non-root, multi-architecture image.

## What it does not do

- It does not parse SQL, pool connections or balance load between replicas of one database. Put a pooler such as pgbouncer behind the pooled endpoint, and a cluster manager such as Patroni behind the direct endpoint.
- It does not authenticate users. Authentication happens on the backend.
- Login based routing is PostgreSQL only. Other databases are routed by TLS server name, see [SNI routing](sni-routing.md). MySQL is not supported.

## Where to go next

| Goal | Page |
| --- | --- |
| Try it in five minutes | [Getting started](getting-started.md) |
| Understand how a connection flows | [Architecture](architecture.md) and [Connection routing](connection-routing.md) |
| Look up an environment variable | [Configuration](configuration.md) |
| Deploy to production | [Deployment](deployment/index.md) |
| See typical setups | [Use cases](use-cases.md) |
| Run it safely | [Security](security.md) and [Operations](operations.md) |
| Change the code | [Development](development.md) |
