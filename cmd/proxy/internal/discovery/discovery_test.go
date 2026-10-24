package discovery

import (
	"context"
	"testing"
	"time"

	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/route"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestStatic(t *testing.T) {
	s, err := NewStatic("db1=h1:5432, db1.pool = h1:6432")
	if err != nil {
		t.Fatal(err)
	}
	if a, _ := s.Resolve(nil, route.Key{Deployment: "db1"}, route.Username); a != "h1:5432" {
		t.Fatal(a)
	}
	if a, _ := s.Resolve(nil, route.Key{Deployment: "db1", Pooled: true}, route.SNI); a != "h1:6432" {
		t.Fatal(a)
	}
	if _, err := s.Resolve(nil, route.Key{Deployment: "x"}, route.Username); err == nil {
		t.Fatal("expected error")
	}
	if _, err := NewStatic("broken"); err == nil {
		t.Fatal("expected parse error")
	}
}

func service(name, deployment, pooled string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns", Labels: map[string]string{
			"xdatabase-proxy-enabled":       "true",
			"xdatabase-proxy-database-type": "postgresql",
			"xdatabase-proxy-deployment-id": deployment,
			"xdatabase-proxy-pooled":        pooled,
		}},
		Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{{Name: "metrics", Port: 9187}, {Name: "pg", Port: 5432}}},
	}
}

func TestKubernetesInformer(t *testing.T) {
	cs := fake.NewSimpleClientset(service("a", "db1", "false"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	k, err := NewKubernetes(ctx, cs, "", "postgresql", "cluster.local", []route.Strategy{route.Username})
	if err != nil {
		t.Fatal(err)
	}
	if a, err := k.Resolve(ctx, route.Key{Deployment: "db1"}, route.Username); err != nil || a != "a.ns.svc.cluster.local:9187" {
		t.Fatalf("got %q %v", a, err)
	}

	svcs := cs.CoreV1().Services("ns")
	b := service("b", "db2", "true")
	b.Labels["xdatabase-proxy-destination-port"] = "pg"
	svcs.Create(ctx, b, metav1.CreateOptions{})
	svcs.Delete(ctx, "a", metav1.DeleteOptions{})
	waitFor(t, func() bool {
		a, _ := k.Resolve(ctx, route.Key{Deployment: "db2", Pooled: true}, route.Username)
		return a == "b.ns.svc.cluster.local:5432"
	})
	waitFor(t, func() bool { _, err := k.Resolve(ctx, route.Key{Deployment: "db1"}, route.Username); return err != nil })
}

// The xdatabase-proxy-routing label (or the type's defaults) decides which strategies may reach a Service.
func TestKubernetesRoutingLabel(t *testing.T) {
	def := service("default", "d", "false")
	sniOnly := service("sni", "s", "false")
	sniOnly.Labels["xdatabase-proxy-routing"] = "sni"
	userOnly := service("user", "u", "false")
	userOnly.Labels["xdatabase-proxy-routing"] = "username"
	all := service("all", "a", "false")
	all.Labels["xdatabase-proxy-routing"] = "all"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	k, err := NewKubernetes(ctx, fake.NewSimpleClientset(def, sniOnly, userOnly, all), "", "postgresql", "cluster.local",
		[]route.Strategy{route.Username})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		dep string
		via route.Strategy
		ok  bool
	}{
		{"d", route.Username, true}, {"d", route.SNI, false}, // default for this type: username only
		{"s", route.SNI, true}, {"s", route.Username, false},
		{"u", route.Username, true}, {"u", route.SNI, false},
		{"a", route.Username, true}, {"a", route.SNI, true},
	} {
		_, err := k.Resolve(ctx, route.Key{Deployment: c.dep}, c.via)
		if (err == nil) != c.ok {
			t.Errorf("%s via %s: err=%v, want ok=%v", c.dep, c.via, err, c.ok)
		}
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	for i := 0; i < 200; i++ {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met")
}
