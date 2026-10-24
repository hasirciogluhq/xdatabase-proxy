---
title: Security
nav_order: 10
---

# Security

## Threat model

The proxy sits on a network edge and speaks to unauthenticated clients before any login. It treats all startup data as hostile. It does not see passwords in clear text beyond forwarding the authentication exchange, and it never logs startup parameters.

## Built-in protections

- Startup packets over 10000 bytes are rejected before allocation.
- A startup must complete in 10 seconds. Slow clients are dropped.
- At most four negotiation packets (SSL, GSS) per connection, and no recursion.
- MAX_CONNECTIONS caps concurrent sessions.
- Backend dial has a timeout.
- Error messages to clients never contain backend addresses or internal errors.
- When one side of a session closes, the other is closed too.

## Kubernetes permissions

The base RBAC grants list and watch on Services, and get, update and create on the one TLS Secret in the proxy namespace. Nothing else. Remove the Role if you manage the certificate yourself, and use a namespaced Role instead of the ClusterRole when you set WATCH_NAMESPACE.

## Pod hardening

The manifests run as non-root with a read-only root filesystem, no privilege escalation, all capabilities dropped and the RuntimeDefault seccomp profile. The image is distroless.

## Supply chain

- Dependencies are limited to pgx (pgproto3) and client-go with its modules. go mod verify runs in CI.
- govulncheck and a Trivy image scan run in CI. Dependabot proposes updates for Go modules, GitHub Actions and the base image.
- Release images are multi-architecture and carry an SBOM and build provenance attestation.
- Pin the image by version or digest in production.

## Things you must handle

- The proxy to backend hop is not encrypted.
- Authentication is the backend's job. Use strong passwords or certificates there.
- Restrict who can create Services with the discovery labels, because a label routes traffic. In shared clusters use WATCH_NAMESPACE or admission policies.

## Reporting a vulnerability

Open a private security advisory on the GitHub repository.
