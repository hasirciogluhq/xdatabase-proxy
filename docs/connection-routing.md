---
title: Connection routing
nav_order: 4
---

# Connection routing

How a login or a host name becomes a route key. The rules are the same for every database; protocol details are in [Databases](databases/index.md).

## Login format

```
USER.DEPLOYMENT[.pool]
```

The last dot separated part is the deployment, unless it is the word `pool`; then the part before it is the deployment and the route is pooled. Everything before the deployment is the user sent to the backend, so user names may contain dots.

| Login | User sent to backend | Deployment | Pooled |
| --- | --- | --- | --- |
| alice.db1 | alice | db1 | no |
| alice.db1.pool | alice | db1 | yes |
| a.b.db1 | a.b | db1 | no |
| a.b.db1.pool | a.b | db1 | yes |
| alice | alice | none, not routable by username | |
| alice.pool | alice.pool | none, not routable by username | |

Deployment names in logins are case sensitive and must match the label or `STATIC_BACKENDS` key exactly.

## Host name format

```
DEPLOYMENT[.pool].ANY.DOMAIN
```

The first label of the TLS server name is the deployment. If the second label is `pool` and at least one more label follows, the route is pooled. Host names are compared in lower case and a trailing dot is ignored.

| Server name | Deployment | Pooled |
| --- | --- | --- |
| db1.example.com | db1 | no |
| db1.pool.example.com | db1 | yes |
| DB1.Pool.Example.com. | db1 | yes |
| db1 | db1 | no |
| db1.pool | db1 | no (needs a domain after pool) |

Because names are lower cased, deployment ids used for SNI routing must be lower case. A wildcard DNS record such as `*.db.example.com` pointing at the proxy covers every deployment and its pooled variant (`*.db.example.com` matches `db1.pool.db.example.com` in DNS, but a TLS certificate needs `*.pool.db.example.com` as an extra name if the proxy terminates TLS).

## Which one is used

| Database | Order |
| --- | --- |
| PostgreSQL | Login first. The server name only when the login has no deployment |
| Redis, ClickHouse | Server name first if the client uses TLS and a route allows sni. Otherwise the login |
| MongoDB, tls | Server name only |

Details and the label that controls this: [Routing strategies](routing-strategies.md).

## Database name (PostgreSQL)

If the client sends no database, or the database equals the full login, the proxy sends `postgres`. psql does the latter when no `dbname` is given. Any other database name is forwarded as is. Redis database numbers and ClickHouse databases are never changed.

## Direct and pooled endpoints

One deployment can expose two backends: a direct one (migrations, long transactions, LISTEN/NOTIFY, admin work) and a pooled one (application traffic). The client chooses with `.pool` in the login or the host name. A missing pooled endpoint is not replaced by the direct one: `alice.db1.pool` fails if only `db1` exists.

## Errors a client can see

| Situation | PostgreSQL | Redis | ClickHouse, MongoDB, tls |
| --- | --- | --- | --- |
| No deployment in login | `no backend for this user; expected user.deployment[.pool]` (08001) | `-ERR no backend …` | closed |
| Unknown deployment or route not allowed | same | same | closed |
| Backend unreachable | `backend unavailable` (08001) | `-ERR backend unavailable` | closed |
| Unroutable first message | closed | `-ERR unsupported first message: …` | closed |
| Too slow (10 s), malformed, connection limit | closed | closed | closed |

Messages never contain backend addresses. Details are in the proxy log, see [Log messages](reference/log-messages.md).
