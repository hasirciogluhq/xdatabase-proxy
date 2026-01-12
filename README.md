# XDatabase Proxy

![xdatabase-proxy v2.0.0 banner](static/images/xdatabase-proxy-en.jpeg)

> **XDatabase Proxy in Action:**
> The screenshot demonstrates a successful, secure PostgreSQL connection established through xdatabase-proxy. The proxy automatically generates and manages TLS certificates, ensuring encrypted traffic between your client and the database. This seamless integration with Kubernetes and real-time certificate handling provides both security and ease of use for your cloud-native database workloads.

XDatabase Proxy is a production-grade, enterprise-ready proxy solution for database deployments. Designed with flexibility in mind, it runs seamlessly in Kubernetes clusters, containers, VMs, or bare-metal environments.

## Features

- 🔄 **Dynamic Service Discovery**: Automatic backend discovery via Kubernetes API or static configuration
- 🎯 **Deployment-Based Routing**: Route connections based on deployment IDs
- 🌊 **Connection Pooling Support**: Works with any pooler (pgbouncer, odyssey, etc.)
- 🚀 **Multi-Runtime Support**: Kubernetes, Container, VM, or Bare-Metal deployments
- 📊 **Smart Load Balancing**: Intelligent routing between backends
- 🔍 **Real-Time Monitoring**: Live service discovery and health checks
- 🔀 **Multi-Node Cluster Support**: Works with any cluster manager (pgpool-II, patroni, etc.)
- 🔒 **Enterprise TLS/SSL**:
  - Automatic certificate generation and renewal
  - Certificate expiration monitoring
  - Multiple certificate sources (file, Kubernetes secret, memory)
  - Self-signed certificate support for development
- 🏷️ **Label-Based Configuration**: No hard dependencies on specific implementations
- 🔌 **Flexible Discovery**: Kubernetes API or static backend configuration
- 🩺 **Health Check Endpoints**: Built-in health and readiness checks
- 🪵 **Structured Logging**: JSON-formatted logs with debug mode
- 🏗️ **Production-Grade Architecture**: Factory pattern, dependency injection, configuration-driven

## Supported Databases

| Database   | Status          |
| ---------- | --------------- |
| PostgreSQL | ✅ Full Support |
| MySQL      | 📋 Planned      |
| MongoDB    | 📋 Planned      |

## Requirements

- Go 1.23.4 or higher
- Kubernetes cluster (optional - for Kubernetes discovery mode)
- kubectl configuration (optional - for remote Kubernetes access)

## Installation

```bash
# Clone the project
git clone https://github.com/hasirciogluhq/xdatabase-proxy.git
cd xdatabase-proxy

# Install dependencies
go mod download

# Build the project
go build -o xdatabase-proxy cmd/proxy/main.go
```

## Configuration

### Environment Variables

#### Core Configuration

| Variable        | Description                                    | Required | Default    | Example Value |
| --------------- | ---------------------------------------------- | -------- | ---------- | ------------- |
| DATABASE_TYPE   | Database type to proxy                         | No       | postgresql | postgresql    |
| PROXY_START_PORT| Port for proxy listener                        | No       | 5432       | 5432          |
| HEALTH_SERVER_PORT | Health check server port                    | No       | 8080       | 8080          |
| DEBUG           | Enable debug logging                           | No       | false      | true          |

#### Runtime Configuration

| Variable  | Description                                                                                      | Required | Default      | Example Value | When to Use |
| --------- | ------------------------------------------------------------------------------------------------ | -------- | ------------ | ------------- | ----------- |
| RUNTIME   | Execution environment: `kubernetes`, `container`, `vm`                                           | No       | Auto-detect  | kubernetes    | Set explicitly only if auto-detection fails |
| NAMESPACE | Kubernetes namespace                                                                             | Conditional | default   | production    | **Required** when `RUNTIME=kubernetes` OR `TLS_MODE=kubernetes` |

**Runtime Auto-Detection:**
- `kubernetes`: Detected if `/var/run/secrets/kubernetes.io/serviceaccount` exists
- `container`: Detected if `/.dockerenv` exists
- `vm`: Default fallback

**Configuration Rules:**
- ✅ If `RUNTIME=kubernetes`: `NAMESPACE` is **mandatory** for service discovery
- ✅ If `RUNTIME=container|vm` + `TLS_MODE=kubernetes`: `NAMESPACE` is **mandatory** for TLS secret access
- ✅ If `RUNTIME=container|vm` + `TLS_MODE=file|memory`: `NAMESPACE` is optional

#### Backend Discovery

| Variable         | Description                                                                            | Required | Default      | Example Value                           | When to Use |
| ---------------- | -------------------------------------------------------------------------------------- | -------- | ------------ | --------------------------------------- | ----------- |
| DISCOVERY_MODE   | Discovery strategy: `kubernetes` or `static`                                           | No       | kubernetes   | static                                  | Auto-set to `static` if `STATIC_BACKENDS` is provided |
| STATIC_BACKENDS  | Static backend mapping (`deployment_id[.pool]=host:port` comma-separated)              | Conditional | -         | db1=10.0.1.5:5432,db1.pool=10.0.1.5:6432 | **Required** when not using Kubernetes discovery |
| KUBECONFIG       | Path to kubeconfig file                                                                | Conditional | ~/.kube/config | /path/to/config                    | **Required** when `DISCOVERY_MODE=kubernetes` AND running outside cluster (VM/Container) |
| KUBE_CONTEXT     | Kubernetes context name                                                                | No       | -            | production-cluster                      | Use for multi-cluster setups with kubeconfig |

**Discovery Modes:**
- **kubernetes**: Dynamic discovery via Kubernetes API
  - Works from inside Kubernetes (in-cluster) 
  - Works from outside Kubernetes (with KUBECONFIG)
  - Can run in VM/Container and connect to remote Kubernetes
- **static**: Static backend list (no Kubernetes dependency)

**Configuration Rules:**
- ✅ **In Kubernetes Pod**: `DISCOVERY_MODE=kubernetes` (default, uses in-cluster config)
- ✅ **VM/Container → Remote K8s**: `DISCOVERY_MODE=kubernetes` + `KUBECONFIG=/path/to/config`
- ✅ **Static Backends**: `STATIC_BACKENDS='db1=host:5432,db1.pool=host:6432'` (auto-sets `DISCOVERY_MODE=static`)
- ⚠️ **Cannot mix**: Cannot use both `STATIC_BACKENDS` and `DISCOVERY_MODE=kubernetes` at same time
- ⚠️ **KUBECONFIG required**: If `DISCOVERY_MODE=kubernetes` + not in cluster → must provide `KUBECONFIG`
- ⚠️ **NAMESPACE required**: If `DISCOVERY_MODE=kubernetes` → must provide `NAMESPACE`

**Static Backends Format:**
- `deployment_id=host:port` → direct connections
- `deployment_id.pool=host:port` → pooled connections (optional)
- Multiple entries comma-separated, e.g. `db1=10.0.1.5:5432,db1.pool=10.0.1.5:6432`

#### TLS/SSL Configuration

| Variable                     | Description                                                                    | Required | Default | Example Value       | When to Use |
| ---------------------------- | ------------------------------------------------------------------------------ | -------- | ------- | ------------------- | ----------- |
| TLS_ENABLED                  | Enable/disable TLS completely                                                  | No       | true    | false               | Set to `false` for development or internal non-encrypted networks |
| TLS_MODE                     | TLS provider: `file`, `kubernetes`, `memory`                                   | No       | Auto    | kubernetes          | Auto-detected based on other TLS settings |
| TLS_CERT_FILE                | Path to TLS certificate file                                                   | Conditional | -    | /certs/tls.crt      | **Required** when `TLS_MODE=file` AND `TLS_AUTO_GENERATE=false` |
| TLS_KEY_FILE                 | Path to TLS private key file                                                   | Conditional | -    | /certs/tls.key      | **Required** when `TLS_MODE=file` AND `TLS_AUTO_GENERATE=false` |
| TLS_SECRET_NAME              | Kubernetes secret name for TLS certificate                                     | Conditional | -    | xdatabase-proxy-tls | **Required** when `TLS_MODE=kubernetes` |
| TLS_AUTO_GENERATE            | Generate self-signed certificate if none exists                                | No       | true    | true                | Recommended `true` for development, `false` for production with real certs |
| TLS_AUTO_RENEW               | Automatically renew certificate if expired or invalid                          | No       | true    | false               | Set `false` if using externally managed certificates |
| TLS_RENEWAL_THRESHOLD_DAYS   | Days before expiry to trigger renewal                                          | No       | 30      | 60                  | Adjust based on cert renewal process |

**TLS Mode Auto-Detection:**
1. `file`: When `TLS_CERT_FILE` is set
2. `kubernetes`: When `TLS_SECRET_NAME` is set
3. `memory`: Default fallback (in-memory certificate)

**TLS Certificate Lifecycle:**
- If certificate doesn't exist and `TLS_AUTO_GENERATE=true`: Generate new self-signed certificate
- If certificate is invalid/expired and `TLS_AUTO_RENEW=true`: Regenerate certificate
- Kubernetes secret automatically created if it doesn't exist
- Multi-instance safe: Race condition handling for concurrent pod startups

**Configuration Rules:**
- ✅ **No TLS**: `TLS_ENABLED=false` → All other TLS settings ignored
- ✅ **Auto TLS in K8s**: `TLS_MODE=kubernetes` + `TLS_SECRET_NAME=my-tls` + `TLS_AUTO_GENERATE=true` → Auto-creates secret
- ✅ **Existing K8s Secret**: `TLS_MODE=kubernetes` + `TLS_SECRET_NAME=existing-tls` + `TLS_AUTO_GENERATE=false`
- ✅ **File-based TLS**: `TLS_MODE=file` + `TLS_CERT_FILE=/path/cert` + `TLS_KEY_FILE=/path/key`
- ✅ **Auto-generated File TLS**: `TLS_MODE=file` + `TLS_AUTO_GENERATE=true` → Creates certs in `./development_data/`
- ✅ **Memory TLS**: `TLS_MODE=memory` + `TLS_AUTO_GENERATE=true` → In-memory self-signed cert
- ⚠️ **TLS_MODE=file + No files**: Must have `TLS_AUTO_GENERATE=true` OR provide `TLS_CERT_FILE` + `TLS_KEY_FILE`
- ⚠️ **TLS_MODE=kubernetes**: Requires `NAMESPACE` + `TLS_SECRET_NAME`
- ⚠️ **Kubernetes Secret Access**: Requires proper RBAC permissions for secret read/write

**Common TLS Scenarios:**
| Scenario | TLS_ENABLED | TLS_MODE | TLS_AUTO_GENERATE | TLS_SECRET_NAME | Notes |
|----------|-------------|----------|-------------------|-----------------|-------|
| **Production K8s with auto TLS** | `true` | `kubernetes` | `true` | `xdatabase-proxy-tls` | Recommended for production in K8s |
| **Production K8s with existing cert** | `true` | `kubernetes` | `false` | `my-existing-tls` | Use pre-created TLS secret |
| **Development (no TLS)** | `false` | - | - | - | Fast local testing |
| **Development (with TLS)** | `true` | `file` | `true` | - | Auto-creates local cert files |
| **VM/Container with file certs** | `true` | `file` | `false` | - | Requires `TLS_CERT_FILE` + `TLS_KEY_FILE` |

#### Legacy Support (Backward Compatibility)

| Legacy Variable              | Maps To                                      |
| ---------------------------- | -------------------------------------------- |
| POSTGRESQL_PROXY_ENABLED     | Sets DATABASE_TYPE=postgresql                |
| POSTGRESQL_PROXY_START_PORT  | PROXY_START_PORT                             |
| TLS_ENABLE_SELF_SIGNED       | TLS_AUTO_GENERATE                            |
| POD_NAMESPACE                | NAMESPACE                                    |

### Kubernetes Service Discovery

Labels act as a **composite index** for service discovery. Proxy uses `(xdatabase-proxy-deployment-id, xdatabase-proxy-database-type, xdatabase-proxy-pooled)` as the lookup key.

**Label Matching Strategy:**
- Proxy searches for services matching the composite index
- If multiple services match the same criteria, **the first one is used** (like `findFirst()` in databases)
- Extra labels are ignored (safe to add additional labels)
- Missing optional labels are handled gracefully

| Label                             | Type    | Description                                        | Example Value   | Index |
| --------------------------------- | ------- | -------------------------------------------------- | --------------- | ----- |
| **xdatabase-proxy-deployment-id** | String  | Database deployment ID (routing key)               | db-deployment-1 | ✅ YES |
| **xdatabase-proxy-database-type** | String  | Database type (filter)                             | postgresql      | ✅ YES |
| **xdatabase-proxy-pooled**        | Boolean | Pooled connections (true/false)                    | true            | ✅ YES |
| xdatabase-proxy-destination-port  | Integer | Target port for the database connection            | 5432            | —     |
| xdatabase-proxy-enabled           | Boolean | (Deprecated) Whether service is managed by proxy   | true            | —     |

**Label Indexing Example:**

When proxy receives connection: `postgres://user.db-prod.pool@proxy:5432/db`
- Extracts: `deployment_id=db-prod`, `pooled=true`
- Searches: services with `deployment_id=db-prod` AND `pooled=true`
- Returns: **first matching service** (even if multiple exist)

```
Cluster Services:
1. Service: db-prod-1     (deployment_id=db-prod, pooled=true)   → ✅ MATCHED & USED
2. Service: db-prod-2     (deployment_id=db-prod, pooled=true)   → ⏭️ SKIPPED (duplicate)
3. Service: db-prod-pool  (deployment_id=db-prod, pooled=false)  → ⏭️ SKIPPED (diff pooled)
4. Service: db-staging    (deployment_id=db-staging, pooled=true)→ ⏭️ SKIPPED (diff id)
```

**Connection String Routing:**
- `postgres://user.db-prod@proxy:5432/db` → uses `deployment_id=db-prod, pooled=false`
- `postgres://user.db-prod.pool@proxy:5432/db` → uses `deployment_id=db-prod, pooled=true`

## PoC/PoW 
![XDatabase Proxy in Action](static/images/works-perfect.png)

## Usage Examples

### 1. Kubernetes Deployment (In-Cluster)

```bash
# Apply production configuration
kubectl apply -f kubernetes/examples/production/deploy.yaml
```

The proxy auto-detects Kubernetes runtime and uses in-cluster config.

### 2. Container with Remote Kubernetes Discovery

```bash
docker run -d \
  -e DATABASE_TYPE=postgresql \
  -e DISCOVERY_MODE=kubernetes \
  -e KUBECONFIG=/kubeconfig/config \
  -e KUBE_CONTEXT=production-cluster \
  -e TLS_AUTO_GENERATE=true \
  -v /path/to/kubeconfig:/kubeconfig \
  -p 5432:5432 \
  -p 8080:8080 \
  ghcr.io/hasirciogluhq/xdatabase-proxy:latest
```

### 3. VM with Static Backends

```bash
export DATABASE_TYPE=postgresql
export RUNTIME=vm
export DISCOVERY_MODE=static
export STATIC_BACKENDS='db1=10.0.1.5:5432,db1.pool=10.0.1.5:6432,db2=10.0.1.6:5432'
export TLS_AUTO_GENERATE=true
export TLS_AUTO_RENEW=true

./xdatabase-proxy
```

### 4. Local Development

```bash
export DATABASE_TYPE=postgresql
export DEBUG=true
export RUNTIME=vm
export DISCOVERY_MODE=kubernetes
export KUBECONFIG=~/.kube/config
export KUBE_CONTEXT=minikube
export TLS_AUTO_GENERATE=true

./xdatabase-proxy
```

### 5. Production Kubernetes with External TLS

```bash
export DATABASE_TYPE=postgresql
export RUNTIME=kubernetes
export NAMESPACE=production
export TLS_MODE=kubernetes
export TLS_SECRET_NAME=xdatabase-proxy-tls
export TLS_AUTO_GENERATE=true
export TLS_AUTO_RENEW=true
export TLS_RENEWAL_THRESHOLD_DAYS=30
```

## Connection String Format

```
postgresql://username.deployment_id[.pool]@proxy-host:port/dbname
```

Examples:

```
# Direct PostgreSQL Connection
postgresql://myuser.db-deployment-1@localhost:5432/mydb

# Connection through Pooler
postgresql://myuser.db-deployment-1.pool@localhost:5432/mydb

# Multi-node Cluster
postgresql://myuser.db-deployment-1.pool@localhost:5432/mydb
```

## Architecture

```
┌───────────────────────────────────────────────────────────────┐
│                       xdatabase-proxy                         │
│                                                               │
│  ┌──────────────────┐    ┌──────────────────────┐             │
│  │ Config & Runtime  │    │ Orchestrator (app.go)│             │
│  │  env -> types    │ →  │ wires factories      │             │
│  └──────────────────┘    └──────────────────────┘             │
│            |                          |                       │
│            v                          v                       │
│  ┌──────────────────┐    ┌──────────────────────┐             │
│  │ ResolverFactory  │    │ TLSFactory           │             │
│  │ (k8s | static)   │    │ (k8s | file | memory) │             │
│  └──────────────────┘    └──────────────────────┘             │
│            \                          /                       │
│             v                        v                        │
│              ┌────────────────────────────────┐               │
│              │ ProxyFactory (PostgreSQL)      │               │
│              │ builds ConnectionHandler       │               │
│              └────────────────────────────────┘               │
│                               |                               │
│                 ┌────────────────────────┐                    │
│                 │ Core Server            │                    │
│                 │ (TCP accept loop)      │                    │
│                 └────────────────────────┘                    │
│                               |                               │
│                 ┌────────────────────────┐                    │
│                 │ Health Server          │                    │
│                 │ /health, /ready        │                    │
│                 └────────────────────────┘                    │
└───────────────────────────────────────────────────────────────┘
```

**Flow**
- Env → `config`: validates runtime, discovery, TLS, ports.
- `app.Application`: initializes logger, resolver, TLS provider (optional), proxy handler, listener.
- Factories: runtime-aware resolver (k8s/static), pluggable TLS (k8s/file/memory), protocol proxy.
- `core.Server`: TCP accept loop, delegates to connection handler.
- `api.HealthServer`: `/health` liveness, `/ready` readiness.

## Health Check Endpoints

- `GET /health` - Basic health check
- `GET /ready` - Readiness check (returns 200 when proxy is ready)

```bash
curl http://localhost:8080/health
curl http://localhost:8080/ready
```

## Security

- **TLS/SSL Encryption**: All connections encrypted
- **Certificate Auto-Renewal**: Prevents expired certificates
- **Deployment Isolation**: Separate routing per deployment
- **Connection Validation**: Parameter validation and sanitization
- **Multi-Instance Safe**: Race condition handling

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Create a Pull Request

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Contact

GitHub Issues: https://github.com/hasirciogluhq/xdatabase-proxy/issues

---

**Note:** This is production-grade software designed for enterprise use cases. For questions, feature requests, or bug reports, please use GitHub Issues.
