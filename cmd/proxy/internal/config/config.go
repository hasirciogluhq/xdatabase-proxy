package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// RuntimeEnvironment represents the execution environment
type RuntimeEnvironment string

const (
	RuntimeKubernetes RuntimeEnvironment = "kubernetes"
	RuntimeContainer  RuntimeEnvironment = "container"
	RuntimeVM         RuntimeEnvironment = "vm"
)

// DiscoveryMode represents backend discovery strategy
type DiscoveryMode string

const (
	DiscoveryKubernetes DiscoveryMode = "kubernetes"
	DiscoveryStatic     DiscoveryMode = "static"
)

// TLSMode represents TLS certificate source
type TLSMode string

const (
	TLSModeFile       TLSMode = "file"
	TLSModeKubernetes TLSMode = "kubernetes"
	TLSModeMemory     TLSMode = "memory"
)

// Config holds all application configuration
type Config struct {
	// Core
	Debug        bool
	DatabaseType string // postgresql, mysql, mongodb

	// Runtime
	Runtime   RuntimeEnvironment
	Namespace string // Only for Kubernetes runtime

	// Server
	HealthServerPort string
	ProxyStartPort   string

	// Backend Discovery
	DiscoveryMode  DiscoveryMode
	StaticBackends string
	KubeConfigPath string
	KubeContext    string

	// TLS Configuration
	TLSEnabled              bool
	TLSMode                 TLSMode
	TLSCertFile             string
	TLSKeyFile              string
	TLSSecretName           string
	TLSAutoGenerate         bool // Generate self-signed if cert doesn't exist
	TLSAutoRenew            bool // Regenerate if cert is invalid/expired
	TLSRenewalThresholdDays int  // Days before expiry to trigger renewal
}

// LoadFromEnv loads configuration from environment variables
func LoadFromEnv() (*Config, error) {
	cfg := &Config{
		// Core
		Debug:        getEnvBool("DEBUG", false),
		DatabaseType: getEnv("DATABASE_TYPE", "postgresql"),

		// Runtime - Auto-detect or explicit
		Runtime:   determineRuntime(),
		Namespace: determineNamespace(),

		// Server
		HealthServerPort: getEnv("HEALTH_SERVER_PORT", "8080"),
		ProxyStartPort:   getEnv("PROXY_START_PORT", "5432"),

		// Backend Discovery
		DiscoveryMode:  determineDiscoveryMode(),
		StaticBackends: getEnv("STATIC_BACKENDS", ""),
		KubeConfigPath: getEnv("KUBECONFIG", ""),
		KubeContext:    getEnv("KUBE_CONTEXT", ""),

		// TLS
		TLSEnabled:              getEnvBool("TLS_ENABLED", true),
		TLSMode:                 determineTLSMode(),
		TLSCertFile:             getEnv("TLS_CERT_FILE", ""),
		TLSKeyFile:              getEnv("TLS_KEY_FILE", ""),
		TLSSecretName:           getEnv("TLS_SECRET_NAME", ""),
		TLSAutoGenerate:         getEnvBool("TLS_AUTO_GENERATE", true),
		TLSAutoRenew:            getEnvBool("TLS_AUTO_RENEW", true),
		TLSRenewalThresholdDays: getEnvInt("TLS_RENEWAL_THRESHOLD_DAYS", 30),
	}

	// Legacy support
	cfg.applyLegacySupport()

	// Validation
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// validate ensures configuration is coherent
func (c *Config) validate() error {
	// Validate database type
	validDatabases := []string{"postgresql", "mysql", "mongodb"}
	if !contains(validDatabases, c.DatabaseType) {
		return fmt.Errorf("unsupported DATABASE_TYPE: %s (supported: %s)",
			c.DatabaseType, strings.Join(validDatabases, ", "))
	}

	// TLS validation only if TLS is enabled
	if c.TLSEnabled {
		if c.TLSMode == TLSModeFile {
			if c.TLSCertFile == "" || c.TLSKeyFile == "" {
				return fmt.Errorf("TLS_CERT_FILE and TLS_KEY_FILE must be set when using file-based TLS")
			}
		}

		if c.TLSMode == TLSModeKubernetes {
			if c.TLSSecretName == "" {
				return fmt.Errorf("TLS_SECRET_NAME must be set when using kubernetes TLS mode")
			}
			if c.DiscoveryMode == DiscoveryStatic {
				return fmt.Errorf("kubernetes TLS mode requires kubernetes discovery (cannot use STATIC_BACKENDS)")
			}
		}
	}

	// Validate discovery mode
	if c.DiscoveryMode == DiscoveryKubernetes && c.Runtime == RuntimeContainer && c.KubeConfigPath == "" {
		return fmt.Errorf("kubernetes discovery in container runtime requires KUBECONFIG path")
	}

	return nil
}

// applyLegacySupport handles backward compatibility
func (c *Config) applyLegacySupport() {
	// Legacy: POSTGRESQL_PROXY_ENABLED
	if getEnvBool("POSTGRESQL_PROXY_ENABLED", false) {
		c.DatabaseType = "postgresql"
	}

	// Legacy: POSTGRESQL_PROXY_START_PORT
	if legacyPort := getEnv("POSTGRESQL_PROXY_START_PORT", ""); legacyPort != "" {
		c.ProxyStartPort = legacyPort
	}

	// Legacy: TLS_ENABLE_SELF_SIGNED
	if getEnvBool("TLS_ENABLE_SELF_SIGNED", false) {
		c.TLSAutoGenerate = true
	}

	// Legacy: POD_NAMESPACE
	if podNS := getEnv("POD_NAMESPACE", ""); podNS != "" && c.Namespace == "" {
		c.Namespace = podNS
	}
}

// Helper functions

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	boolValue, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}
	return boolValue
}

func getEnvInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	intValue, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return intValue
}

func determineRuntime() RuntimeEnvironment {
	// Explicit runtime setting
	if runtime := os.Getenv("RUNTIME"); runtime != "" {
		switch strings.ToLower(runtime) {
		case "kubernetes", "k8s":
			return RuntimeKubernetes
		case "container", "docker":
			return RuntimeContainer
		case "vm", "virtual-machine", "bare-metal":
			return RuntimeVM
		}
	}

	// Auto-detect: Check if running in Kubernetes
	if _, err := os.Stat("/var/run/secrets/kubernetes.io/serviceaccount"); err == nil {
		return RuntimeKubernetes
	}

	// Auto-detect: Check if running in container
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return RuntimeContainer
	}

	// Default to VM
	return RuntimeVM
}

func determineNamespace() string {
	// Explicit namespace
	if ns := os.Getenv("NAMESPACE"); ns != "" {
		return ns
	}

	// Kubernetes downward API
	if ns := os.Getenv("POD_NAMESPACE"); ns != "" {
		return ns
	}

	// Read from service account (in-cluster)
	if data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		return strings.TrimSpace(string(data))
	}

	return "default"
}

func determineDiscoveryMode() DiscoveryMode {
	// Explicit mode
	if mode := os.Getenv("DISCOVERY_MODE"); mode != "" {
		if strings.ToLower(mode) == "static" {
			return DiscoveryStatic
		}
		return DiscoveryKubernetes
	}

	// Auto-detect: Static if STATIC_BACKENDS is set
	if os.Getenv("STATIC_BACKENDS") != "" {
		return DiscoveryStatic
	}

	return DiscoveryKubernetes
}

func determineTLSMode() TLSMode {
	// Explicit mode
	if mode := os.Getenv("TLS_MODE"); mode != "" {
		switch strings.ToLower(mode) {
		case "file", "filesystem":
			return TLSModeFile
		case "kubernetes", "k8s", "secret":
			return TLSModeKubernetes
		case "memory", "in-memory":
			return TLSModeMemory
		}
	}

	// Auto-detect based on configuration
	if os.Getenv("TLS_CERT_FILE") != "" {
		return TLSModeFile
	}

	if os.Getenv("TLS_SECRET_NAME") != "" {
		return TLSModeKubernetes
	}

	return TLSModeMemory
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
