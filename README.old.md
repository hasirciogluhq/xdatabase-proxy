# XDatabase Proxy

![XDatabase Proxy in Action](static/images/works-perfect.png)

> **XDatabase Proxy in Action:**
> The screenshot below demonstrates a successful, secure PostgreSQL connection established through the xdatabase-proxy. The proxy automatically generates and manages TLS certificates, ensuring encrypted traffic between your client and the database. This seamless integration with Kubernetes and real-time certificate handling provides both security and ease of use for your cloud-native database workloads.

XDatabase Proxy is a smart proxy solution for your database deployments running in Kubernetes environments. This proxy is designed to manage and route connections between different database deployments.

## Features

- 🔄 **Dynamic Service Discovery**: Automatic backend discovery via Kubernetes API
- 🎯 **Deployment-Based Routing**: Route connections based on deployment IDs
- 🌊 **Connection Pooling Support**: Works with any pooler (pgbouncer, odyssey, etc.)
- 🚀 **Kubernetes Native**: Seamless integration with Kubernetes environments
- 📊 **Smart Load Balancing**: Intelligent routing between backends
- 🔍 **Real-Time Monitoring**: Live service discovery and health checks
- 🔀 **Multi-Node Cluster Support**: Works with any cluster manager (pgpool-II, patroni, etc.)
- 🔒 **Advanced TLS/SSL Support**: 
  - Auto-generated self-signed certificates
  - File-based certificate management
  - Kubernetes Secret integration
  - Automatic certificate creation if not exists
- 🏷️ **Label-Based Configuration**: No hard dependencies on specific implementations
- 🔌 **Static Backend Support**: Run without Kubernetes for local development
- 🩺 **Health Check Endpoint**: Built-in health and readiness checks on port 8080
- 🪵 **Structured Logging**: JSON-formatted logs with debug mode support

## Supported Databases

Currently, the following databases are supported:

- PostgreSQL (Full Support)
- MySQL (Planned)
- MongoDB (Planned)

## Requirements

- Go 1.23.4 or higher
- Kubernetes cluster or local test environment
- kubectl configuration

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

| Variable             | Description                                                                 | Required | Default | Example Value   |
| -------------------- | --------------------------------------------------------------------------- | -------- | ------- | --------------- |
| PROXY_ENABLED        | Enable proxy. Must be set to 'true' to activate the proxy.                 | Yes      | false   | true            |
| PROXY_START_PORT     | Starting port for proxy.                                                    | No       | 5432    | 5432            |
| DATABASE_TYPE        | Database type to proxy.                                                     | No       | postgresql | postgresql   |
| HEALTH_SERVER_PORT   | Health check server port.                                                   | No       | 8080    | 8080            |
| DEBUG                | Enable debug logging. Set to 'true' for detailed logs.                      | No       | false   | true            |

**Legacy Support:**
- `POSTGRESQL_PROXY_ENABLED`: Alias for `PROXY_ENABLED=true` with `DATABASE_TYPE=postgresql`
- `POSTGRESQL_PROXY_START_PORT`: Alias for `PROXY_START_PORT`

#### Kubernetes Configuration

| Variable     | Description                                                                                                        | Required | Default | Example Value  |
| ------------ | ------------------------------------------------------------------------------------------------------------------ | -------- | ------- | -------------- |
| KUBECONFIG   | Path to kubeconfig file. Used for local development and testing.                                                   | No       | ~/.kube/config | /path/to/config |
| KUBE_CONTEXT | Kubernetes context name. Only used in development/test mode for multi-cluster setups.                              | No       | -       | local-test     |
| POD_NAMESPACE| Namespace where the proxy pod is running. Automatically set by Kubernetes downward API in production.              | No       | -       | xdatabase-proxy|
| NAMESPACE    | Generic namespace fallback. Used if POD_NAMESPACE is not set.                                                       | No       | default | xdatabase-proxy|

#### Backend Configuration

| Variable         | Description                                                                                      | Required | Default | Example Value                    |
| ---------------- | ------------------------------------------------------------------------------------------------ | -------- | ------- | -------------------------------- |
| STATIC_BACKENDS  | Static backend configuration for non-Kubernetes deployments. Format: JSON array of backends. Automatically sets discovery mode to 'static'.    | No       | -       | [{"name":"db1","host":"localhost","port":5432}] |

> **Discovery Modes:** The proxy automatically determines the discovery mode:
> - `static`: When `STATIC_BACKENDS` is set
> - `kubernetes`: Default mode when `STATIC_BACKENDS` is not set

#### TLS/SSL Configuration

| Variable                 | Description                                                                                         | Required | Default | Example Value          |
| ------------------------ | --------------------------------------------------------------------------------------------------- | -------- | ------- | ---------------------- |
| TLS_CERT_FILE            | Path to TLS certificate file. Takes priority over other TLS configurations.                         | No       | -       | /certs/tls.crt         |
| TLS_KEY_FILE             | Path to TLS private key file. Required when TLS_CERT_FILE is set.                                   | No       | -       | /certs/tls.key         |
| TLS_SECRET_NAME          | Kubernetes secret name containing TLS certificate. Used when TLS_CERT_FILE is not set.              | No       | -       | xdatabase-proxy-tls    |
| TLS_ENABLE_SELF_SIGNED   | Generate and store a self-signed certificate if no certificate exists. Useful for development.      | No       | false   | true                   |

> **TLS Provider Selection:** The proxy automatically determines the TLS provider:
> 1. `file`: When `TLS_CERT_FILE` is set
> 2. `kubernetes`: When `TLS_SECRET_NAME` is set
> 3. `memory`: Default fallback (auto-generates self-signed certificate if `TLS_ENABLE_SELF_SIGNED=true`)
> 
> **Auto-create in Kubernetes Secret:** If secret doesn't exist, it will be automatically created when using `TLS_SECRET_NAME`

### Kubernetes Labels

The following labels are required for the proxy to identify database services:

| Label                            | Description                                        | Example Value   |
| -------------------------------- | -------------------------------------------------- | --------------- |
| xdatabase-proxy-enabled          | Whether the service should be managed by the proxy | true            |
| xdatabase-proxy-deployment-id    | Database deployment ID                             | db-deployment-1 |
| xdatabase-proxy-database-type    | Database type                                      | postgresql      |
| xdatabase-proxy-pooled           | Whether this is a connection pooling service       | true/false      |
| xdatabase-proxy-destination-port | Target port for the database connection            | 5432            |

> **Important**: This proxy is designed to be tool-agnostic. You don't need to use any specific pooling or cluster management solution. Simply add the appropriate labels to any service, and the proxy will route connections accordingly based on those labels.

## Connection Scenarios

The proxy supports three connection scenarios:

1. **Direct Connection**

   - Client → PostgreSQL
   - Simple, direct connection to a single PostgreSQL instance
   - Use when connection pooling is not needed

2. **Connection Pooling**

   - Client → Connection Pooler → PostgreSQL
   - Efficient connection management
   - Recommended for applications with many connections
   - Works with any connection pooler (pgbouncer, odyssey, etc.)

3. **Multi-Node Cluster**
   - Client → Connection Pooler → Cluster Manager → [Master + Follower Nodes]
   - High availability and load balancing
   - Required for multi-node PostgreSQL clusters
   - Works with any cluster manager (pgpool-II, patroni, etc.)

## Usage

### Quick Start with Docker

```bash
# Using environment variables
docker run -d \
  -e PROXY_ENABLED=true \
  -e DATABASE_TYPE=postgresql \
  -e PROXY_START_PORT=5432 \
  -e TLS_ENABLE_SELF_SIGNED=true \
  -e STATIC_BACKENDS='[{"name":"mydb","host":"postgres.example.com","port":5432}]' \
  -p 5432:5432 \
  -p 8080:8080 \
  ghcr.io/hasirciogluhq/xdatabase-proxy:latest
```

### Local Development

```bash
# Set environment variables
export PROXY_ENABLED=true
export DATABASE_TYPE=postgresql
export PROXY_START_PORT=5432
export TLS_ENABLE_SELF_SIGNED=true
export DEBUG=true
export KUBECONFIG=~/.kube/config
export KUBE_CONTEXT=minikube

# Run the proxy
./xdatabase-proxy
```

### Service Definition Examples

#### 1. Direct PostgreSQL Service

```yaml
apiVersion: v1
kind: Service
metadata:
  name: postgres-db
  labels:
    xdatabase-proxy-enabled: "true"
    xdatabase-proxy-deployment-id: "db-deployment-1"
    xdatabase-proxy-database-type: "postgresql"
    xdatabase-proxy-pooled: "false" # Direct PostgreSQL connection
    xdatabase-proxy-destination-port: "5432" # Target PostgreSQL port
spec:
  ports:
    - port: 5432
      name: postgresql
```

#### 2. Connection Pooling Service (Example with PgBouncer)

```yaml
apiVersion: v1
kind: Service
metadata:
  name: pgbouncer-pool
  labels:
    xdatabase-proxy-enabled: "true"
    xdatabase-proxy-deployment-id: "db-deployment-1"
    xdatabase-proxy-database-type: "postgresql"
    xdatabase-proxy-pooled: "true" # This indicates it's a connection pooling service
    xdatabase-proxy-destination-port: "6432" # Target pooler port
spec:
  ports:
    - port: 6432
      name: postgresql
```

#### 3. Multi-Node Cluster Setup (Example with Pgpool-II)

```yaml
# Connection Pooler Service (Required for multi-node)
apiVersion: v1
kind: Service
metadata:
  name: connection-pool
  labels:
    xdatabase-proxy-enabled: "true"
    xdatabase-proxy-deployment-id: "db-deployment-1"
    xdatabase-proxy-database-type: "postgresql"
    xdatabase-proxy-pooled: "true" # Required for multi-node setup
    xdatabase-proxy-destination-port: "6432" # Target cluster manager port (Same as the connection pooler port)
spec:
  ports:
    - port: 6432
      name: postgresql
```

> **Note:** For multi-node clusters (e.g., Pgpool-II, Patroni, etc.), you only need to define the connection pooler service as shown above. There is no need for a separate cluster manager service definition; the proxy will automatically handle routing based on labels.

### Connection String Format

```
postgresql://username.deployment_id[.pool]@proxy-host:port/dbname
```

Examples:

```
# 1. Direct PostgreSQL Connection
postgresql://myuser.db-deployment-1@localhost:3001/mydb

# 2. Connection through Connection Pooler
postgresql://myuser.db-deployment-1.pool@localhost:3001/mydb

# 3. Multi-node Cluster Connection (automatically uses the right services)
postgresql://myuser.db-deployment-1.pool@localhost:3001/mydb
```

## Features and Capabilities

### Backend Discovery Modes

1. **Kubernetes Mode** (Default)
   - Automatic service discovery via Kubernetes API
   - Real-time updates when services change
   - Label-based routing and filtering
   - Namespace-aware operations

2. **Static Mode** (Development/Testing)
   - Configure backends via `STATIC_BACKENDS` environment variable
   - No Kubernetes dependency
   - Perfect for local development
   - Example: `STATIC_BACKENDS='[{"name":"db1","host":"localhost","port":5432}]'`

### TLS/SSL Modes

1. **File-Based TLS**
   - Set `TLS_CERT_FILE` and `TLS_KEY_FILE`
   - Highest priority
   - Use for custom certificates

2. **Kubernetes Secret**
   - Set `TLS_SECRET_NAME`
   - Automatically creates secret if it doesn't exist
   - Self-signed certificate generated on first use
   - Perfect for production Kubernetes deployments

3. **In-Memory TLS**
   - Default fallback mode
   - Use `TLS_ENABLE_SELF_SIGNED=true` to auto-generate
   - No persistence (regenerated on restart)

### Health Check Endpoints

The proxy exposes health check endpoints on port `8080`:

- `GET /health` - Basic health check
- `GET /ready` - Readiness check (returns 200 when proxy is ready to accept connections)

```bash
# Check health
curl http://localhost:8080/health

# Check readiness
curl http://localhost:8080/ready
```

## Features and Limitations

- ✅ Separate database services for each deployment
- ✅ Automatic load balancing and routing based on labels
- ✅ Works with any connection pooling solution
- ✅ Works with any cluster management solution
- ✅ Label-based service discovery (no hardcoded dependencies)
- ✅ Real-time service discovery via Kubernetes API
- ✅ Full TLS/SSL support with multiple configuration methods
- ✅ Auto-generated certificates with Kubernetes Secret integration
- ✅ Static backend support for non-Kubernetes environments
- ✅ Built-in health and readiness checks
- ⚠️ Currently only PostgreSQL is fully supported
- 📋 MySQL support planned
- 📋 MongoDB support planned

## Security

- Isolation between deployments
- Connection parameter validation
- Secure TLS/SSL connection support

## Contributing

1. Fork it
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Create a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Contact

If you have any questions or suggestions, please reach out through GitHub Issues.

## Production Deployment

### Kubernetes Deployment

To deploy xdatabase-proxy in your production Kubernetes cluster:

```bash
# Apply production configuration
kubectl apply -f kubernetes/examples/production/deploy.yaml

# Or use the raw GitHub URL directly
kubectl apply -f https://raw.githubusercontent.com/hasirciogluhq/xdatabase-proxy/main/kubernetes/examples/production/deploy.yaml
```

### Docker Deployment

```bash
# Pull the latest image
docker pull ghcr.io/hasirciogluhq/xdatabase-proxy:latest

# Run with custom configuration
docker run -d \
  --name xdatabase-proxy \
  -e POSTGRESQL_PROXY_ENABLED=true \
  -e POSTGRESQL_PROXY_START_PORT=5432 \
  -e TLS_ENABLE_SELF_SIGNED=true \
  -e TLS_SECRET_NAME=xdatabase-proxy-tls \
  -e POD_NAMESPACE=default \
  -p 5432:5432 \
  -p 8080:8080 \
  ghcr.io/hasirciogluhq/xdatabase-proxy:latest
```

### Environment-Specific Configurations

#### Development

```bash
export DEBUG=true
export PROXY_ENABLED=true
export DATABASE_TYPE=postgresql
export TLS_ENABLE_SELF_SIGNED=true
export KUBECONFIG=~/.kube/config
export KUBE_CONTEXT=minikube
```

#### Staging

```bash
export PROXY_ENABLED=true
export DATABASE_TYPE=postgresql
export TLS_SECRET_NAME=xdatabase-proxy-tls-staging
export POD_NAMESPACE=staging
export PROXY_START_PORT=5432
```

#### Production

```bash
export PROXY_ENABLED=true
export DATABASE_TYPE=postgresql
export TLS_SECRET_NAME=xdatabase-proxy-tls
export POD_NAMESPACE=production
export PROXY_START_PORT=5432
# DEBUG should not be enabled in production
```

## How it works ?
<img width="1303" height="3840" alt="Untitled diagram _ Mermaid Chart-2025-07-27-142550" src="https://github.com/user-attachments/assets/e27af19d-4784-4c9b-8e5d-4d036d07a6d2" />


---

_(Note: A Turkish version of this README is planned and will be added soon.)_

---
