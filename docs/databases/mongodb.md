---
title: MongoDB and TLS
parent: Databases
nav_order: 4
---

# MongoDB and generic TLS

`DATABASE_TYPE=mongodb` or `DATABASE_TYPE=tls`. Both behave the same way: they are routed only by TLS server name.

## Why only SNI

MongoDB clients start with a `hello` command and authenticate only after the server answered it, so there is no user name the proxy could read before choosing a backend. The TLS server name, sent in the very first packet, is the only routing key.

| Client connection | Result |
| --- | --- |
| TLS with SNI (connecting by host name) | routed |
| TLS without SNI (connecting by IP address) | closed |
| Plaintext | closed |
| Deployment in the user name | not possible |

## Setup

1. Pick a domain and point a wildcard DNS record at the proxy: `*.mongo.example.com`.
2. Give each deployment a route: `m1` → `m1.mongo.example.com`, pooled → `m1.pool.mongo.example.com`.
3. Run the proxy:

```bash
DATABASE_TYPE=mongodb
PROXY_START_PORT=27017
STATIC_BACKENDS="m1=10.0.0.9:27017"
```

or label the Services with `xdatabase-proxy-database-type: mongodb`.

4. Connect with TLS:

```bash
mongosh "mongodb://m1.mongo.example.com:27017/?tls=true&directConnection=true"
```

## Passthrough (default)

With `SNI_TLS_MODE=passthrough` the proxy forwards the encrypted bytes untouched. It needs no certificate (TLS settings are ignored and `TLS_ENABLED` is forced off), and the TLS session is end to end between client and mongod.

- mongod must run with `--tlsMode requireTLS` (or `preferTLS`).
- The client sees mongod's certificate. Its names must cover the routed host name (`*.mongo.example.com`), or the client must relax verification (`tlsAllowInvalidHostnames`, `tlsCAFile`, `tlsAllowInvalidCertificates` for tests).

## Terminate

With `SNI_TLS_MODE=terminate` the proxy completes TLS with its own certificate (see [TLS](../tls.md)) and talks plain TCP to the backend. mongod can run without TLS, and you manage one wildcard certificate on the proxy instead of one per database. Clients still need TLS with SNI.

## Replica sets

A replica set member advertises the host names in its configuration, and drivers that discover the topology then connect to those names directly, bypassing the proxy. Through the proxy use:

- `directConnection=true` to a single member, typically the Service of the primary maintained by your operator; or
- replica set members whose configured host names are themselves routed through the proxy (`rs0-0.mongo.example.com`, …), each as its own deployment.

`mongodb+srv://` works if the SRV records point at proxy-routed host names.

## Generic TLS protocols

`DATABASE_TYPE=tls` routes any protocol whose client starts with a TLS ClientHello carrying SNI: databases with TLS-only ports, message brokers, HTTPS, gRPC with TLS. Label Services with `xdatabase-proxy-database-type: tls`. Passthrough and terminate work as described above.

## Errors

Every failure (no SNI, unknown host, route not allowed, backend unreachable, plaintext) closes the connection. Drivers report a network or server selection timeout. Run with `DEBUG=true` to see `sni route unavailable` with the host name.

## Limits

- The ClientHello must fit in one TLS record (16 KiB), which all common clients do.
- Plaintext MongoDB cannot be routed at all.
