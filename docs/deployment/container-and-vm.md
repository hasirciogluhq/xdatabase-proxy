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
  -e TLS_MODE=file -e TLS_CERT_FILE=/tls/tls.crt -e TLS_KEY_FILE=/tls/tls.key \
  -v /etc/xdatabase/tls:/tls:ro \
  ghcr.io/hasirciogluhq/xdatabase-proxy:latest
```

Notes:

- The image is distroless and runs as a non-root user. Mounted certificate files must be readable by it.
- With TLS_AUTO_GENERATE=true and a read-only mount the proxy cannot write a missing file. Use memory mode, or a writable volume.
- The container health check calls the built-in -healthcheck flag.

## VM or bare metal

Build the binary and run it under a supervisor:

```bash
CGO_ENABLED=0 go build -trimpath -o xdatabase-proxy ./cmd/proxy
```

Example systemd unit:

```
[Service]
ExecStart=/usr/local/bin/xdatabase-proxy
Environment=STATIC_BACKENDS=crm=10.1.0.4:5432
Environment=TLS_MODE=file
Environment=TLS_CERT_FILE=/etc/xdatabase/tls.crt
Environment=TLS_KEY_FILE=/etc/xdatabase/tls.key
User=xdatabase
Restart=always
```

Port 5432 is not privileged, so no extra capabilities are needed.

## Kubernetes discovery from outside a cluster

Set KUBECONFIG to a kubeconfig whose user can list and watch services, and optionally KUBE_CONTEXT. TLS mode kubernetes also works with it, with the Secret in the NAMESPACE variable. Mind that the proxy resolves backends by Service DNS name, so on a VM prefer static routing or make sure those names resolve.

## Compose example

```yaml
services:
  proxy:
    image: ghcr.io/hasirciogluhq/xdatabase-proxy:latest
    ports: ["5432:5432"]
    environment:
      STATIC_BACKENDS: app=postgres:5432
      TLS_MODE: memory
  postgres:
    image: postgres:17-alpine
    environment: { POSTGRES_PASSWORD: secret }
```

Connect with user postgres.app.
