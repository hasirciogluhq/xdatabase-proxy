---
title: SNI routing
nav_order: 5
---

# SNI routing

For databases other than PostgreSQL the proxy routes by the server name (SNI) of the TLS handshake. It reads the ClientHello, picks the backend and forwards all bytes untouched. The backend terminates TLS, so the proxy needs no certificate and never sees plaintext.

## Enable

```
DATABASE_TYPE=mongodb      # or redis, clickhouse, tls
```

The names are equivalent for routing. The value only selects which Services are discovered through the label `xdatabase-proxy-database-type`.

## Host names

| Client connects to | Deployment | Pooled |
| --- | --- | --- |
| db1.mongo.example.com | db1 | no |
| db1.pool.mongo.example.com | db1 | yes |
| db1 | db1 | no |

The first label is the deployment. If the second label is pool the pooled route is used. Names are case insensitive. Create a wildcard DNS record such as `*.mongo.example.com` that points at the proxy.

## Requirements

- TLS must be enabled on the backend and the client must send SNI. Drivers do this by default when TLS is on.
- The client verifies the backend certificate, so it must cover the host name the client uses.
- Connections without SNI or with an unknown deployment are closed without a reply.

## Static example

```
DATABASE_TYPE=redis
STATIC_BACKENDS="cache1=10.0.0.7:6380"
```

Then connect with `redis-cli --tls -h cache1.redis.example.com`.

## Limits

- One proxy instance serves one database type. Run one instance per type, each on its own port or address.
- Protocols that start in plaintext and upgrade later (MySQL, MongoDB without TLS-first, Redis without TLS) cannot be routed this way.
