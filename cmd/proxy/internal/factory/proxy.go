package factory

import (
	"context"
	"crypto/tls"
	"fmt"

	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/config"
	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/core"
	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/logger"
	postgresql_proxy "github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/proxy/postgresql"
)

// ProxyFactory creates protocol-specific proxy handlers
type ProxyFactory struct {
	cfg *config.Config
}

// NewProxyFactory creates a new proxy factory
func NewProxyFactory(cfg *config.Config) *ProxyFactory {
	return &ProxyFactory{cfg: cfg}
}

// Create creates a connection handler based on database type
func (f *ProxyFactory) Create(ctx context.Context, tlsProvider core.TLSProvider, resolver core.BackendResolver) (core.ConnectionHandler, error) {
	switch f.cfg.DatabaseType {
	case "postgresql":
		return f.createPostgreSQLProxy(ctx, tlsProvider, resolver)
	case "mysql":
		return nil, fmt.Errorf("MySQL proxy not yet implemented")
	case "mongodb":
		return nil, fmt.Errorf("MongoDB proxy not yet implemented")
	default:
		return nil, fmt.Errorf("unknown database type: %s", f.cfg.DatabaseType)
	}
}

func (f *ProxyFactory) createPostgreSQLProxy(ctx context.Context, tlsProvider core.TLSProvider, resolver core.BackendResolver) (core.ConnectionHandler, error) {
	logger.Info("Creating PostgreSQL Proxy Handler", "tls_enabled", f.cfg.TLSEnabled)

	var tlsConfig *tls.Config

	// TLS is optional
	if f.cfg.TLSEnabled && tlsProvider != nil {
		cert, err := tlsProvider.GetCertificate(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to load certificate for PostgreSQL proxy: %w", err)
		}
		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{*cert},
		}
	} else {
		logger.Warn("TLS is disabled. Connections will not be encrypted!")
	}

	return &postgresql_proxy.PostgresProxy{
		TLSConfig: tlsConfig,
		Resolver:  resolver,
	}, nil
}
