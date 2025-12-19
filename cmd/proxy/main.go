package main

import (
	"context"
	"crypto/tls"
	"log"
	"net"
	"os"

	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/api"
	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/core"
	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/discovery/kubernetes"
	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/discovery/memory"
	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/protocol/postgresql"
	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/storage/filesystem"
	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/utils"

	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	log.Println("Starting xdatabase-proxy...")

	// Check if proxy is enabled
	if os.Getenv("POSTGRESQL_PROXY_ENABLED") != "true" {
		log.Println("PostgreSQL proxy is not enabled (POSTGRESQL_PROXY_ENABLED != true)")
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
		log.Println("Using Memory Resolver (STATIC_BACKENDS set)")
		memResolver, err := memory.NewResolver(staticBackends)
		if err != nil {
			log.Fatalf("Failed to create memory resolver: %v", err)
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
				log.Fatalf("Failed to build kubeconfig: %v", err)
			}
		}

		clientset, err = k8s.NewForConfig(config)
		if err != nil {
			log.Fatalf("Failed to create k8s client: %v", err)
		}
		resolver = kubernetes.NewK8sResolver(clientset)
	}

	// 3. TLS Provider
	var tlsProvider core.TLSProvider

	// Priority 1: File-based TLS (Explicit configuration)
	if certFile := os.Getenv("TLS_CERT_FILE"); certFile != "" {
		keyFile := os.Getenv("TLS_KEY_FILE")
		if keyFile == "" {
			log.Fatal("TLS_KEY_FILE must be set when TLS_CERT_FILE is set")
		}
		log.Printf("Using File TLS provider (cert: %s, key: %s)", certFile, keyFile)
		tlsProvider = filesystem.NewFileTLSProvider(certFile, keyFile)
	} else if secretName := os.Getenv("TLS_SECRET_NAME"); secretName != "" {
		// Priority 2: Kubernetes Secret (Explicit configuration)
		if clientset == nil {
			log.Fatal("Cannot use Kubernetes TLS provider without Kubernetes environment (STATIC_BACKENDS is set)")
		}
		namespace := os.Getenv("POD_NAMESPACE")
		if namespace == "" {
			namespace = os.Getenv("NAMESPACE") // Fallback to generic NAMESPACE env
		}
		if namespace == "" {
			namespace = "default"
		}
		log.Printf("Using Kubernetes TLS provider (secret: %s/%s)", namespace, secretName)
		tlsProvider = kubernetes.NewK8sTLSProvider(clientset, namespace, secretName)
	} else {
		log.Println("Using Memory TLS provider (Default)")
		tlsProvider = memory.NewMemoryTLSProvider()
	}

	// Check if we should generate and store a self-signed certificate
	if os.Getenv("TLS_ENABLE_SELF_SIGNED") == "true" {
		log.Println("TLS_ENABLE_SELF_SIGNED is true. Generating and storing self-signed certificate...")
		certPEM, keyPEM, err := utils.GenerateSelfSignedCert()
		if err != nil {
			log.Fatalf("Failed to generate self-signed cert: %v", err)
		}

		if err := tlsProvider.Store(context.Background(), certPEM, keyPEM); err != nil {
			log.Fatalf("Failed to store self-signed cert: %v", err)
		}
	}

	// Load initial certificate
	cert, err := tlsProvider.GetCertificate(context.Background())
	if err != nil {
		log.Fatalf("Failed to load initial certificate: %v", err)
	}

	// 4. Protocol Layer (PostgreSQL)
	protocolHandler := &postgresql.PostgresHandler{
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{*cert},
		},
	}

	// 5. Core Layer (Proxy)
	startPort := os.Getenv("POSTGRESQL_PROXY_START_PORT")
	if startPort == "" {
		startPort = "5432"
	}

	listener, err := net.Listen("tcp", ":"+startPort)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	log.Printf("Listening on :%s", startPort)

	server := &core.Server{
		Listener:        listener,
		Resolver:        resolver,
		ProtocolHandler: protocolHandler,
	}

	// Mark as ready
	healthServer.SetReady(true)

	if err := server.Serve(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
