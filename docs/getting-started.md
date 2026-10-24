---
title: Getting started
nav_order: 2
---

# Getting started

## Try it locally with a static backend

You need Go (the version in go.mod) and a PostgreSQL on localhost port 5432.

```bash
scripts/dev-run.sh
psql "host=localhost port=6432 user=postgres.dev dbname=postgres"
```

The script starts the proxy on port 6432 with the route `dev` pointing at `localhost:5432`. The login `postgres.dev` means user `postgres`, deployment `dev`.

## Try it in a local Kubernetes cluster

```bash
scripts/local-deploy.sh
kubectl -n xdatabase-proxy port-forward svc/xdatabase-proxy 6432:5432
psql "host=localhost port=6432 user=postgres.test-db dbname=db"
```

The script builds the image, loads it into minikube, kind or k3d, applies `kubernetes/development` and a test PostgreSQL whose Service carries the discovery labels. The test password is `1234`.

## Run the published image

```bash
docker run -p 5432:5432 \
  -e STATIC_BACKENDS="db1=10.0.0.5:5432,db1.pool=10.0.0.5:6432" \
  ghcr.io/hasirciogluhq/xdatabase-proxy:latest
```

Pin a version tag or digest for anything beyond a trial.

## Check that it works

```bash
curl http://localhost:8080/health   # ok
curl http://localhost:8080/ready    # ready
```

With TLS on, `psql ... sslmode=require` shows an encrypted session. `sslmode=verify-full` fails with the default self-signed certificate. See [TLS](tls.md).
