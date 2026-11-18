---
title: Container and VM
parent: Deployment
nav_order: 2
---

# Container and VM

## Container

```bash
docker run -d --name xdatabase-proxy --restart unless-stopped \
  -p 5432:5432 -p 8080:8080 \
  -e STATIC_BACKENDS="db1=10.0.0.5:5432,db1.pool=10.0.0.5:6432" \
  -e TLS_CERT_FILE=/tls/tls.crt -e TLS_KEY_FILE=/tls/tls.key -e TLS_AUTO_GENERATE=false \
  -v /etc/xdatabase/tls:/tls:ro \
  ghcr.io/hasirciogluhq/xdatabase-proxy:2.2.0
```

Notes:

- The image is distroless and runs as the non-root user 65532. Mounted certificate files must be readable by it.
- With a read-only mount, set `TLS_AUTO_GENERATE=false`; otherwise a missing file would make the proxy try to write a self-signed pair and fail. For trials, leave the TLS variables out and memory mode is used.
- `docker ps` shows the health state from the built-in `-healthcheck`.
- Logs are JSON on standard output: `docker logs -f xdatabase-proxy`.

## Compose

```yaml
services:
  proxy:
    image: ghcr.io/hasirciogluhq/xdatabase-proxy:2.2.0
    ports: ["5432:5432"]
    environment:
      STATIC_BACKENDS: app=postgres:5432,app.pool=pgbouncer:6432
  postgres:
    image: postgres:17-alpine
    environment: { POSTGRES_PASSWORD: secret }
  pgbouncer:
    image: edoburu/pgbouncer:latest
    environment:
      DATABASE_URL: postgres://postgres:secret@postgres:5432/postgres
      AUTH_TYPE: scram-sha-256
```

Connect with user `postgres.app` (direct) or `postgres.app.pool` (pooled).

Several protocols on one host:

```yaml
services:
  proxy-pg:
    image: ghcr.io/hasirciogluhq/xdatabase-proxy:2.2.0
    ports: ["5432:5432"]
    environment: { STATIC_BACKENDS: "app=postgres:5432" }
  proxy-redis:
    image: ghcr.io/hasirciogluhq/xdatabase-proxy:2.2.0
    ports: ["6379:6379"]
    environment: { DATABASE_TYPE: redis, PROXY_START_PORT: "6379", HEALTH_SERVER_PORT: "8081", STATIC_BACKENDS: "cache=redis:6379" }
```

## VM or bare metal

Build the binary (or copy it out of the image):

```bash
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=v2.2.0" -o xdatabase-proxy ./cmd/proxy
# or
docker create --name x ghcr.io/hasirciogluhq/xdatabase-proxy:2.2.0 && docker cp x:/xdatabase-proxy . && docker rm x
```

systemd unit:

```ini
[Unit]
Description=XDatabase Proxy
After=network-online.target
Wants=network-online.target

[Service]
ExecStart=/usr/local/bin/xdatabase-proxy
Environment=STATIC_BACKENDS=crm=10.1.0.4:5432,billing=10.1.0.9:5432
Environment=TLS_CERT_FILE=/etc/xdatabase/tls.crt
Environment=TLS_KEY_FILE=/etc/xdatabase/tls.key
Environment=TLS_AUTO_GENERATE=false
User=xdatabase
Restart=always
LimitNOFILE=65536
TimeoutStopSec=40
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true

[Install]
WantedBy=multi-user.target
```

Ports 5432, 6379, 9000 and 27017 are not privileged, so no capabilities are needed. Raise `LimitNOFILE` above `2 × MAX_CONNECTIONS` for large fleets. `TimeoutStopSec` must exceed the 25 second drain.

## Kubernetes discovery from outside a cluster

Set `KUBECONFIG` to a kubeconfig whose user can list and watch Services, and optionally `KUBE_CONTEXT`. TLS mode kubernetes also works with it, with the Secret in `NAMESPACE`. The proxy dials backends by Service DNS name (`name.namespace.svc.cluster.local`), so on a VM those names must resolve and be reachable (for example through a VPN with cluster DNS); otherwise prefer static routing.
