---
title: Getting started
nav_order: 2
---

# Getting started

Four ways to see the proxy work, from the quickest to a real cluster.

## 1. Docker, one PostgreSQL, no Kubernetes

```bash
docker network create xdp
docker run -d --name pg --network xdp -e POSTGRES_PASSWORD=secret postgres:17-alpine
docker run -d --name proxy --network xdp -p 5432:5432 -p 8080:8080 \
  -e STATIC_BACKENDS="app=pg:5432,app.pool=pg:5432" \
  ghcr.io/hasirciogluhq/xdatabase-proxy:2.2.0

psql "host=localhost port=5432 user=postgres.app dbname=postgres sslmode=require"   # password: secret
```

The login `postgres.app` means user `postgres`, deployment `app`. The proxy strips `.app`, so PostgreSQL sees `postgres`. With `sslmode=require` the session is encrypted with a self-signed certificate the proxy created in memory. `postgres.app.pool` takes the pooled route (here the same server).

Clean up with `docker rm -f proxy pg && docker network rm xdp`.

## 2. From source against a local PostgreSQL

You need Go (the version in `go.mod`) and a PostgreSQL on `localhost:5432`.

```bash
scripts/dev-run.sh
psql "host=localhost port=6432 user=postgres.dev dbname=postgres"
```

The script runs `go run ./cmd/proxy` on port 6432 with `STATIC_BACKENDS=dev=localhost:5432` and debug logs. Override `STATIC_BACKENDS` or `PROXY_START_PORT` in the environment to change that.

## 3. Redis in two minutes

```bash
docker network create xdp
docker run -d --name redis --network xdp redis:7-alpine redis-server --requirepass secret
docker run -d --name proxy-redis --network xdp -p 6379:6379 \
  -e DATABASE_TYPE=redis -e PROXY_START_PORT=6379 -e STATIC_BACKENDS="cache1=redis:6379" \
  ghcr.io/hasirciogluhq/xdatabase-proxy:2.2.0

redis-cli -h localhost --user default.cache1 --pass secret ping      # PONG
```

See [Redis](databases/redis.md) for TLS, RESP3 and SNI.

## 4. A local Kubernetes cluster

```bash
scripts/local-deploy.sh
kubectl -n xdatabase-proxy port-forward svc/xdatabase-proxy 6432:5432
psql "host=localhost port=6432 user=postgres.test-db dbname=db"      # password: 1234
```

The script builds the image as `ghcr.io/hasirciogluhq/xdatabase-proxy-development:development`, loads it into minikube, kind or k3d (other contexts must be able to pull it), applies `kubernetes/development` and the test PostgreSQL from `kubernetes/test/postgresql.yaml`, whose Service carries the discovery labels for deployment `test-db`.

## Check that it works

```bash
curl http://localhost:8080/health   # ok
curl http://localhost:8080/ready    # ready
```

The first log lines show the version and mode:

```json
{"level":"INFO","msg":"starting xdatabase-proxy","version":"v2.2.0","discovery":"static","tls":true}
{"level":"INFO","msg":"listening","port":"5432"}
```

`sslmode=verify-full` fails with the default self-signed certificate. Provide your own certificate for that, see [TLS](tls.md).

## Next steps

- Pick your database guide: [Databases](databases/index.md).
- Move to labelled Services: [Service discovery](service-discovery.md).
- Deploy for real: [Deployment](deployment/index.md).
