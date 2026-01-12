package factory

import (
	"context"
	"fmt"
	"os"

	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/config"
	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/core"
	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/discovery/kubernetes"
	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/discovery/memory"
	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/logger"

	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// ResolverFactory creates backend resolvers based on configuration
type ResolverFactory struct {
	cfg *config.Config
}

// NewResolverFactory creates a new resolver factory
func NewResolverFactory(cfg *config.Config) *ResolverFactory {
	return &ResolverFactory{cfg: cfg}
}

// Create creates a backend resolver based on configuration
func (f *ResolverFactory) Create(ctx context.Context) (core.BackendResolver, *k8s.Clientset, error) {
	switch f.cfg.DiscoveryMode {
	case config.DiscoveryStatic:
		return f.createStaticResolver()
	case config.DiscoveryKubernetes:
		return f.createKubernetesResolver()
	default:
		return nil, nil, fmt.Errorf("unknown discovery mode: %s", f.cfg.DiscoveryMode)
	}
}

func (f *ResolverFactory) createStaticResolver() (core.BackendResolver, *k8s.Clientset, error) {
	logger.Info("Creating Static Backend Resolver", "backends", f.cfg.StaticBackends)

	resolver, err := memory.NewResolver(f.cfg.StaticBackends)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create static resolver: %w", err)
	}

	return resolver, nil, nil
}

func (f *ResolverFactory) createKubernetesResolver() (core.BackendResolver, *k8s.Clientset, error) {
	logger.Info("Creating Kubernetes Backend Resolver",
		"runtime", f.cfg.Runtime,
		"kubeconfig", f.cfg.KubeConfigPath,
		"context", f.cfg.KubeContext)

	kubeconfig := f.cfg.KubeConfigPath

	// For non-Kubernetes runtime, kubeconfig is required
	if f.cfg.Runtime != config.RuntimeKubernetes && kubeconfig == "" {
		if home := os.Getenv("HOME"); home != "" {
			kubeconfig = home + "/.kube/config"
		}
	}

	configOverrides := &clientcmd.ConfigOverrides{}
	if f.cfg.KubeContext != "" {
		configOverrides.CurrentContext = f.cfg.KubeContext
		logger.Info("Using specific Kubernetes context", "context", f.cfg.KubeContext)
	}

	var config *rest.Config
	var err error

	// Try kubeconfig first (for VM/Container runtime or explicit config)
	if kubeconfig != "" {
		config, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig},
			configOverrides,
		).ClientConfig()

		if err != nil {
			logger.Warn("Failed to load kubeconfig, will try in-cluster config", "error", err)
		}
	}

	// Fallback to in-cluster config (for Kubernetes runtime)
	if config == nil {
		logger.Info("Attempting in-cluster Kubernetes configuration")
		config, err = clientcmd.BuildConfigFromFlags("", "")
		if err != nil {
			return nil, nil, fmt.Errorf("failed to build kubernetes config (tried kubeconfig and in-cluster): %w", err)
		}
	}

	clientset, err := k8s.NewForConfig(config)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	resolver := kubernetes.NewK8sResolver(clientset)
	logger.Info("Kubernetes resolver created successfully")
	return resolver, clientset, nil
}
