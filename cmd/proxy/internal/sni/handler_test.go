package sni

import (
	"context"
	"crypto/tls"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/discovery"
	"github.com/hasirciogluhq/xdatabase-proxy/pkg/proxy"
	"github.com/hasirciogluhq/xdatabase-proxy/pkg/tlsutil"
)

func TestParseHost(t *testing.T) {
	for _, c := range []struct {
		in, dep string
		pooled  bool
	}{
		{"db1.example.com", "db1", false},
		{"db1.pool.example.com", "db1", true},
		{"DB1.Pool.Example.com.", "db1", true},
		{"db1", "db1", false},
		{"db1.pool", "db1", false},
	} {
		if d, p := ParseHost(c.in); d != c.dep || p != c.pooled {
			t.Errorf("%q: got (%q,%v)", c.in, d, p)
		}
	}
}

// tlsEchoBackend is a TLS server (the "database") that echoes what it reads.
func tlsEchoBackend(t *testing.T) string {
	t.Helper()
	certPEM, keyPEM, _ := tlsutil.SelfSigned()
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() { defer c.Close(); io.Copy(c, c) }()
		}
	}()
	return ln.Addr().String()
}

func startProxy(t *testing.T, backend string) string {
	t.Helper()
	log := slog.New(slog.DiscardHandler)
	h := &Handler{Resolver: discovery.Static{"db1": backend, "db1.pool": backend}, DialTimeout: time.Second, Log: log}
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	ctx, cancel := context.WithCancel(context.Background())
	srv := &proxy.Server{Handler: h, ShutdownTimeout: time.Second, Log: log}
	done := make(chan struct{})
	go func() { srv.Serve(ctx, ln); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	return ln.Addr().String()
}

func TestPassthroughRoutesBySNI(t *testing.T) {
	addr := startProxy(t, tlsEchoBackend(t))
	for _, name := range []string{"db1.example.com", "db1.pool.example.com"} {
		c, err := tls.Dial("tcp", addr, &tls.Config{ServerName: name, InsecureSkipVerify: true})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		c.SetDeadline(time.Now().Add(2 * time.Second))
		c.Write([]byte("ping"))
		buf := make([]byte, 4)
		if _, err := io.ReadFull(c, buf); err != nil || string(buf) != "ping" {
			t.Fatalf("%s: echo %q %v", name, buf, err)
		}
		c.Close()
	}
}

func TestUnknownOrMissingSNIIsClosed(t *testing.T) {
	addr := startProxy(t, tlsEchoBackend(t))
	for _, name := range []string{"nope.example.com", ""} {
		cfg := &tls.Config{ServerName: name, InsecureSkipVerify: true}
		if c, err := tls.Dial("tcp", addr, cfg); err == nil {
			c.Close()
			t.Errorf("%q: handshake should fail", name)
		}
	}
}
