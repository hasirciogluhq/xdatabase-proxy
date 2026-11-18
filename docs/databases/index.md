---
title: Databases
nav_order: 6
has_children: true
---

# Databases

One page per protocol: how clients connect, what the proxy reads and rewrites, which TLS modes apply, and the known limits.

| Database | DATABASE_TYPE | Username routing | SNI routing | Default routing label | Test coverage |
| --- | --- | --- | --- | --- | --- |
| [PostgreSQL](postgresql.md) | postgresql | yes, StartupMessage | yes, if the label allows it | username | unit tests and e2e with real PostgreSQL 17/18 |
| [PostgreSQL-compatible](postgresql.md#postgresql-compatible-databases) | cockroachdb, yugabytedb, timescaledb, citus, greenplum, questdb, cratedb | yes | yes, if allowed | username | same code path, not run against the real products |
| [Redis](redis.md) | redis | yes, AUTH or HELLO … AUTH | yes | username and sni | unit tests and e2e with real Redis |
| [ClickHouse](clickhouse.md) | clickhouse | yes, native Hello packet | yes | username and sni | unit tests and e2e with real ClickHouse |
| [MongoDB](mongodb.md) | mongodb | no | yes, required | sni | e2e with real MongoDB 8 (passthrough and terminate) |
| [Generic TLS](mongodb.md#generic-tls-protocols) | tls | no | yes, required | sni | unit tests with a TLS backend |
| MySQL / MariaDB | – | – | – | – | planned |

Default ports in the examples: PostgreSQL 5432, Redis 6379, ClickHouse 9000, MongoDB 27017. The proxy listens on `PROXY_START_PORT` (default 5432) whatever the type, so set it to the port your clients expect.
