---
title: Routing strategies
nav_order: 5
---

# Routing strategies

A connection can be mapped to a backend in two ways. Which ways are allowed for a route is decided by the Kubernetes administrator with a Service label.

| Strategy | Key comes from | Example |
| --- | --- | --- |
| username | The login in the first client message | `alice.db1.pool` selects deployment db1, pooled. The backend sees `alice` |
| sni | The TLS server name the client connected to | `db1.pool.example.com` selects deployment db1, pooled |

## Supported per database

| DATABASE_TYPE | username | sni | Default for Services without the label |
| --- | --- | --- | --- |
| postgresql (and cockroachdb, yugabytedb, timescaledb, citus, greenplum, questdb, cratedb) | yes | only if the label enables it; the proxy terminates TLS and uses the server name as the key | username |
| redis | yes (first command AUTH or HELLO with AUTH) | yes, passthrough | username and sni |
| clickhouse (native protocol) | yes (Hello packet) | yes, passthrough | username and sni |
| mongodb | no, the username arrives only after the server answered hello | yes, passthrough | sni |
| tls (any TLS-first protocol) | no | yes, passthrough | sni |
| mysql, mariadb | not supported | not supported | none |

One proxy instance serves one database type. Run one instance per type.

## The label

```yaml
metadata:
  labels:
    xdatabase-proxy-enabled: "true"
    xdatabase-proxy-database-type: redis
    xdatabase-proxy-deployment-id: cache1
    xdatabase-proxy-routing: username   # username | sni | all
```

- `username`: only login based routing may reach this Service.
- `sni`: only host name based routing may reach it.
- `all`: both.
- Missing label: the default of the database type, see the table above.

If several Services share a deployment, only those allowing the strategy used by the client are considered. With `STATIC_BACKENDS` every strategy is allowed.

## How a connection is decided

1. The proxy looks at the first bytes. A TLS handshake starts with 0x16 0x03.
2. TLS with a server name: if a route exists for that name and the label allows sni, the connection is passed through. The proxy never decrypts it and the backend terminates TLS, so the client must trust the backend certificate.
3. Otherwise, if the protocol can carry a login and the proxy has a certificate, the proxy terminates TLS itself, reads the login and routes by username. The hop to the backend is then plain TCP.
4. Plaintext connections are routed by username. For protocols without username routing (MongoDB, tls) plaintext is refused.

PostgreSQL always reads the username from its startup message. It negotiates TLS with SSLRequest, or direct TLS (PostgreSQL 17). The server name is used only when the login has no deployment and the label enables sni.

## Redis

The first command must carry the credentials:

```
AUTH alice.db1.pool secret
HELLO 3 AUTH alice.db1.pool secret
```

The proxy replaces the user with `alice` before forwarding and passes everything else through. Clients that send something else first (HELLO without AUTH, PING, a password-only AUTH, inline commands) get `-ERR` and are disconnected. Use the sni strategy for those clients.

With the default user and `requirepass`, log in as `default.db1`.

## ClickHouse

The native protocol Hello packet carries database, user and password. The user is rewritten like in Redis. The HTTP interface is not supported.

## MongoDB and other TLS databases

What works and what does not:

| Client connection | Routed? |
| --- | --- |
| TLS with SNI (host name, not an IP address) | yes |
| TLS without SNI (connecting by IP) | no, closed |
| Plaintext, no TLS | no, closed |
| Username in the login | not possible: MongoDB sends the user only after the server answered `hello` |

The proxy does not decrypt the traffic, so the client talks TLS with the backend and sees the backend's certificate. For a self-signed or private-CA certificate the client must trust it or relax verification (for example `tlsAllowInvalidCertificates` or `tlsCAFile`). Nothing has to be installed on the proxy.

Use SNI. Enable TLS on the backend and point a wildcard DNS name at the proxy:

```
DATABASE_TYPE=mongodb
STATIC_BACKENDS="db1=10.0.0.9:27017,db1.pool=10.0.0.9:27018"
```

Clients connect to `db1.mongo.example.com` or `db1.pool.mongo.example.com` with TLS enabled. The proxy needs no certificate.

## Errors

| Situation | Redis | Others |
| --- | --- | --- |
| Unroutable first message | `-ERR unsupported first message: ...` | connection closed |
| No deployment in login | `-ERR no backend for this user; expected user.deployment[.pool]` | PostgreSQL sends an ErrorResponse, others close |
| Route not allowed by label | same as no backend | same |
| Unknown or missing server name | connection closed | connection closed |
