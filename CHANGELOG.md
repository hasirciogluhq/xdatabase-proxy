# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

### Changed

### Fixed

### Removed

### Security

## [2.0.0] - 2026-01-12

### Added
- **Runtime Environment Detection**: Auto-detect execution environment (Kubernetes/Container/VM)
- **Flexible Discovery Modes**: 
  - Kubernetes discovery from any runtime (in-cluster or remote with kubeconfig)
  - Static backend configuration for non-Kubernetes deployments
- **Enterprise TLS Management**:
  - `TLS_AUTO_GENERATE`: Automatic self-signed certificate generation
  - `TLS_AUTO_RENEW`: Automatic certificate renewal for expired/invalid certs
  - `TLS_RENEWAL_THRESHOLD_DAYS`: Configurable renewal threshold
  - Certificate expiration validation
  - **Optional TLS**: `TLS_ENABLED` flag to make TLS completely optional
- **Professional Configuration System**:
  - Runtime-aware configuration (kubernetes/container/vm)
  - Type-safe configuration with validation
  - Smart auto-detection for all modes (runtime, discovery, TLS)
- **Enhanced Environment Variables**:
  - `DATABASE_TYPE`: Database type (postgresql)
  - `RUNTIME`: Explicit runtime environment setting
  - `DISCOVERY_MODE`: Explicit discovery mode
  - `TLS_MODE`: Explicit TLS provider mode
  - `TLS_ENABLED`: Enable/disable TLS
  - `NAMESPACE`: Professional naming (replaces POD_NAMESPACE)
- **Multi-Runtime Kubernetes Access**:
  - Access Kubernetes from VM/Container using KUBECONFIG
  - Support for multiple Kubernetes contexts
  - Remote cluster discovery from non-Kubernetes environments
- **Configuration Validation**: Comprehensive validation with helpful error messages
- **Backward Compatibility**: Full support for legacy environment variables
- New proxy manager component for advanced proxy infrastructure management
- Comprehensive structured logger with improved console output formatting
- Self-signed certificate generation capability for TLS
- Database type support in resolver configurations (Kubernetes and Memory resolvers)
- Test client script (`scripts/test-client.sh`) for connection testing
- Certificate utility functions for certificate management (`cmd/proxy/internal/utils/cert.go`)
- Memory-based TLS provider implementation
- API server component for proxy management
- Core server architecture with improved request handling
- Protocol handler for PostgreSQL connections
- Filesystem-based TLS storage provider

### Changed
- **Removed `PROXY_ENABLED`**: Proxy always runs when started (fail-fast on misconfiguration)
- **Professional Configuration Architecture**:
  - Type-safe enums for modes (RuntimeEnvironment, DiscoveryMode, TLSMode)
  - Validation-first approach with clear error messages
  - Smart defaults for all settings
- **Improved Factory Pattern**:
  - Runtime-aware resolver factory
  - TLS factory with certificate lifecycle management
  - Better error handling and logging
- **Enhanced README**:
  - Complete rewrite with enterprise focus
  - Clear runtime scenarios and examples
  - Professional documentation structure
  - Architecture diagram
- **Better Logging**:
  - Runtime information in startup logs
  - TLS mode and certificate status
  - Discovery mode details
- Merged main branch into development branch for latest stable features
- Updated Docker CMD path to reflect new folder structure (`cmd/proxy`)
- Refactored connection handler to improve error handling and lifecycle management for production environments
- Moved PostgreSQL handler from protocol package to proxy package
- Reorganized project structure: moved from `apps/proxy` to `cmd/proxy` and `pkg/*` to `cmd/proxy/internal/*`
- Updated TLS provider implementation with enhanced self-signed certificate support
- Improved discovery system for both Kubernetes and memory-based resolvers
- Enhanced PostgreSQL protocol parser implementation
- Restructured folder hierarchy for better organization
- Updated GitHub usernames and repository references across all configuration files
- Modified platform support in deployment workflow (amd64, arm64, 386)
- **TLS Now Optional**: Proxy can run without TLS if `TLS_ENABLED=false`
- Updated all Kubernetes deployment YAMLs to use new environment variable scheme
- Updated test and development scripts to use new configuration format

### Fixed
- **PostgreSQL Database Parameter Handling**: Complete fix for database defaulting logic
  - Defaults to "postgres" when client omits database name
  - Detects when client auto-fills database with username (common psql behavior)
  - Prevents "database does not exist" errors when username contains deployment_id suffix
  - Example: `database=postgres.team-1992252154561` now correctly becomes `database=postgres`
- **PostgreSQL Resolver**: Uses correct database type (postgresql) instead of mysql for routing
- **StartupMessage Rebuild**: Simplified logic - now rebuilds on every handshake for consistency
  - Every TCP connection gets a fresh StartupMessage
  - Eliminates unnecessary optimization that caused edge cases
  - Ensures username parsing and database defaulting always work correctly
- **Enhanced Logging**: Added comprehensive logging for connection parameters and username parsing
- Multi-instance TLS certificate creation race conditions
- Kubernetes discovery from non-Kubernetes runtimes
- Certificate lifecycle management issues
- Configuration validation edge cases
- Connection lifecycle issues in production environments
- Error response handling in connection handler
- Binary file cleanup (removed `proxy` binary from tracking)
- Certificate file management in repository

### Removed
- `PROXY_ENABLED` environment variable (proxy always runs)
- `POD_NAMESPACE` in favor of generic `NAMESPACE`
- `POSTGRESQL_PROXY_ENABLED` in favor of `DATABASE_TYPE`
- `TLS_ENABLE_SELF_SIGNED` in favor of `TLS_AUTO_GENERATE`
- Unnecessary configuration complexity
- Deleted old HTTP health check implementation (`cmd/proxy/internal/http/health.go`)
- Removed legacy Kubernetes client implementation
- Cleaned up old proxy server implementations and tests
- Removed temporary binary and certificate files from repository root

### Security
- Enhanced certificate validation
- Automatic certificate renewal prevents expired certs
- Multi-instance safe certificate generation
- Better TLS configuration validation
- Enhanced TLS configuration with improved certificate management
- Added self-signed certificate generation for development environments
- Improved certificate storage security with filesystem provider

### Architecture
- **Configuration-Driven Design**: All behavior controlled by environment variables
- **Runtime Awareness**: Detects and adapts to execution environment
- **Fail-Fast Philosophy**: Clear errors on misconfiguration
- **Production-Grade**: Enterprise-ready with proper validation and error handling
- **Optional TLS**: Full support for non-TLS deployments

## [1.0.8] - 2025-07-07

### Fixed

- Fixed the buildx error

## [1.0.7] - 2025-07-07

### Added

- Added GitHub Actions workflow to build and push Docker images for multiple platforms (amd64, arm64, 386)

## [1.0.6] - 2025-07-01

### Added

- Added KUBE_CONTEXT environment variable to support multiple Kubernetes contexts (only used in development/test mode, ignored in prod) (dummy variable update for testing)

### Changed

- Updated README.md with new details

## [1.0.5] - 2025-06-30

### Added

### Changed

- Improved TLS handshake timeout handling
- Enhanced TLS configuration with stronger security settings
- Added session ticket support for improved session resumption
- Updated TLS version support to include TLS 1.2 and 1.3
- Improved error handling for TLS handshake failures
- Added more detailed logging for TLS handshake process

### Removed

### Fixed

### Security

## [1.0.4] - 2024-03-26

### Added

### Changed

- Updated logging logic to remove unused parts
- Improved PostgreSQL proxy configuration with auto TLS
- Updated scripts and project settings to enforce SSL mode
- Applied patches for replicas configuration
- Adjusted deployment strategy to use DaemonSet instead of Deployment

### Deprecated

### Removed

- Removed unused logging logic
- Removed unnecessary entries from gitignore
- Deleted 001-rbac.yaml, daemonset.yaml, service.yaml, kustomization.yaml from base and overlays
- Removed postgresql.yaml and postgresql-service.yaml from postgresql directory
- Eliminated database-patch.yaml and its kustomization from test overlay

### Fixed

### Security

## [1.0.3] - 2025-04-24

### Added

- Enhanced tool-agnostic proxy behavior (supports any connection pooler, not just pgbouncer)
- Updated README with comprehensive documentation about label-based routing
- Dynamic namespace support through environment variables
- Port-forwarding integrated in test scripts for easier local testing
- Automatic service discovery for labeled Kubernetes services

### Changed

- Improved TLS/SSL certificate management: certificates now only stored in Kubernetes, not in local filesystem
- Directly loading certificates from memory instead of temporary files, improving security and performance
- Updated Go version to 1.23.4 in Dockerfile
- Enhanced Kubernetes integration with automatic secret management
- Optimized health check endpoints with atomic state management
- Improved resource utilization in proxy connections

### Security

- Eliminated local file system access for SSL certificates
- Certificates are now stored and retrieved exclusively from Kubernetes secrets
- Memory-only certificate handling reduces security exposure
- Improved TLS handshake error handling with better error messages
- Environment-based configuration to prevent hardcoded secrets

### Fixed

- Resolved potential memory leaks in connection handling
- Fixed certificate renewal logic when certificates expire
- Improved connection cleanup on proxy shutdown
- Better error handling for malformed PostgreSQL protocol messages

## [1.0.2] - 2025-03-16

### Added

- Postgresql deployment yaml
- Postgresql service yaml
- Psql Script

### Changed

- Deployment -> DaemonSet
- Minikube scripts
- Kubernetes Yamls
- Kubernetes Kustomize yamls

## [1.0.1] - 2025-03-16

### Added

- Kubernetes RBAC configuration
- Health check endpoints
- Startup probe
- Liveness probe
- Readiness probe

### Changed

- Minikube test environment setup
- Health check endpoints (!!!CURRENTLY NOT USED!!!)
- Minikube RBAC configuration

## [1.0.0] - 2025-03-15

### Added

- First stable release
- Kubernetes deployment support
- Automated deployments with GitHub Actions
- Separate configurations for test and production environments
- Container registry integration with GHCR

### Changed

- Optimized deployment strategy
- Fine-tuned resource limits and requests
- Enhanced build pipeline performance

### Security

- Added container security configurations
- Implemented secure registry authentication
- Added RBAC configurations
