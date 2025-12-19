package kubernetes

import (
	"context"
	"fmt"
	"time"

	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/core"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type K8sResolver struct {
	store cache.Store
}

func NewK8sResolver(clientset *kubernetes.Clientset) *K8sResolver {
	factory := informers.NewSharedInformerFactory(clientset, 10*time.Minute)
	serviceInformer := factory.Core().V1().Services().Informer()

	// Start the informer in the background
	stopCh := make(chan struct{})
	go factory.Start(stopCh)
	factory.WaitForCacheSync(stopCh)

	return &K8sResolver{
		store: serviceInformer.GetStore(),
	}
}

func (r *K8sResolver) Resolve(ctx context.Context, metadata core.RoutingMetadata) (string, error) {
	dbName, ok := metadata["database"]
	if !ok {
		return "", fmt.Errorf("metadata missing 'database' key")
	}

	// Scan services for matching annotation
	// In a real implementation, use an Indexer for O(1) lookup
	for _, obj := range r.store.List() {
		svc, ok := obj.(*corev1.Service)
		if !ok {
			continue
		}

		if targetDB, exists := svc.Annotations["xdatabase-proxy/db-name"]; exists && targetDB == dbName {
			return fmt.Sprintf("%s.%s.svc.cluster.local:%d", svc.Name, svc.Namespace, svc.Spec.Ports[0].Port), nil
		}
	}

	return "", fmt.Errorf("database '%s' not found", dbName)
}
