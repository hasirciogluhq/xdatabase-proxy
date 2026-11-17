package discovery

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/route"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

// These tests pin effective routing for manifests written before v2.1, where an
// implicit pooled default could change which Service a login reaches.

func newTestResolver(t *testing.T, objs ...runtime.Object) *Kubernetes {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	k, err := NewKubernetes(ctx, fake.NewSimpleClientset(objs...), "", "postgresql", "cluster.local", []route.Strategy{route.Username})
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func captureWarnings(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

// A Service without the pooled label is a direct endpoint, and a .pool login
// must fail loudly instead of falling through to it.
func TestMissingPooledLabelIsDirect(t *testing.T) {
	svc := service("orders-db", "orders", "")
	delete(svc.Labels, labelPooled)
	svc.Labels[labelDestination] = "pg"
	k := newTestResolver(t, svc)

	if a, err := k.Resolve(context.Background(), route.Key{Deployment: "orders"}, route.Username); err != nil || a != "orders-db.ns.svc.cluster.local:5432" {
		t.Fatalf("direct route: got %q %v", a, err)
	}
	if a, err := k.Resolve(context.Background(), route.Key{Deployment: "orders", Pooled: true}, route.Username); err == nil {
		t.Fatalf("pooled route without a pooled Service must fail, got %q", a)
	}
}

// A pooler whose pooled label was forgotten shares the direct route. The pick
// stays deterministic, and the ambiguity is reported once.
func TestAmbiguousRouteIsReported(t *testing.T) {
	direct := service("db-prod-postgres", "db-prod", "false")
	pooler := service("db-prod-pgbouncer", "db-prod", "")
	delete(pooler.Labels, labelPooled)
	k := newTestResolver(t, direct, pooler)
	logs := captureWarnings(t)

	for range 3 {
		a, err := k.Resolve(context.Background(), route.Key{Deployment: "db-prod"}, route.Username)
		if err != nil || !strings.HasPrefix(a, "db-prod-pgbouncer.ns.") {
			t.Fatalf("got %q %v", a, err)
		}
	}
	if n := strings.Count(logs.String(), "several services match one route"); n != 1 {
		t.Fatalf("want exactly one warning, got %d: %s", n, logs.String())
	}
	if !strings.Contains(logs.String(), "ns/db-prod-postgres") {
		t.Fatalf("warning should list every candidate: %s", logs.String())
	}
}

// A destination-port label that matches no port must not fall back to the
// first port (here the metrics port).
func TestUnknownDestinationPortFails(t *testing.T) {
	svc := service("orders-db", "orders", "false")
	svc.Labels[labelDestination] = "pgg"
	k := newTestResolver(t, svc)

	if a, err := k.Resolve(context.Background(), route.Key{Deployment: "orders"}, route.Username); err == nil {
		t.Fatalf("want error for unknown destination port, got %q", a)
	}
}
