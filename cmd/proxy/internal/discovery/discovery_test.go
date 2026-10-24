package discovery

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestStatic(t *testing.T) {
	s, err := NewStatic("db1=h1:5432, db1.pool = h1:6432")
	if err != nil {
		t.Fatal(err)
	}
	if a, _ := s.Resolve(nil, "db1", false); a != "h1:5432" {
		t.Fatal(a)
	}
	if a, _ := s.Resolve(nil, "db1", true); a != "h1:6432" {
		t.Fatal(a)
	}
	if _, err := s.Resolve(nil, "x", false); err == nil {
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
	k, err := NewKubernetes(ctx, cs, "", "postgresql", "cluster.local")
	if err != nil {
		t.Fatal(err)
	}
	if a, err := k.Resolve(ctx, "db1", false); err != nil || a != "a.ns.svc.cluster.local:9187" {
		t.Fatalf("got %q %v", a, err)
	}

	svcs := cs.CoreV1().Services("ns")
	b := service("b", "db2", "true")
	b.Labels["xdatabase-proxy-destination-port"] = "pg"
	svcs.Create(ctx, b, metav1.CreateOptions{})
	svcs.Delete(ctx, "a", metav1.DeleteOptions{})
	waitFor(t, func() bool { a, _ := k.Resolve(ctx, "db2", true); return a == "b.ns.svc.cluster.local:5432" })
	waitFor(t, func() bool { _, err := k.Resolve(ctx, "db1", false); return err != nil })
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
