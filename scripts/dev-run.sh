#!/usr/bin/env bash
# Run the proxy locally against a PostgreSQL on localhost:5432.
# Connect with: psql "host=localhost port=6432 user=postgres.dev dbname=postgres"
set -euo pipefail
cd "$(dirname "$0")/.."
STATIC_BACKENDS="${STATIC_BACKENDS:-dev=localhost:5432}" \
PROXY_START_PORT="${PROXY_START_PORT:-6432}" \
DEBUG=true go run ./cmd/proxy
