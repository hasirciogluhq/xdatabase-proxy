---
title: Security
nav_order: 12
---

# Security

## Threat model

The proxy sits on a network edge and talks to unauthenticated clients before any login. Everything a client sends before the backend is chosen is treated as hostile. The proxy does not authenticate users; it forwards the authentication exchange to the backend. It sees passwords only where the protocol sends them in the first message (Redis AUTH, ClickHouse Hello) and forwards them without logging. With SNI passthrough it never sees decrypted data at all.

## Built-in protections

| Area | Protection |
| --- | --- |
| PostgreSQL startup | Packets over 10000 bytes rejected before allocation; at most four negotiation packets, no recursion |
| Redis first command | At most 32 arguments and 4096 bytes in total; inline commands refused |
| ClickHouse Hello | Strings at most 4096 bytes |
| TLS ClientHello | Peeked at most one TLS record (16 KiB plus header) |
| Time | Startup, login or ClientHello must arrive within 10 seconds; backend resolve and dial bounded by `DIAL_TIMEOUT_SECONDS` |
| Load | `MAX_CONNECTIONS` caps concurrent sessions; extra connections are closed at once. Accept errors (such as running out of file descriptors) back off instead of crashing |
| Sessions | When one side closes, the other is closed too, so no half-open sessions pile up |
| Errors | Client messages never contain backend addresses or internal errors |
| Logs | Startup parameters, passwords and first messages are never logged |
| TLS | Minimum TLS 1.2, Go defaults for ciphers; private keys of generated certificates written with mode 0600 |

## Kubernetes permissions

The base RBAC grants:

- `list` and `watch` on Services, cluster wide (ClusterRole);
- `get` and `update` on the Secret `xdatabase-proxy-tls` and `create` on Secrets, in the proxy namespace only (Role).

Nothing else. Reduce it further:

- Use a namespaced Role instead of the ClusterRole when you set `WATCH_NAMESPACE`.
- Drop `create` and `update` when cert-manager manages the certificate, or remove the Role entirely with file or memory TLS.

## Who can route traffic

A label routes traffic. Anyone who can create or label a Service with `xdatabase-proxy-enabled: "true"` and a deployment id can attract connections for that deployment, including credentials sent in the first message. In shared clusters:

- restrict `services` create and patch rights,
- use `WATCH_NAMESPACE` so only one trusted namespace is watched,
- or enforce label ownership with an admission policy (Kyverno, Gatekeeper, ValidatingAdmissionPolicy),
- and watch for the `several services match one route` warning, which also reveals a conflicting Service.

## Pod hardening

The manifests run as non-root with a read-only root filesystem, no privilege escalation, all capabilities dropped and the RuntimeDefault seccomp profile. The image is distroless static, with no shell or package manager.

## Supply chain

- Direct dependencies are pgx (only pgproto3 is used) and client-go with its modules. `go mod verify` runs in CI and in the Dockerfile.
- govulncheck and a Trivy image scan run in CI. Dependabot proposes updates for Go modules, GitHub Actions and the base image.
- Release images are multi-architecture and carry an SBOM and build provenance attestation; verify with `gh attestation verify`.
- Pin the image by version or digest in production.

## Things you must handle

- When the proxy terminates TLS (PostgreSQL, username routing, `SNI_TLS_MODE=terminate`), the proxy to backend hop is plain TCP. Keep it on a trusted network or use a mesh. Only SNI passthrough keeps TLS end to end.
- Authentication is the backend's job. Use strong passwords, SCRAM, ACL users or certificates there.
- The default self-signed certificate protects against passive sniffing, not against an active attacker. Use a certificate clients verify.
- NetworkPolicies: allow clients to the proxy, the proxy to the databases, and deny clients direct access to the databases.

## Reporting a vulnerability

Open a private security advisory on the GitHub repository. See `SECURITY.md`.
