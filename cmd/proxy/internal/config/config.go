// Package config loads the proxy configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Debug bool
	// DatabaseType selects the handler: postgresql (login based) or
	// mongodb, redis, clickhouse, tls (SNI based passthrough).
	DatabaseType string
	ProxyPort    string
	// HealthPort serves /health and /ready.
	HealthPort  string
	MaxConns    int
	DialTimeout time.Duration

	// Discovery is "static" or "kubernetes".
	Discovery      string
	StaticBackends string // "deployment[.pool]=host:port,..."
	// Namespace is where the proxy runs (TLS secret). WatchNamespace limits
	// service discovery; empty means all namespaces (needs a ClusterRole).
	Namespace      string
	WatchNamespace string
	ClusterDomain  string
	// Out-of-cluster access to the Kubernetes API (empty in a pod = in-cluster).
	KubeConfig  string
	KubeContext string

	TLSEnabled       bool
	TLSMode          string // file, kubernetes, memory
	TLSCertFile      string
	TLSKeyFile       string
	TLSSecretName    string
	TLSAutoGenerate  bool
	TLSRenewalWindow time.Duration
}

func Load() (*Config, error) {
	c := &Config{
		Debug:          boolEnv("DEBUG", false),
		ProxyPort:      env("PROXY_START_PORT", env("POSTGRESQL_PROXY_START_PORT", "5432")),
		HealthPort:     env("HEALTH_SERVER_PORT", "8080"),
		MaxConns:       intEnv("MAX_CONNECTIONS", 10000),
		DialTimeout:    time.Duration(intEnv("DIAL_TIMEOUT_SECONDS", 5)) * time.Second,
		StaticBackends: env("STATIC_BACKENDS", ""),
		Namespace:      env("NAMESPACE", env("POD_NAMESPACE", namespaceFromServiceAccount())),
		WatchNamespace: env("WATCH_NAMESPACE", ""),
		ClusterDomain:  env("CLUSTER_DOMAIN", "cluster.local"),
		KubeConfig:     env("KUBECONFIG", ""),
		KubeContext:    env("KUBE_CONTEXT", ""),

		TLSEnabled:       boolEnv("TLS_ENABLED", true),
		TLSCertFile:      env("TLS_CERT_FILE", ""),
		TLSKeyFile:       env("TLS_KEY_FILE", ""),
		TLSSecretName:    env("TLS_SECRET_NAME", ""),
		TLSAutoGenerate:  boolEnv("TLS_AUTO_GENERATE", true),
		TLSRenewalWindow: time.Duration(intEnv("TLS_RENEWAL_THRESHOLD_DAYS", 30)) * 24 * time.Hour,
	}

	c.DatabaseType = strings.ToLower(env("DATABASE_TYPE", "postgresql"))
	switch c.DatabaseType {
	case "postgresql":
	case "cockroachdb", "yugabytedb", "timescaledb", "citus", "greenplum", "questdb", "cratedb":
		// Same wire protocol; Services keep the label xdatabase-proxy-database-type: postgresql.
		c.DatabaseType = "postgresql"
	case "redis", "clickhouse":
		// Username routing (proxy terminates TLS if the client uses it) and SNI passthrough.
	case "mongodb", "tls":
		// No username available before authentication: SNI passthrough only, the backend terminates TLS.
		c.TLSEnabled = false
	default:
		return nil, fmt.Errorf("unsupported DATABASE_TYPE %q (postgresql, mongodb, redis, clickhouse, tls)", c.DatabaseType)
	}

	c.Discovery = strings.ToLower(env("DISCOVERY_MODE", ""))
	if c.Discovery == "" {
		c.Discovery = "kubernetes"
		if c.StaticBackends != "" {
			c.Discovery = "static"
		}
	}
	if c.Discovery != "static" && c.Discovery != "kubernetes" {
		return nil, fmt.Errorf("unknown DISCOVERY_MODE %q (static, kubernetes)", c.Discovery)
	}

	c.TLSMode = strings.ToLower(env("TLS_MODE", ""))
	if c.TLSMode == "" {
		switch {
		case c.TLSCertFile != "":
			c.TLSMode = "file"
		case c.TLSSecretName != "":
			c.TLSMode = "kubernetes"
		default:
			c.TLSMode = "memory"
		}
	}

	if c.TLSEnabled {
		switch c.TLSMode {
		case "file":
			if c.TLSCertFile == "" || c.TLSKeyFile == "" {
				return nil, fmt.Errorf("TLS_CERT_FILE and TLS_KEY_FILE are required when TLS_MODE=file")
			}
		case "kubernetes":
			if c.TLSSecretName == "" {
				return nil, fmt.Errorf("TLS_SECRET_NAME is required when TLS_MODE=kubernetes")
			}
		case "memory":
		default:
			return nil, fmt.Errorf("unknown TLS_MODE %q (file, kubernetes, memory)", c.TLSMode)
		}
	}
	return c, nil
}

// NeedsKube reports whether a Kubernetes API client is required.
func (c *Config) NeedsKube() bool {
	return c.Discovery == "kubernetes" || (c.TLSEnabled && c.TLSMode == "kubernetes")
}

func namespaceFromServiceAccount() string {
	if b, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		return strings.TrimSpace(string(b))
	}
	return "default"
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func boolEnv(key string, def bool) bool {
	if v, err := strconv.ParseBool(os.Getenv(key)); err == nil {
		return v
	}
	return def
}

func intEnv(key string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil {
		return v
	}
	return def
}
