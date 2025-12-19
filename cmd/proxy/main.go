package main

import (
	"context"
	"crypto/tls"
	"net"
	"os"

	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/api"
	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/core"
	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/discovery/kubernetes"
	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/discovery/memory"
	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/logger"
	postgresql_proxy "github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/proxy/postgresql"
	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/storage/filesystem"
	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/utils"

	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	logger.Init()
	logger.Info("Starting xdatabase-proxy...")

	// Check if proxy is enabled
	if os.Getenv("POSTGRESQL_PROXY_ENABLED") != "true" {
		logger.Warn("PostgreSQL proxy is not enabled (POSTGRESQL_PROXY_ENABLED != true)")
		// We might still want to run health checks or just exit?
		// For now, let's assume we just block or exit.
		// But usually a pod runs the proxy if it's deployed.
		// Let's just log and continue, or maybe return.
		// The original code returned.
		return
	}

	// 1. Health Server
	healthServer := api.NewHealthServer(":8080")
	healthServer.Start()

	// 2. Infrastructure Layer (Resolver)
	var resolver core.BackendResolver
	var clientset *k8s.Clientset

	if staticBackends := os.Getenv("STATIC_BACKENDS"); staticBackends != "" {
		logger.Info("Using Memory Resolver (STATIC_BACKENDS set)")
		memResolver, err := memory.NewResolver(staticBackends)
		if err != nil {
			logger.Fatal("Failed to create memory resolver", "error", err)
		}
		resolver = memResolver
	} else {
		// Kubernetes Resolver
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			kubeconfig = os.Getenv("HOME") + "/.kube/config"
		}

		// Use KUBE_CONTEXT if provided (dev mode)
		contextName := os.Getenv("KUBE_CONTEXT")

		configOverrides := &clientcmd.ConfigOverrides{}
		if contextName != "" {
			configOverrides.CurrentContext = contextName
		}

		config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig},
			configOverrides,
		).ClientConfig()

		if err != nil {
			// Fallback to in-cluster config
			config, err = clientcmd.BuildConfigFromFlags("", "")
			if err != nil {
				logger.Fatal("Failed to build kubeconfig", "error", err)
			}
		}

		clientset, err = k8s.NewForConfig(config)
		if err != nil {
			logger.Fatal("Failed to create k8s client", "error", err)
		}
		resolver = kubernetes.NewK8sResolver(clientset)
	}

	// 3. TLS Provider
	var tlsProvider core.TLSProvider

	// Priority 1: File-based TLS (Explicit configuration)
	if certFile := os.Getenv("TLS_CERT_FILE"); certFile != "" {
		keyFile := os.Getenv("TLS_KEY_FILE")
		if keyFile == "" {
			logger.Fatal("TLS_KEY_FILE must be set when TLS_CERT_FILE is set")
		}
		logger.Info("Using File TLS provider", "cert", certFile, "key", keyFile)
		tlsProvider = filesystem.NewFileTLSProvider(certFile, keyFile)
	} else if secretName := os.Getenv("TLS_SECRET_NAME"); secretName != "" {
		// Priority 2: Kubernetes Secret (Explicit configuration)
		if clientset == nil {
			logger.Fatal("Cannot use Kubernetes TLS provider without Kubernetes environment (STATIC_BACKENDS is set)")
		}
		namespace := os.Getenv("POD_NAMESPACE")
		if namespace == "" {
			namespace = os.Getenv("NAMESPACE") // Fallback to generic NAMESPACE env
		}
		if namespace == "" {
			namespace = "default"
		}
		logger.Info("Using Kubernetes TLS provider", "namespace", namespace, "secret", secretName)
		tlsProvider = kubernetes.NewK8sTLSProvider(clientset, namespace, secretName)
	} else {
		logger.Info("Using Memory TLS provider (Default)")
		tlsProvider = memory.NewMemoryTLSProvider()
	}

	// Check if we should generate and store a self-signed certificate
	if os.Getenv("TLS_ENABLE_SELF_SIGNED") == "true" {
		logger.Info("TLS_ENABLE_SELF_SIGNED is true. Generating and storing self-signed certificate...")
		certPEM, keyPEM, err := utils.GenerateSelfSignedCert()
		if err != nil {
			logger.Fatal("Failed to generate self-signed cert", "error", err)
		}

		if err := tlsProvider.Store(context.Background(), certPEM, keyPEM); err != nil {
			logger.Fatal("Failed to store self-signed cert", "error", err)
		}
	}

	// Load initial certificate
	cert, err := tlsProvider.GetCertificate(context.Background())
	if err != nil {
		logger.Fatal("Failed to load initial certificate", "error", err)
	}

	// 4. Protocol Layer (PostgreSQL)
	postgresProxy := &postgresql_proxy.PostgresProxy{
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{*cert},
		},
		Resolver: resolver,
	}

	// 5. Core Layer (Proxy)
	startPort := os.Getenv("POSTGRESQL_PROXY_START_PORT")
	if startPort == "" {
		startPort = "5432"
	}

	listener, err := net.Listen("tcp", ":"+startPort)
	if err != nil {
		logger.Fatal("Failed to listen", "error", err)
	}
	logger.Info("Listening on", "port", startPort)

	server := &core.Server{
		Listener:          listener,
		ConnectionHandler: postgresProxy,
	}

	// Mark as ready
	healthServer.SetReady(true)

	if err := server.Serve(); err != nil {
		logger.Fatal("Server error", "error", err)
	}
}
