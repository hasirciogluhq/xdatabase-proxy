package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/config"
	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/discovery"
	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/health"
	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/postgres"
	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/tlsstore"
	"github.com/hasirciogluhq/xdatabase-proxy/pkg/proxy"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var version = "dev"

func main() {
	healthcheck := flag.Bool("healthcheck", false, "probe the local /health endpoint and exit (for container HEALTHCHECK)")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "configuration error:", err)
		os.Exit(1)
	}
	if *healthcheck {
		os.Exit(probe(cfg.HealthPort))
	}

	level := slog.LevelInfo
	if cfg.Debug {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	if err := run(cfg, log); err != nil {
		log.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run(cfg *config.Config, log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	log.Info("starting xdatabase-proxy", "version", version, "discovery", cfg.Discovery, "tls", cfg.TLSEnabled)

	hs := health.New(":" + cfg.HealthPort)
	go func() {
		if err := hs.ListenAndServe(); err != http.ErrServerClosed {
			log.Error("health server", "error", err)
		}
	}()

	var kc kubernetes.Interface
	if cfg.NeedsKube() {
		var err error
		if kc, err = kubeClient(cfg); err != nil {
			return fmt.Errorf("kubernetes client: %w (outside a pod set KUBECONFIG)", err)
		}
	}

	var resolver postgres.Resolver
	if cfg.Discovery == "static" {
		r, err := discovery.NewStatic(cfg.StaticBackends)
		if err != nil {
			return err
		}
		resolver = r
	} else {
		r, err := discovery.NewKubernetes(ctx, kc, cfg.WatchNamespace, "postgresql", cfg.ClusterDomain)
		if err != nil {
			return err
		}
		resolver = r
	}

	h := &postgres.Handler{Resolver: resolver, DialTimeout: cfg.DialTimeout, Log: log}
	if cfg.TLSEnabled {
		getTLS, err := tlsConfig(ctx, cfg, kc, log)
		if err != nil {
			return err
		}
		h.TLS = getTLS
	} else {
		log.Warn("TLS is disabled, client connections will not be encrypted")
	}

	ln, err := net.Listen("tcp", ":"+cfg.ProxyPort)
	if err != nil {
		return err
	}
	srv := &proxy.Server{Handler: h, MaxConns: cfg.MaxConns, ShutdownTimeout: 25 * time.Second, Log: log}
	hs.SetReady(true)
	log.Info("listening", "port", cfg.ProxyPort)

	err = srv.Serve(ctx, ln)
	hs.SetReady(false)
	sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	hs.Shutdown(sctx)
	log.Info("stopped")
	return err
}

// tlsConfig loads the certificate and refreshes it in the background so
// renewed or rotated certificates are picked up without a restart.
func tlsConfig(ctx context.Context, cfg *config.Config, kc kubernetes.Interface, log *slog.Logger) (func() *tls.Config, error) {
	var store tlsstore.Store
	switch cfg.TLSMode {
	case "file":
		store = tlsstore.File{CertFile: cfg.TLSCertFile, KeyFile: cfg.TLSKeyFile}
	case "kubernetes":
		store = tlsstore.Secret{Client: kc, Namespace: cfg.Namespace, Name: cfg.TLSSecretName}
	default:
		store = &tlsstore.Memory{}
	}
	load := func(ctx context.Context) (*tls.Certificate, error) {
		return tlsstore.Ensure(ctx, store, cfg.TLSAutoGenerate, cfg.TLSRenewalWindow, log)
	}
	cert, err := load(ctx)
	if err != nil {
		return nil, err
	}
	var current atomic.Pointer[tls.Certificate]
	current.Store(cert)
	go func() {
		t := time.NewTicker(6 * time.Hour)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if c, err := load(ctx); err != nil {
					log.Warn("certificate refresh failed", "error", err)
				} else {
					current.Store(c)
				}
			}
		}
	}()
	return func() *tls.Config {
		return &tls.Config{
			MinVersion:     tls.VersionTLS12,
			GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) { return current.Load(), nil },
		}
	}, nil
}

func probe(port string) int {
	c := http.Client{Timeout: 2 * time.Second}
	resp, err := c.Get("http://127.0.0.1:" + port + "/health")
	if err != nil || resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

// kubeClient uses the pod's service account, or KUBECONFIG when set (VM/container runtimes).
func kubeClient(cfg *config.Config) (kubernetes.Interface, error) {
	var rc *rest.Config
	var err error
	if cfg.KubeConfig == "" {
		rc, err = rest.InClusterConfig()
	} else {
		rc, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{ExplicitPath: cfg.KubeConfig},
			&clientcmd.ConfigOverrides{CurrentContext: cfg.KubeContext}).ClientConfig()
	}
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(rc)
}
