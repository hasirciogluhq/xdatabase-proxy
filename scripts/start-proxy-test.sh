#!/bin/bash

# Test ortamı için PostgreSQL proxy'sini başlatan betik

DATABASE_TYPE="postgresql" \
    RUNTIME="container" \
    DISCOVERY_MODE="static" \
    STATIC_BACKENDS="testdb=localhost:5432" \
    PROXY_START_PORT="1881" \
    TLS_ENABLED="false" \
    DEBUG="true" \
    go run cmd/proxy/main.go
