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
| cmd/proxy/internal/postgres | Startup negotiation, username parsing, backend dial, session pipe |
| cmd/proxy/internal/discovery | Static table and Kubernetes informer, both return a backend address |
| cmd/proxy/internal/tlsstore | Certificate storage: file, Kubernetes Secret, memory |
| cmd/proxy/internal/health | Liveness and readiness HTTP endpoints |
| pkg/proxy | Generic TCP server with connection limit and graceful shutdown, plus the bidirectional pipe |
| pkg/tlsutil | Self-signed certificate generation and inspection |

The rule: pkg holds code that knows nothing about this product. internal holds product logic. A new database protocol would add a package next to postgres and reuse pkg/proxy.

External libraries: pgx pgproto3 for the PostgreSQL wire format, client-go for Kubernetes, slog from the standard library for logging.

## Life of a connection

1. pkg/proxy accepts the TCP connection. If MAX_CONNECTIONS is reached it closes the connection at once.
2. The postgres handler sets a 10 second deadline and reads the startup phase with pgproto3.
3. An SSLRequest is answered with S and a TLS handshake if TLS is enabled, otherwise with N. A GSSENCRequest is answered with N. At most four negotiation packets are accepted. A CancelRequest is rejected because it carries no user name.
4. The StartupMessage gives the user name. ParseUser splits it into user, deployment and pooled flag.
5. The resolver maps deployment and pooled flag to host:port. Kubernetes lookups are local memory reads.
6. The proxy dials the backend, bounded by DIAL_TIMEOUT_SECONDS, and sends a rewritten StartupMessage with the plain user name.
7. From here on the bytes are copied in both directions until either side closes. Authentication, queries and results pass through untouched.

Errors before step 7 are sent to the client as a PostgreSQL ErrorResponse with severity FATAL and code 08001.

## Startup message rewriting

- user becomes the name without the deployment suffix.
- database becomes postgres if it was empty or equal to the full login. psql does the latter when no database is given.
- Every other parameter is forwarded unchanged.

## Service discovery

The Kubernetes resolver runs a shared informer filtered by label selector (enabled and database type). An index keyed by deployment and pooled flag gives constant time lookup. Watch, relist and reconnect logic comes from client-go.

## Shutdown

On SIGTERM or SIGINT the listener closes, readiness turns to not ready, and running sessions get 25 seconds to finish. Remaining sessions are closed. Set terminationGracePeriodSeconds above 25 (the manifests use 40).
