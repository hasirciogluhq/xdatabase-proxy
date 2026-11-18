---
title: Architecture
nav_order: 3
---

# Architecture

## Components

| Package | Role |
| --- | --- |
| cmd/proxy/main.go | Wires everything, handles signals, builds the Kubernetes client |
| cmd/proxy/internal/config | Reads environment variables and validates them |
| cmd/proxy/internal/route | Routing key (deployment, pooled), strategies (username, sni), login and host name parsing, the Resolver contract |
| cmd/proxy/internal/postgres | PostgreSQL handler: startup negotiation, SSLRequest and direct TLS, username or SNI routing, backend dial, session pipe |
| cmd/proxy/internal/gateway | Handler for Redis, ClickHouse, MongoDB and tls: SNI passthrough or termination, otherwise TLS termination and username routing |
| cmd/proxy/internal/protocol | Reads and rewrites the login of Redis (AUTH, HELLO AUTH) and ClickHouse (native Hello) |
| cmd/proxy/internal/discovery | Static table and Kubernetes informer, both return a backend address |
| cmd/proxy/internal/tlsstore | Certificate storage: file, Kubernetes Secret, memory |
| cmd/proxy/internal/health | Liveness and readiness HTTP endpoints |
| pkg/proxy | Generic TCP server with connection limit and graceful shutdown, plus the bidirectional pipe |
| pkg/tlsutil | Self-signed certificate generation and inspection |

The rule: pkg holds code that knows nothing about this product. internal holds product logic. A protocol whose login arrives in the first client message is added as a Login reader in protocol and plugged into gateway; a protocol that needs its own negotiation gets a handler next to postgres. Both reuse route, discovery and pkg/proxy.

main.go picks one handler per process from DATABASE_TYPE: postgres for postgresql, gateway with the Redis or ClickHouse login reader for redis and clickhouse, gateway without a login reader for mongodb and tls.

External libraries: pgx pgproto3 for the PostgreSQL wire format, client-go for Kubernetes, slog from the standard library for logging.

## Life of a PostgreSQL connection

1. pkg/proxy accepts the TCP connection. If MAX_CONNECTIONS is reached it closes the connection at once.
2. The postgres handler sets a 10 second deadline. If the first byte is 0x16 it performs a direct TLS handshake (ALPN postgresql). Then it reads the startup phase with pgproto3.
3. An SSLRequest is answered with S and a TLS handshake if TLS is enabled, otherwise with N. A GSSENCRequest is answered with N. At most four negotiation packets are accepted. A CancelRequest is rejected because it carries no user name.
4. The StartupMessage gives the user name. route.ParseLogin splits it into user, deployment and pooled flag. If there is no deployment and the client sent a TLS server name, route.ParseHost takes the key from the host name and the strategy becomes sni.
5. The resolver maps deployment and pooled flag to host:port, provided the Service allows the strategy. Kubernetes lookups are local memory reads.
6. The proxy dials the backend, bounded by DIAL_TIMEOUT_SECONDS, and sends a rewritten StartupMessage with the plain user name.
7. From here on the bytes are copied in both directions until either side closes. Authentication, queries and results pass through untouched.

Errors before step 7 are sent to the client as a PostgreSQL ErrorResponse with severity FATAL and code 08001.

## Life of a gateway connection (Redis, ClickHouse, MongoDB, tls)

1. The gateway sets a 10 second deadline and peeks at the first bytes without consuming them.
2. If they are a TLS record, it peeks the whole ClientHello (at most one TLS record) and reads the server name. If a route exists for that name and allows sni, the connection is passed through (the ClientHello is replayed to the backend) or, with SNI_TLS_MODE=terminate, the proxy finishes the handshake and talks plain TCP to the backend.
3. Otherwise, if the protocol has a login reader and TLS is configured, the proxy terminates TLS and continues with step 4 on the decrypted stream. Without a login reader (mongodb, tls) the connection is closed.
4. The login reader parses the first message (Redis AUTH or HELLO AUTH, ClickHouse Hello), the user is split like in PostgreSQL, the backend is resolved with the username strategy, and the first message is re-encoded with the plain user and sent to the backend.
5. Bytes are copied in both directions. Redis clients get `-ERR` messages for routing errors, other protocols are closed.

## Startup message rewriting

- user becomes the name without the deployment suffix.
- database becomes postgres if it was empty or equal to the full login. psql does the latter when no database is given.
- Every other parameter is forwarded unchanged.

## Service discovery

The Kubernetes resolver runs a shared informer filtered by label selector (enabled and database type). An index keyed by deployment and pooled flag gives constant time lookup. Candidates are then filtered by the xdatabase-proxy-routing label; if more than one remains, the lowest namespace/name wins and a warning is logged once. Watch, relist and reconnect logic comes from client-go.

## Shutdown

On SIGTERM or SIGINT the listener closes, readiness turns to not ready, and running sessions get 25 seconds to finish. Remaining sessions are closed. Set terminationGracePeriodSeconds above 25 (the manifests use 40).
