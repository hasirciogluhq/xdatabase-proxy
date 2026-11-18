---
title: Concepts
nav_order: 3
---

# Concepts

A handful of ideas explain the whole proxy.

## Deployment and route

A **deployment** is a name for one logical database, for example `orders` or `customer-42`. It has up to two endpoints:

| Endpoint | Typical backend | Selected by |
| --- | --- | --- |
| direct | The database itself (primary Service) | `alice.orders` or `orders.example.com` |
| pooled | A pooler in front of it (pgbouncer, odyssey, a Redis proxy) | `alice.orders.pool` or `orders.pool.example.com` |

The pair (deployment, pooled) is the **route key**. Every connection is reduced to a route key, and a **resolver** turns the key into `host:port`.

## Two ways to get the route key

| Strategy | Where the key comes from | Databases |
| --- | --- | --- |
| username | The login in the first client message: `user.deployment[.pool]` | PostgreSQL, Redis, ClickHouse |
| sni | The TLS server name the client connected to: `deployment[.pool].any.domain` | All, required for MongoDB and `tls` |

With username routing the proxy removes the suffix before forwarding, so the backend sees `alice`. With SNI routing nothing in the login changes.

Which strategy may reach a backend is decided per Kubernetes Service with the label `xdatabase-proxy-routing`. Details: [Routing strategies](routing-strategies.md).

## Two resolvers

| Resolver | Configured by | Use for |
| --- | --- | --- |
| static | `STATIC_BACKENDS="orders=10.0.0.5:5432,orders.pool=10.0.0.5:6432"` | VMs, containers, tests |
| kubernetes | Labels on Services, watched live | Clusters |

Details: [Service discovery](service-discovery.md).

## Who terminates TLS

| Situation | TLS ends at | Proxy needs a certificate | Proxy to backend |
| --- | --- | --- | --- |
| PostgreSQL | proxy | yes (auto self-signed by default) | plain TCP |
| Redis / ClickHouse routed by username | proxy | yes | plain TCP |
| SNI route, `SNI_TLS_MODE=passthrough` (default) | backend | no | the client's TLS, untouched |
| SNI route, `SNI_TLS_MODE=terminate` | proxy | yes | plain TCP |

Details: [TLS](tls.md).

## One process, one protocol

`DATABASE_TYPE` selects the protocol handler. A proxy for PostgreSQL and a proxy for Redis are two processes, typically two Deployments on two ports. Kubernetes discovery only watches Services whose `xdatabase-proxy-database-type` label equals the proxy's type, so the instances never see each other's routes.

## What happens after routing

Once the backend is dialled and the (possibly rewritten) first message is forwarded, the proxy copies bytes in both directions until either side closes. Authentication, queries, results, transactions and protocol upgrades pass through untouched. The proxy adds no per-query overhead.
