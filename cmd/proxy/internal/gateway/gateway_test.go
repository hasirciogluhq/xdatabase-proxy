package gateway

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/discovery"
	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/protocol"
	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/route"
	"github.com/hasirciogluhq/xdatabase-proxy/pkg/proxy"
	"github.com/hasirciogluhq/xdatabase-proxy/pkg/tlsutil"
)

func testCert(t *testing.T) tls.Certificate {
	t.Helper()
	certPEM, keyPEM, err := tlsutil.SelfSigned()
	if err != nil {
		t.Fatal(err)
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	return cert
}

// tlsEchoBackend is a TLS server (the "database") that echoes what it reads.
func tlsEchoBackend(t *testing.T) string {
	t.Helper()
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{testCert(t)}})
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

// captureBackend is a plaintext server that reports the first bytes it receives, then echoes.
func captureBackend(t *testing.T) (string, <-chan string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	got := make(chan string, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		buf := make([]byte, 1024)
		n, _ := c.Read(buf)
		got <- string(buf[:n])
		io.Copy(c, c)
	}()
	return ln.Addr().String(), got
}

func start(t *testing.T, h *Handler) string {
	t.Helper()
	h.DialTimeout = time.Second
	h.Log = slog.New(slog.DiscardHandler)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	srv := &proxy.Server{Handler: h, ShutdownTimeout: time.Second, Log: h.Log}
	done := make(chan struct{})
	go func() { srv.Serve(ctx, ln); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	return ln.Addr().String()
}

func serverTLS(t *testing.T) func() *tls.Config {
	cert := testCert(t)
	return func() *tls.Config { return &tls.Config{Certificates: []tls.Certificate{cert}} }
}

// denySNI wraps a resolver and refuses the sni strategy, like a Service labelled xdatabase-proxy-routing: username.
type denySNI struct{ route.Resolver }

func (d denySNI) Resolve(ctx context.Context, k route.Key, via route.Strategy) (string, error) {
	if via == route.SNI {
		return "", errors.New("sni disabled by label")
	}
	return d.Resolver.Resolve(ctx, k, via)
}

func echoes(t *testing.T, c net.Conn) {
	t.Helper()
	c.SetDeadline(time.Now().Add(2 * time.Second))
	c.Write([]byte("ping"))
	buf := make([]byte, 4)
	if _, err := io.ReadFull(c, buf); err != nil || string(buf) != "ping" {
		t.Fatalf("echo: %q %v", buf, err)
	}
}

func firstBytes(t *testing.T, got <-chan string) string {
	t.Helper()
	select {
	case s := <-got:
		return s
	case <-time.After(2 * time.Second):
		t.Fatal("backend never received data")
		return ""
	}
}

const redisAuth = "*3\r\n$4\r\nAUTH\r\n$14\r\nalice.db1.pool\r\n$6\r\nsecret\r\n"
const redisAuthPlain = "*3\r\n$4\r\nAUTH\r\n$5\r\nalice\r\n$6\r\nsecret\r\n"

func TestSNIPassthrough(t *testing.T) {
	be := tlsEchoBackend(t)
	addr := start(t, &Handler{Resolver: discovery.Static{"db1": be, "db1.pool": be}})
	for _, name := range []string{"db1.example.com", "db1.pool.example.com"} {
		c, err := tls.Dial("tcp", addr, &tls.Config{ServerName: name, InsecureSkipVerify: true})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		echoes(t, c) // the handshake was done with the backend's certificate
		c.Close()
	}
}

func TestSNIUnknownOrMissingIsClosed(t *testing.T) {
	addr := start(t, &Handler{Resolver: discovery.Static{"db1": tlsEchoBackend(t)}})
	for _, name := range []string{"nope.example.com", ""} {
		if c, err := tls.Dial("tcp", addr, &tls.Config{ServerName: name, InsecureSkipVerify: true}); err == nil {
			c.Close()
			t.Errorf("%q: handshake should fail", name)
		}
	}
}

func TestSNIOnlyProtocolRejectsPlaintext(t *testing.T) {
	be, _ := captureBackend(t)
	addr := start(t, &Handler{Resolver: discovery.Static{"db1.pool": be}})
	c, _ := net.Dial("tcp", addr)
	defer c.Close()
	c.Write([]byte(redisAuth))
	c.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := c.Read(make([]byte, 1)); err == nil {
		t.Fatal("connection should be closed")
	}
}

func TestRedisUsernamePlain(t *testing.T) {
	be, got := captureBackend(t)
	addr := start(t, &Handler{Resolver: discovery.Static{"db1.pool": be}, Login: protocol.Redis{}})
	c, _ := net.Dial("tcp", addr)
	defer c.Close()
	c.Write([]byte(redisAuth))
	if s := firstBytes(t, got); s != redisAuthPlain {
		t.Fatalf("backend saw %q", s)
	}
	echoes(t, c)
}

func TestRedisHelloAuthPlain(t *testing.T) {
	be, got := captureBackend(t)
	addr := start(t, &Handler{Resolver: discovery.Static{"db1": be}, Login: protocol.Redis{}})
	c, _ := net.Dial("tcp", addr)
	defer c.Close()
	c.Write([]byte("*5\r\n$5\r\nHELLO\r\n$1\r\n3\r\n$4\r\nAUTH\r\n$9\r\nalice.db1\r\n$2\r\npw\r\n"))
	if s := firstBytes(t, got); s != "*5\r\n$5\r\nHELLO\r\n$1\r\n3\r\n$4\r\nAUTH\r\n$5\r\nalice\r\n$2\r\npw\r\n" {
		t.Fatalf("backend saw %q", s)
	}
}

// TLS client, no SNI route matches: the proxy terminates TLS and routes by the username.
func TestRedisUsernameOverTerminatedTLS(t *testing.T) {
	be, got := captureBackend(t)
	addr := start(t, &Handler{Resolver: discovery.Static{"db1.pool": be}, Login: protocol.Redis{}, TLS: serverTLS(t)})
	c, err := tls.Dial("tcp", addr, &tls.Config{ServerName: "unrelated.example.com", InsecureSkipVerify: true})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.Write([]byte(redisAuth))
	if s := firstBytes(t, got); s != redisAuthPlain {
		t.Fatalf("backend saw %q", s)
	}
	echoes(t, c)
}

// The operator restricted the Service to username routing: even a matching host name must not be passed through.
func TestSNIDisabledByLabelFallsBackToUsername(t *testing.T) {
	be, got := captureBackend(t)
	res := denySNI{discovery.Static{"db1.pool": be}}
	addr := start(t, &Handler{Resolver: res, Login: protocol.Redis{}, TLS: serverTLS(t)})
	c, err := tls.Dial("tcp", addr, &tls.Config{ServerName: "db1.pool.example.com", InsecureSkipVerify: true})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.Write([]byte(redisAuth))
	if s := firstBytes(t, got); s != redisAuthPlain {
		t.Fatalf("backend saw %q", s)
	}
}

// Both strategies enabled: SNI passthrough wins and the backend terminates TLS.
func TestSNIWinsOverUsernameWhenBothEnabled(t *testing.T) {
	be := tlsEchoBackend(t)
	addr := start(t, &Handler{Resolver: discovery.Static{"db1.pool": be}, Login: protocol.Redis{}, TLS: serverTLS(t)})
	c, err := tls.Dial("tcp", addr, &tls.Config{ServerName: "db1.pool.example.com", InsecureSkipVerify: true})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	echoes(t, c)
}

func TestRedisUnroutableFirstCommandGetsError(t *testing.T) {
	be, _ := captureBackend(t)
	addr := start(t, &Handler{Resolver: discovery.Static{"db1": be}, Login: protocol.Redis{}})
	c, _ := net.Dial("tcp", addr)
	defer c.Close()
	c.Write([]byte("*1\r\n$4\r\nPING\r\n"))
	c.SetDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 64)
	n, _ := c.Read(buf)
	if !strings.HasPrefix(string(buf[:n]), "-ERR ") {
		t.Fatalf("got %q", buf[:n])
	}
}

func TestRedisUnknownDeploymentGetsError(t *testing.T) {
	be, _ := captureBackend(t)
	addr := start(t, &Handler{Resolver: discovery.Static{"db1": be}, Login: protocol.Redis{}})
	c, _ := net.Dial("tcp", addr)
	defer c.Close()
	c.Write([]byte("*3\r\n$4\r\nAUTH\r\n$10\r\nalice.nope\r\n$2\r\npw\r\n"))
	c.SetDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 64)
	n, _ := c.Read(buf)
	if !strings.HasPrefix(string(buf[:n]), "-ERR ") {
		t.Fatalf("got %q", buf[:n])
	}
}

func chHello(user string) []byte {
	var b []byte
	str := func(s string) { b = append(binary.AppendUvarint(b, uint64(len(s))), s...) }
	b = append(b, 0)
	str("ClickHouse client")
	for _, v := range []uint64{24, 8, 54471} {
		b = binary.AppendUvarint(b, v)
	}
	str("default")
	str(user)
	str("pw")
	return b
}

func TestClickHouseUsernamePlain(t *testing.T) {
	be, got := captureBackend(t)
	addr := start(t, &Handler{Resolver: discovery.Static{"db1": be}, Login: protocol.ClickHouse{}})
	c, _ := net.Dial("tcp", addr)
	defer c.Close()
	c.Write(chHello("alice.db1"))
	if s := firstBytes(t, got); s != string(chHello("alice")) {
		t.Fatalf("backend saw % x", s)
	}
	echoes(t, c)
}

// SNI route with TLS terminated at the proxy: the client sees the proxy's certificate and the backend gets plain TCP.
func TestSNITerminateTalksPlainToBackend(t *testing.T) {
	be, got := captureBackend(t)
	addr := start(t, &Handler{Resolver: discovery.Static{"db1": be}, TLS: serverTLS(t), TerminateSNI: true})
	c, err := tls.Dial("tcp", addr, &tls.Config{ServerName: "db1.example.com", InsecureSkipVerify: true})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.Write([]byte("hello"))
	if s := firstBytes(t, got); s != "hello" {
		t.Fatalf("backend saw %q, want plaintext", s)
	}
	echoes(t, c)
}

func TestSNITerminateUnknownHostIsClosed(t *testing.T) {
	be, _ := captureBackend(t)
	addr := start(t, &Handler{Resolver: discovery.Static{"db1": be}, TLS: serverTLS(t), TerminateSNI: true})
	if c, err := tls.Dial("tcp", addr, &tls.Config{ServerName: "nope.example.com", InsecureSkipVerify: true}); err == nil {
		c.Close()
		t.Fatal("handshake should fail for an unknown deployment")
	}
}
