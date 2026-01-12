package main

import (
	"context"
	"fmt"
	"net"
	"os"

	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/api"
	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/config"
	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/core"
	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/factory"
	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/logger"
)

func main() {
	ctx := context.Background()

	// Load configuration from environment
	cfg, err := config.LoadFromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logger.Init()
	logger.Info("Starting xdatabase-proxy...",
		"database", cfg.DatabaseType,
		"runtime", cfg.Runtime,
		"discovery", cfg.DiscoveryMode,
		"tls_mode", cfg.TLSMode)

	// Start health server
	healthServer := api.NewHealthServer(":" + cfg.HealthServerPort)
	healthServer.Start()
	logger.Info("Health server started", "port", cfg.HealthServerPort)

	// Create backend resolver
	resolverFactory := factory.NewResolverFactory(cfg)
	resolver, clientset, err := resolverFactory.Create(ctx)
	if err != nil {
		logger.Fatal("Failed to create backend resolver", "error", err)
	}

	// Create TLS provider (optional)
	var tlsProvider core.TLSProvider
	if cfg.TLSEnabled {
		tlsFactory := factory.NewTLSFactory(cfg)
		var err error
		tlsProvider, err = tlsFactory.Create(ctx, clientset)
		if err != nil {
			logger.Fatal("Failed to create TLS provider", "error", err)
		}

		// Ensure certificate exists (load or generate)
		if err := tlsFactory.EnsureCertificate(ctx, tlsProvider); err != nil {
			logger.Fatal("Failed to ensure certificate", "error", err)
		}
		logger.Info("TLS enabled and configured")
	} else {
		logger.Warn("TLS is disabled - connections will not be encrypted")
	}

	// Create protocol-specific proxy handler
	proxyFactory := factory.NewProxyFactory(cfg)
	connectionHandler, err := proxyFactory.Create(ctx, tlsProvider, resolver)
	if err != nil {
		logger.Fatal("Failed to create proxy handler", "error", err)
	}

	// Start TCP listener
	listener, err := net.Listen("tcp", ":"+cfg.ProxyStartPort)
	if err != nil {
		logger.Fatal("Failed to start listener", "port", cfg.ProxyStartPort, "error", err)
	}
	logger.Info("Proxy listening", "port", cfg.ProxyStartPort, "database", cfg.DatabaseType)

	// Create and start server
	server := &core.Server{
		Listener:          listener,
		ConnectionHandler: connectionHandler,
	}

	// Mark as ready
	healthServer.SetReady(true)
	logger.Info("Proxy is ready to accept connections")

	// Start serving (blocking)
	if err := server.Serve(); err != nil {
		logger.Fatal("Server error", "error", err)
	}
}
