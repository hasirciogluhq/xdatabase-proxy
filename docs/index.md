---
title: Home
nav_order: 1
permalink: /
---

# XDatabase Proxy

XDatabase Proxy is a small, dependency-light database connection router. Clients connect to one endpoint. The proxy looks at the login or at the TLS server name, finds the matching backend and forwards the connection. Behind one address you can run hundreds of databases, each with a direct and a pooled endpoint, without handing out internal host names.

These docs describe the code on `main`: release **v2.1.2** plus the fixes merged after it (ambiguous route warning, strict `xdatabase-proxy-destination-port`, Go 1.27.4), see [Upgrading](upgrading.md#after-212-main-not-yet-released). Latest image: `ghcr.io/hasirciogluhq/xdatabase-proxy:2.1.2`; the post-2.1.2 fixes ship with the next release, or build from `main`.

![psql connected through the proxy with TLS]({{ site.baseurl }}/assets/images/psql-tls-connection.png)

## Supported databases

| Database | DATABASE_TYPE | Routed by | Guide |
| --- | --- | --- | --- |
| PostgreSQL | postgresql | Login `user.deployment[.pool]`, optionally TLS server name | [PostgreSQL](databases/postgresql.md) |
| CockroachDB, YugabyteDB, TimescaleDB, Citus, Greenplum, QuestDB, CrateDB | product name or postgresql | Same as PostgreSQL | [PostgreSQL](databases/postgresql.md#postgresql-compatible-databases) |
| Redis | redis | `AUTH user.deployment[.pool] …` and/or TLS server name | [Redis](databases/redis.md) |
| ClickHouse (native protocol) | clickhouse | User in the Hello packet and/or TLS server name | [ClickHouse](databases/clickhouse.md) |
| MongoDB | mongodb | TLS server name only | [MongoDB and TLS](databases/mongodb.md) |
| Any TLS-first protocol | tls | TLS server name only | [MongoDB and TLS](databases/mongodb.md#generic-tls-protocols) |
| MySQL, MariaDB | not supported yet | | |

One proxy process serves one database type. Run one instance (or one Deployment) per type.

## What it does

- **Routes by username.** The login `alice.db-prod.pool` goes to the pooled endpoint of deployment `db-prod`. The backend sees the user `alice`.
- **Routes by TLS server name (SNI).** A client connecting to `db-prod.pool.example.com` reaches the same endpoint. TLS is passed through to the backend or terminated at the proxy, your choice.
- **Discovers backends** from labelled Kubernetes Services, live through an informer, or from a static list for VMs and containers.
- **Terminates TLS** for clients with a certificate from a file, a Kubernetes Secret or memory, and can create and renew its own self-signed one. Rotated certificates are picked up without a restart.
- **Runs anywhere**: Deployment behind a NodePort, DaemonSet on host ports, plain container, VM or bare metal.
- **Ships hardened**: distroless, non-root, multi-architecture image with SBOM and provenance.

## What it does not do

- It does not parse queries, pool connections or balance load between replicas of one database. Put a pooler such as pgbouncer behind the pooled endpoint and a cluster manager such as Patroni or CloudNativePG behind the direct endpoint.
- It does not authenticate users. Authentication happens on the backend; the proxy forwards the exchange untouched.
- MongoDB and generic TLS protocols cannot be routed by username, only by TLS server name.
- PostgreSQL query cancellation (CancelRequest) is not routed.

## Where to go next

| Goal | Page |
| --- | --- |
| Try it in five minutes | [Getting started](getting-started.md) |
| Understand the model | [Concepts](concepts.md) |
| Set up a specific database | [Databases](databases/index.md) |
| Decide username vs. SNI routing | [Routing strategies](routing-strategies.md) |
| Look up an environment variable | [Configuration](configuration.md) |
| Label Kubernetes Services | [Service discovery](service-discovery.md) |
| Certificates | [TLS](tls.md) |
| Deploy to production | [Deployment](deployment/index.md) |
| Upgrade from an older version | [Upgrading](upgrading.md) |
| Run it safely | [Security](security.md) and [Operations](operations.md) |
| Find a log message | [Log messages](reference/log-messages.md) |
| Common questions | [FAQ](faq.md) |
| Change the code | [Architecture](architecture.md) and [Development](development.md) |
