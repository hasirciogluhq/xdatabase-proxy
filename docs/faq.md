---
title: FAQ
nav_order: 15
---

# FAQ

**Does the proxy pool connections?**
No. It routes and pipes. Put pgbouncer, odyssey or pgcat behind the pooled endpoint.

**Does it load balance between replicas?**
No. It dials one address per route. Point the route at a Service and Kubernetes spreads connections over its endpoints, or at a read-only Service for replicas.

**Does it add latency to queries?**
Only at connect time (one resolve from memory, one dial, TLS handshake). After that it copies bytes; there is no per-query work.

**Can one proxy serve PostgreSQL and Redis?**
No. One process serves one `DATABASE_TYPE`. Run one per protocol, each on its own port.

**Why can't MongoDB be routed by user name?**
MongoDB clients send the user only after the server answered `hello`. The proxy must pick the server before that, so only the TLS server name is available. See [MongoDB](databases/mongodb.md).

**My client cannot change the user name. What now?**
Use SNI routing: connect to `deployment.your-domain` with TLS and set `xdatabase-proxy-routing: sni` or `all` on the Service.

**Does `sslmode=verify-full` work?**
With your own certificate whose names match the host clients use. The default self-signed certificate is only for `require`.

**Can clients authenticate with client certificates?**
Only through SNI passthrough, where TLS reaches the backend untouched. When the proxy terminates TLS it does not request client certificates.

**Is traffic encrypted between proxy and database?**
Only with SNI passthrough. Otherwise the proxy terminates TLS and the backend hop is plain TCP.

**Why does psql's Ctrl+C not cancel my query?**
PostgreSQL sends cancellation on a new connection that carries only a process id and secret, no user, so it cannot be routed. Use `statement_timeout` or `pg_cancel_backend()`.

**Two Services have the same deployment id. Which wins?**
Among those allowed for the client's strategy and pooled flag, the lowest `namespace/name`. A warning names all candidates.

**Do I need to restart the proxy when Services change?**
No for Kubernetes discovery; changes apply within a second. Yes for `STATIC_BACKENDS`.

**Do I need to restart when the certificate rotates?**
No. It is re-read every 6 hours. Restart to pick up a new one sooner.

**Is there a Helm chart or metrics endpoint?**
Not yet. Manifests are Kustomize; health is exposed on `/health` and `/ready`.

**Is it free for commercial use?**
Yes, Apache License 2.0, with no fees or revenue limits.
