---
title: ClickHouse
parent: Databases
nav_order: 3
---

# ClickHouse

`DATABASE_TYPE=clickhouse`. Usually with `PROXY_START_PORT=9000` (or 9440 for TLS clients).

Only the **native TCP protocol** is supported. The HTTP interface (port 8123) is not.

## Connecting by username

The native Hello packet, the first client message, carries the database, user and password. Put the deployment in the user:

```bash
clickhouse-client --host proxy.example.com --port 9000 --user default.events --password secret
clickhouse-client --host proxy.example.com --port 9000 --user default.events.pool --password secret
clickhouse-client --host proxy.example.com --port 9440 --secure --accept-invalid-certificate --user default.events --password secret
```

The proxy rewrites the user to `default` and forwards the Hello packet with everything else unchanged (client name, version, revision, database, password). Native-protocol drivers such as clickhouse-go (native mode), clickhouse-driver (Python) and the Rust/C++ clients work the same way.

Hello packets with strings longer than 4096 bytes are rejected.

## Connecting by server name

With TLS and a host name that has a route, the connection is routed by SNI and the Hello packet is not inspected:

```bash
clickhouse-client --host events.ch.example.com --port 9440 --secure --user default --password secret
```

`SNI_TLS_MODE=passthrough` (default) needs TLS on the backend (`tcp_port_secure`). `SNI_TLS_MODE=terminate` lets the backend run plain `tcp_port`. If the host name has no route, the proxy terminates TLS itself and routes by the user in Hello.

## Errors

ClickHouse has no error message the proxy could send before the handshake, so every routing failure closes the connection. clickhouse-client reports this as `Connection reset by peer` or `Unexpected EOF`. The reason is in the proxy log (`resolve failed`, `dial failed`, `login read failed`).

## Limits

- HTTP interface not supported.
- Distributed tables and `remote()` reach other nodes from the server side, not through the proxy, which is fine.
- Username routing is covered by unit tests and an end-to-end test with the official ClickHouse image; report clients that send something unusual before Hello.
