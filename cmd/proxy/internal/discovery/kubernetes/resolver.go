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

func (r *K8sResolver) Resolve(ctx context.Context, metadata core.RoutingMetadata, databaseType core.DatabaseType) (string, error) {
	deploymentID, ok := metadata["deployment_id"]
	if !ok {
		return "", fmt.Errorf("metadata missing 'deployment_id' (check connection string format: user.deployment_id[.pool])")
	}
	pooled := metadata["pooled"] // "true" or "false"

	// Scan services for matching labels
	for _, obj := range r.store.List() {
		svc, ok := obj.(*corev1.Service)
		if !ok {
			continue
		}

		labels := svc.Labels
		if labels["xdatabase-proxy-enabled"] != "true" {
			continue
		}

		if labels["xdatabase-proxy-database-type"] != string(databaseType) {
			continue
		}

		if labels["xdatabase-proxy-deployment-id"] == deploymentID &&
			labels["xdatabase-proxy-pooled"] == pooled {

			// Find the target port
			// If xdatabase-proxy-destination-port label is set, use it to find the port in Spec
			// Otherwise use the first port
			var port int32
			// We are ignoring the specific port value from label for now and just taking the first port
			// In a more robust implementation, we should parse destPortStr and find the matching port in Spec
			if _, ok := labels["xdatabase-proxy-destination-port"]; ok {
				if len(svc.Spec.Ports) > 0 {
					port = svc.Spec.Ports[0].Port
				}
			} else {
				if len(svc.Spec.Ports) > 0 {
					port = svc.Spec.Ports[0].Port
				}
			}

			if port == 0 {
				continue
			}

			return fmt.Sprintf("%s.%s.svc.cluster.local:%d", svc.Name, svc.Namespace, port), nil
		}
	}

	return "", fmt.Errorf("service not found for deployment_id='%s', pooled='%s'", deploymentID, pooled)
}
