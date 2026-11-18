---
title: Architecture
nav_order: 17
---

# Architecture

## Components

| Package | Role |
| --- | --- |
| cmd/proxy/main.go | Wires everything: configuration, health server, Kubernetes client, resolver, certificate refresh, handler choice, signals |
| cmd/proxy/internal/config | Reads environment variables, applies defaults and validates combinations |
| cmd/proxy/internal/route | Route key (deployment, pooled), strategies (username, sni), `ParseLogin`, `ParseHost`, the `Resolver` interface |
| cmd/proxy/internal/postgres | PostgreSQL handler: direct TLS, SSLRequest and GSSENCRequest negotiation, StartupMessage rewrite, error responses |
| cmd/proxy/internal/gateway | Handler for Redis, ClickHouse, MongoDB and tls: ClientHello peek, SNI passthrough or termination, otherwise TLS termination and username routing |
| cmd/proxy/internal/protocol | Login readers: Redis (AUTH, HELLO AUTH, RESP encoding, `-ERR` replies) and ClickHouse (native Hello) |
| cmd/proxy/internal/discovery | Static table and Kubernetes informer resolver with route index, routing label filter and ambiguity warning |
| cmd/proxy/internal/tlsstore | Certificate stores (file, Kubernetes Secret, memory) and `Ensure`: load, generate or renew |
| cmd/proxy/internal/health | `/health` and `/ready` |
| pkg/proxy | Protocol-agnostic TCP server: connection limit, accept back-off, graceful drain, bidirectional `Pipe` |
| pkg/tlsutil | Self-signed certificate generation and inspection |

Rule: `pkg` holds code that knows nothing about this product; `cmd/proxy/internal` holds product logic. `pkg` never imports `internal`.

External libraries: pgx `pgproto3` for the PostgreSQL wire format, client-go for Kubernetes, `log/slog` for logging. Redis RESP and ClickHouse Hello parsing are small, bounded readers in `protocol`.

```
                    ┌────────────── main.go ──────────────┐
 client ─TCP─▶ pkg/proxy.Server ─▶ Handler (one per DATABASE_TYPE)
                                    ├─ postgres.Handler
                                    └─ gateway.Handler ─ Login: protocol.Redis | protocol.ClickHouse | none
                                          │
                                   route.Resolver
                                    ├─ discovery.Static
                                    └─ discovery.Kubernetes (informer + index)
                                          │
                                   backend host:port ─TCP─▶ database
```

## Handler choice

| DATABASE_TYPE | Handler | Login reader | Default strategies |
| --- | --- | --- | --- |
| postgresql | postgres.Handler | (StartupMessage) | username |
| redis | gateway.Handler | protocol.Redis | username, sni |
| clickhouse | gateway.Handler | protocol.ClickHouse | username, sni |
| mongodb, tls | gateway.Handler | none | sni |

## Life of a PostgreSQL connection

1. `pkg/proxy` accepts the connection. At `MAX_CONNECTIONS` it closes it at once.
2. The handler sets a 10 second deadline. If the first byte is `0x16`, it performs a direct TLS handshake with ALPN `postgresql`.
3. It reads startup packets with pgproto3. SSLRequest gets `S` and a TLS handshake when TLS is enabled and the connection is not yet encrypted, otherwise `N`. GSSENCRequest gets `N`. After four packets, or on CancelRequest, the connection is closed.
4. `route.ParseLogin` splits the user. Without a deployment and with a TLS server name, `route.ParseHost` provides the key and the strategy becomes sni.
5. The resolver returns `host:port` if a route exists and allows the strategy.
6. The proxy dials within `DIAL_TIMEOUT_SECONDS` and sends a re-encoded StartupMessage with the plain user (and `postgres` as database when it was empty or equal to the login).
7. The deadline is cleared and `Pipe` copies bytes both ways until either side closes.

Errors before step 7 reach the client as an ErrorResponse with severity FATAL and SQLSTATE 08001 where possible.

## Life of a gateway connection

1. The handler sets a 10 second deadline and peeks at five bytes without consuming them.
2. **TLS record** (`0x16 0x03`): it peeks the whole first record (at most 16 KiB plus header) and extracts the server name by feeding it to a `crypto/tls` server that stops right after reading the ClientHello.
   - Route exists and allows sni: with passthrough, dial the backend, replay the buffered bytes, pipe. With terminate, finish the handshake with the proxy certificate, dial, pipe plaintext.
   - Otherwise, if there is a login reader and a certificate: terminate TLS and continue on the decrypted stream.
   - Otherwise close.
3. **Plaintext**: continue if there is a login reader, else close.
4. The login reader parses the first message and returns the user plus a function that re-encodes the message with another user.
5. `route.ParseLogin`, resolve with strategy username, dial, send the re-encoded first message. Bytes the client already sent after it (pipelined commands) are still in the buffer and follow.
6. `Pipe` until either side closes. Routing errors become `-ERR` replies for Redis; other protocols are closed.

## Service discovery

`discovery.Kubernetes` runs one shared informer for Services, filtered server side by the label selector `xdatabase-proxy-enabled=true,xdatabase-proxy-database-type=<type>` and optionally by namespace, with a 10 minute resync. An index keyed by `deployment|pooled` gives constant time lookup. A lookup filters candidates by the routing label, sorts them by `namespace/name`, warns once per distinct candidate set when more than one remains, and picks the destination port. Watch, relist and reconnect come from client-go. `NewKubernetes` blocks until the first sync, so the proxy is never ready with an empty table.

## Certificates

`tlsstore.Ensure` loads the certificate from the store. If loading fails and auto generation is on, or the certificate is the proxy's own self-signed one and expires within the renewal window, it generates a new one and saves it (Kubernetes: create if missing, update if renewing; concurrent replicas converge on the first writer), then loads again. `main.go` holds the result in an atomic pointer, serves it through `tls.Config.GetCertificate`, and repeats `Ensure` every 6 hours.

## Shutdown

SIGTERM or SIGINT cancels the root context. The listener closes, running sessions get 25 seconds, the remainder are closed, readiness turns off and the health server stops within 5 seconds. Keep `terminationGracePeriodSeconds` above 25 (the manifests use 40).
