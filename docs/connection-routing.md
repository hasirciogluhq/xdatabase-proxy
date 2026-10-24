---
title: Connection routing
nav_order: 4
---

# Connection routing

## Login format

```
postgresql://USER.DEPLOYMENT[.pool]@PROXY_HOST:PORT/DATABASE
```

| Login | User sent to backend | Deployment | Pooled |
| --- | --- | --- | --- |
| alice.db1 | alice | db1 | no |
| alice.db1.pool | alice | db1 | yes |
| a.b.db1.pool | a.b | db1 | yes |
| alice | alice | none | no (no route matches, connection fails) |
| alice.pool | alice.pool | none | no (fails) |

The last dot separated part is the deployment, unless it is the word pool, in which case the part before it is the deployment. User names may contain dots.

## Routing by TLS server name (SNI)

When the login has no deployment part and the client connected with TLS, the proxy uses the TLS server name instead:

| Client connects to host | Login | Deployment | Pooled |
| --- | --- | --- | --- |
| db1.example.com | alice | db1 | no |
| db1.pool.example.com | alice | db1 | yes |

The login always wins when it contains a deployment. This works after the normal SSLRequest upgrade and also for PostgreSQL 17 direct TLS (`sslnegotiation=direct`, ALPN `postgresql`), which the proxy detects by the first byte of the connection. Direct TLS needs TLS enabled on the proxy. A wildcard DNS record such as `*.example.com` pointing at the proxy is enough.

## Database name

If the client sends no database, or the database equals the full login, the proxy sends postgres. Any other database name is forwarded as is.

## Direct and pooled endpoints

One deployment can expose two backends: a direct one (for migrations, long transactions, LISTEN) and a pooled one (for application traffic). The client chooses by adding .pool to the login.

## Errors a client can see

| Situation | Message | Code |
| --- | --- | --- |
| No route for the login | no backend for this user; expected user.deployment[.pool] | 08001 |
| Backend cannot be reached | backend unavailable | 08001 |
| Connection limit reached | connection closed without a message | none |
| Startup too slow or malformed | connection closed without a message | none |

Details are in the proxy log, not in the client message.

## Limitations

- Query cancellation from psql or drivers (CancelRequest) is not routed, because it carries only a process id and key. Cancel by connecting to the backend directly or with pg_cancel_backend.
- Protocol 3.0 and 3.2 are accepted.
- Every other startup parameter (options, application_name, client_encoding, ...) is forwarded unchanged, which PostgreSQL-compatible databases rely on.
