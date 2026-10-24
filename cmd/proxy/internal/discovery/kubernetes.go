package discovery

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"time"

	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/route"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const (
	labelEnabled     = "xdatabase-proxy-enabled"
	labelType        = "xdatabase-proxy-database-type"
	labelDeployment  = "xdatabase-proxy-deployment-id"
	labelPooled      = "xdatabase-proxy-pooled"
	labelDestination = "xdatabase-proxy-destination-port"

	labelRouting = "xdatabase-proxy-routing"

	routeIndex = "route"
)

// Kubernetes resolves routes from labelled Services using a shared informer
// with a route index, so Resolve is a local O(1) lookup.
type Kubernetes struct {
	idx      cache.Indexer
	domain   string
	defaults []route.Strategy
}

// NewKubernetes starts the informer and waits for the first sync.
// namespace == "" watches all namespaces. defaults are the strategies allowed
// for Services without the xdatabase-proxy-routing label.
func NewKubernetes(ctx context.Context, cs kubernetes.Interface, namespace, dbType, clusterDomain string, defaults []route.Strategy) (*Kubernetes, error) {
	factory := informers.NewSharedInformerFactoryWithOptions(cs, 10*time.Minute,
		informers.WithNamespace(namespace),
		informers.WithTweakListOptions(func(o *metav1.ListOptions) {
			o.LabelSelector = labelEnabled + "=true," + labelType + "=" + dbType
		}))
	inf := factory.Core().V1().Services().Informer()
	if err := inf.AddIndexers(cache.Indexers{routeIndex: func(obj any) ([]string, error) {
		if key := routeOf(obj.(*corev1.Service)); key != "" {
			return []string{key}, nil
		}
		return nil, nil
	}}); err != nil {
		return nil, err
	}
	factory.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), inf.HasSynced) {
		return nil, fmt.Errorf("service informer failed to sync")
	}
	return &Kubernetes{idx: inf.GetIndexer(), domain: clusterDomain, defaults: defaults}, nil
}

func routeKey(deployment string, pooled bool) string {
	return deployment + "|" + strconv.FormatBool(pooled)
}

func routeOf(s *corev1.Service) string {
	l := s.Labels
	if l[labelDeployment] == "" || len(s.Spec.Ports) == 0 {
		return ""
	}
	return routeKey(l[labelDeployment], l[labelPooled] == "true")
}

func (k *Kubernetes) Resolve(_ context.Context, key route.Key, via route.Strategy) (string, error) {
	objs, err := k.idx.ByIndex(routeIndex, routeKey(key.Deployment, key.Pooled))
	if err != nil || len(objs) == 0 {
		return "", fmt.Errorf("no service for deployment %q (pooled=%t)", key.Deployment, key.Pooled)
	}
	svcs := make([]*corev1.Service, 0, len(objs))
	for _, o := range objs {
		if s := o.(*corev1.Service); k.allows(s, via) {
			svcs = append(svcs, s)
		}
	}
	if len(svcs) == 0 {
		return "", fmt.Errorf("deployment %q is not routable via %s (label %s)", key.Deployment, via, labelRouting)
	}
	// Several services on one route: pick deterministically.
	sort.Slice(svcs, func(i, j int) bool {
		return svcs[i].Namespace+"/"+svcs[i].Name < svcs[j].Namespace+"/"+svcs[j].Name
	})
	s := svcs[0]
	return fmt.Sprintf("%s.%s.svc.%s:%d", s.Name, s.Namespace, k.domain, targetPort(s)), nil
}

// allows applies the xdatabase-proxy-routing label (username, sni or all);
// without it the database type's defaults apply.
func (k *Kubernetes) allows(s *corev1.Service, via route.Strategy) bool {
	switch v := s.Labels[labelRouting]; v {
	case "all":
		return true
	case string(route.Username), string(route.SNI):
		return v == string(via)
	}
	return slices.Contains(k.defaults, via)
}

// targetPort honours the destination-port label (port number or name), else the first port.
func targetPort(s *corev1.Service) int32 {
	if d := s.Labels[labelDestination]; d != "" {
		for _, p := range s.Spec.Ports {
			if d == strconv.Itoa(int(p.Port)) || d == p.Name {
				return p.Port
			}
		}
	}
	return s.Spec.Ports[0].Port
}
