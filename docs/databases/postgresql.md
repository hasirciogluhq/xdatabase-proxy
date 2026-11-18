---
title: PostgreSQL
parent: Databases
nav_order: 1
---

# PostgreSQL

`DATABASE_TYPE=postgresql` (the default).

## Connecting

```
postgresql://alice.orders@proxy.example.com:5432/appdb          # direct endpoint of deployment "orders"
postgresql://alice.orders.pool@proxy.example.com:5432/appdb     # pooled endpoint
psql "host=proxy.example.com user=alice.orders dbname=appdb sslmode=require"
```

Any driver works, because the proxy only touches the StartupMessage: libpq/psql, pgx, JDBC, npgsql, node-postgres, asyncpg, psycopg, Prisma, SQLAlchemy, and tools such as pgAdmin or DBeaver. Put the full `user.deployment[.pool]` wherever the tool asks for the user name.

## What the proxy does

1. Waits up to 10 seconds for the startup phase.
2. If the first byte is `0x16`, performs a **direct TLS** handshake (PostgreSQL 17 `sslnegotiation=direct`, ALPN `postgresql`).
3. Answers `SSLRequest` with `S` and a TLS handshake when TLS is enabled, otherwise with `N`. An `SSLRequest` on an already encrypted connection gets `N`. `GSSENCRequest` always gets `N`, so clients fall back to TLS or plaintext. At most four negotiation packets are accepted.
4. Reads the StartupMessage and splits the user: `alice.orders.pool` → user `alice`, deployment `orders`, pooled.
5. If the login has no deployment and the client sent a TLS server name, uses the server name instead (see below).
6. Resolves and dials the backend within `DIAL_TIMEOUT_SECONDS`.
7. Sends a new StartupMessage with `user=alice`. If `database` was empty or equal to the full login (psql's default when no database is given), it becomes `postgres`. Every other parameter (`application_name`, `options`, `client_encoding`, `search_path`, …) is forwarded unchanged.
8. Copies bytes both ways. Password, SCRAM, MD5 and certificate authentication happen between client and backend.

Protocol versions 3.0 and 3.2 are accepted.

## Routing by server name

A client without a deployment in the login can be routed by host name:

```
psql "host=orders.pool.pg.example.com user=alice dbname=appdb sslmode=require"
```

This only works when:

- the client uses TLS (SSLRequest or direct TLS) and sends SNI, which libpq does for host names, not IP addresses;
- the Service allows sni: `xdatabase-proxy-routing: sni` or `all`. The PostgreSQL default is username only. With `STATIC_BACKENDS` every strategy is allowed.

A deployment in the login always wins over the server name. Point a wildcard record such as `*.pg.example.com` at the proxy.

## TLS

The proxy always terminates TLS for PostgreSQL; `SNI_TLS_MODE` does not apply and setting `SNI_TLS_MODE=terminate` with `postgresql` is a configuration error. The hop to the backend is plain TCP, so the backend needs no certificate (`ssl=off` is fine). With `TLS_ENABLED=false` every SSLRequest is declined and direct TLS connections are closed.

| Client sslmode | Works with default self-signed cert | Works with your own cert |
| --- | --- | --- |
| disable, allow, prefer | yes | yes |
| require | yes | yes |
| verify-ca | no | yes, with `sslrootcert` |
| verify-full | no | yes, if the certificate names match the host |

## Errors

| Situation | Client sees |
| --- | --- |
| Login without deployment (and no usable server name) | `FATAL: no backend for this user; expected user.deployment[.pool]` (SQLSTATE 08001) |
| Unknown deployment, or route does not allow the strategy | same |
| Backend unreachable | `FATAL: backend unavailable` (08001) |
| Startup slower than 10 s, malformed, too many negotiation packets | connection closed |
| CancelRequest | connection closed |
| Connection limit reached | connection closed |

## Poolers

Label the pooler Service with `xdatabase-proxy-pooled: "true"` and the same deployment id. pgbouncer, odyssey, pgcat and PgBouncer-compatible poolers work unchanged, because they receive a normal StartupMessage with the plain user. In transaction pooling mode, remember the usual pooler limits (prepared statements, session settings).

## Operators

CloudNativePG, Zalando, Crunchy PGO, StackGres and Patroni expose a read-write Service. Label that Service (or create an extra one with the same selector) and failover keeps working, because the proxy always dials the Service DNS name. A read-only Service can be exposed as its own deployment id, for example `orders-ro`.

## PostgreSQL-compatible databases

`DATABASE_TYPE` also accepts `cockroachdb`, `yugabytedb`, `timescaledb`, `citus`, `greenplum`, `questdb` and `cratedb`. They are treated exactly as `postgresql`; Services keep the label `xdatabase-proxy-database-type: postgresql`. Startup options such as CockroachDB's `options=--cluster=tenant-name` are forwarded unchanged. Aurora PostgreSQL, AlloyDB, Neon and similar managed services need nothing special when reached through static routing.

These products share the PostgreSQL code path and its tests, but have not been run against the real products. Reports are welcome.

## Limits

- **Query cancellation** (`CancelRequest`, Ctrl+C in psql, `statement.cancel()` in drivers) is not routed: it carries only a process id and secret, no user. Use `statement_timeout`, or `pg_cancel_backend()` from another session.
- **GSSAPI encryption** is declined; Kerberos authentication inside the session still works.
- The **replication protocol** works only if the login carries the deployment, like any other connection.
