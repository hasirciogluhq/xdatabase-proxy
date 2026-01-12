DATABASE_TYPE="postgresql" \
    RUNTIME="container" \
    DISCOVERY_MODE="static" \
    STATIC_BACKENDS="dblocal=localhost:5432" \
    PROXY_START_PORT="7878" \
    TLS_ENABLED="true" \
    TLS_MODE="file" \
    TLS_CERT_FILE="./development_data/tls.cert" \
    TLS_KEY_FILE="./development_data/tls.key" \
    TLS_AUTO_GENERATE="true" \
    DEBUG="true" \
    go run ./cmd/proxy/main.go