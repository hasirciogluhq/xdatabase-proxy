#!/usr/bin/env bash
# Build the image locally, load it into the current local cluster (minikube/kind/k3d/docker-desktop)
# and deploy the development kustomization plus a test PostgreSQL.
set -euo pipefail
cd "$(dirname "$0")/.."
IMG=ghcr.io/hasirciogluhq/xdatabase-proxy-development:development
docker build -t "$IMG" .
ctx=$(kubectl config current-context)
case "$ctx" in
  *minikube*) minikube image load "$IMG" ;;
  *kind*)     kind load docker-image "$IMG" ;;
  *k3d*)      k3d image import "$IMG" ;;
esac
kubectl apply -k kubernetes/development
kubectl apply -n xdatabase-proxy -f kubernetes/test/postgresql.yaml
kubectl -n xdatabase-proxy rollout restart deployment/xdatabase-proxy
kubectl -n xdatabase-proxy rollout status deployment/xdatabase-proxy --timeout=120s
