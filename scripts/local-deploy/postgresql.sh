#!/bin/bash

# Local build and deploy to Kubernetes cluster
# This script builds the image locally and loads it directly to the cluster
# without pulling from remote registry

set -e

IMAGE_TAG="xdatabase-proxy-local-test:local-test"

echo "🏗️  Building xdatabase-proxy locally..."
echo "Environment: local-test"
echo "Image: $IMAGE_TAG"    

# Build the Docker image locally
docker build -t "$IMAGE_TAG" -f Dockerfile .

echo "✅ Build complete!"

# Detect Kubernetes cluster type and load image
if kubectl config current-context | grep -q "minikube"; then
    echo "📦 Loading image to Minikube..."
    minikube image load "$IMAGE_TAG"
    echo "✅ Image loaded to Minikube"
elif kubectl config current-context | grep -q "kind"; then
    echo "📦 Loading image to Kind..."
    kind load docker-image "$IMAGE_TAG" --name general
    echo "✅ Image loaded to Kind"
elif kubectl config current-context | grep -q "k3d"; then
    echo "📦 Loading image to k3d..."
    k3d image import "$IMAGE_TAG"
    echo "✅ Image loaded to k3d"
elif kubectl config current-context | grep -q "docker-desktop"; then
    echo "📦 Docker Desktop detected - image is already available in local registry"
    echo "✅ Image available locally"
else
    echo "⚠️  Unknown cluster type: $(kubectl config current-context)"
    echo "Assuming local Docker registry is shared with cluster..."
    echo "✅ Image should be available locally"
fi



# Delete the DaemonSet to use the new image
echo "D}} Deleting xdatabase-proxy DaemonSet..."
kubectl delete daemonset/xdatabase-proxy -n xdatabase-proxy

sleep 1

# Apply Kubernetes manifests
echo "🚀 Deploying to Kubernetes..."
kubectl apply -f "kubernetes/examples/local-test/postgresql.yaml"

sleep 3

echo "⏳ Waiting for rollout to complete..."
kubectl rollout status daemonset/xdatabase-proxy -n xdatabase-proxy --timeout=120s

echo "✅ Deployment complete!"
echo ""
echo "📊 Pod status:"
kubectl get pods -n xdatabase-proxy -l app=xdatabase-proxy

echo ""
echo "📝 To view logs:"
echo "kubectl logs -f -n xdatabase-proxy -l app=xdatabase-proxy"
