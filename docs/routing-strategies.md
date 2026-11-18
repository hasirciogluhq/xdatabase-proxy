---
title: Routing strategies
nav_order: 5
---

# Routing strategies

A connection reaches a backend either by **username** (the login carries the deployment) or by **sni** (the TLS server name carries it). The cluster administrator decides per Service which strategies may reach it.

## Supported per database

| DATABASE_TYPE | username | sni | Default for Services without the label |
| --- | --- | --- | --- |
| postgresql (and compatible names) | yes | yes, only when the login has no deployment | username |
| redis | yes, first command AUTH or HELLO … AUTH | yes | username and sni |
| clickhouse | yes, native Hello packet | yes | username and sni |
| mongodb | no | yes | sni |
| tls | no | yes | sni |

These defaults live in `cmd/proxy/main.go` (`defaultRouting`).

## The routing label

```yaml
metadata:
  labels:
    xdatabase-proxy-enabled: "true"
    xdatabase-proxy-database-type: redis
    xdatabase-proxy-deployment-id: cache1
    xdatabase-proxy-routing: username   # username | sni | all
```

| Value | Strategies allowed |
| --- | --- |
| username | username only |
| sni | sni only |
| all | both |
| missing or any other value | the default of the database type |

When several Services share a route key, only those allowing the client's strategy are considered. With `STATIC_BACKENDS` every strategy is allowed for every entry.

Typical uses:

- `sni` on a PostgreSQL Service to let clients that cannot change the user name (BI tools, managed connectors) connect by host name.
- `username` on a Redis Service to make sure nobody reaches it by host name with passthrough.
- Two Services for the same deployment, one `username` and one `sni`, pointing at different ports of the same backend (plain and TLS).

## TLS: terminate or pass through

`SNI_TLS_MODE` decides how SNI routes handle TLS, per proxy instance:

| SNI_TLS_MODE | What happens | Backend | Client sees | Proxy certificate |
| --- | --- | --- | --- | --- |
| passthrough (default) | Encrypted bytes are forwarded untouched | must speak TLS | backend's certificate | not needed |
| terminate | Proxy ends TLS and talks plain TCP | plain TCP | proxy's certificate | required (`TLS_ENABLED=true`) |

Username routing always terminates TLS at the proxy, because the login is inside the encrypted stream. PostgreSQL always terminates; `SNI_TLS_MODE` is rejected for it.

| Database | TLS terminated by proxy | TLS passthrough | Route by username | Route by SNI |
| --- | --- | --- | --- | --- |
| PostgreSQL | always | no | yes | when the label allows |
| Redis | username routing, or SNI with terminate | default for SNI | yes | yes |
| ClickHouse | username routing, or SNI with terminate | default for SNI | yes | yes |
| MongoDB, tls | SNI with terminate | default | no | yes |

## How a connection is decided (Redis, ClickHouse, MongoDB, tls)

1. The proxy peeks at the first bytes (10 second deadline). A TLS record starts with `0x16 0x03`.
2. **TLS with a server name**: if a route exists for that name and allows sni, the connection is passed through or, with `SNI_TLS_MODE=terminate`, terminated. The login is not read.
3. **TLS without a usable route**: if the protocol can carry a login (Redis, ClickHouse) and the proxy has a certificate, the proxy terminates TLS, reads the login and routes by username. Otherwise the connection is closed.
4. **Plaintext**: routed by username. MongoDB and tls close it.

## How a connection is decided (PostgreSQL)

PostgreSQL always reads the StartupMessage, after an SSLRequest upgrade or direct TLS. The login decides. Only if the login has no deployment and the client sent a server name is the server name used, with strategy sni.

## Examples

Redis Service reachable both ways, plain backend:

```bash
DATABASE_TYPE=redis SNI_TLS_MODE=terminate
# AUTH default.cache1 secret                              -> username route
# redis-cli --tls -h cache1.redis.example.com --pass x    -> sni route, proxy terminates
```

PostgreSQL deployment for a BI tool that can only set a host:

```yaml
labels:
  xdatabase-proxy-deployment-id: analytics
  xdatabase-proxy-routing: all
# psql "host=analytics.pg.example.com user=reporter sslmode=require"
```
