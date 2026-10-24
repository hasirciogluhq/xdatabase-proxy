package postgres

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
	"github.com/jackc/pgx/v5/pgproto3"
)

func TestParseUser(t *testing.T) {
	for _, c := range []struct {
		in, name, dep string
		pooled        bool
	}{
		{"alice.db1", "alice", "db1", false},
		{"alice.db1.pool", "alice", "db1", true},
		{"a.b.db1.pool", "a.b", "db1", true},
		{"a.b.db1", "a.b", "db1", false},
		{"alice", "alice", "", false},
		{"alice.pool", "alice.pool", "", false},
	} {
		n, d, p := ParseUser(c.in)
		if n != c.name || d != c.dep || p != c.pooled {
			t.Errorf("%q: got (%q,%q,%v)", c.in, n, d, p)
		}
	}
}

// startProxy runs a Handler in front of a fake backend and returns the proxy
// address plus a channel delivering the startup message the backend received.
func startProxy(t *testing.T, tlsCfg func() *tls.Config) (string, <-chan *pgproto3.StartupMessage) {
	t.Helper()
	backend, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	got := make(chan *pgproto3.StartupMessage, 1)
	go func() {
		c, err := backend.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		msg, err := pgproto3.NewBackend(c, c).ReceiveStartupMessage()
		if err != nil {
			return
		}
		got <- msg.(*pgproto3.StartupMessage)
		io.Copy(c, c) // echo
	}()

	h := &Handler{
		Resolver:    discovery.Static{"db1.pool": backend.Addr().String()},
		TLS:         tlsCfg,
		DialTimeout: time.Second,
		Log:         slog.New(slog.DiscardHandler),
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	srv := &proxy.Server{Handler: h, ShutdownTimeout: time.Second, Log: h.Log}
	done := make(chan struct{})
	go func() { srv.Serve(ctx, ln); close(done) }()
	t.Cleanup(func() { cancel(); <-done; backend.Close() })
	return ln.Addr().String(), got
}

func sendStartup(t *testing.T, c net.Conn) {
	t.Helper()
	st := &pgproto3.StartupMessage{ProtocolVersion: pgproto3.ProtocolVersion30,
		Parameters: map[string]string{"user": "alice.db1.pool", "database": "alice.db1.pool"}}
	buf, _ := st.Encode(nil)
	if _, err := c.Write(buf); err != nil {
		t.Fatal(err)
	}
}

func checkRouted(t *testing.T, c net.Conn, got <-chan *pgproto3.StartupMessage) {
	t.Helper()
	select {
	case st := <-got:
		if st.Parameters["user"] != "alice" || st.Parameters["database"] != "postgres" {
			t.Fatalf("backend saw %+v", st.Parameters)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("backend never received startup")
	}
	c.SetDeadline(time.Now().Add(2 * time.Second))
	c.Write([]byte("ping"))
	buf := make([]byte, 4)
	if _, err := io.ReadFull(c, buf); err != nil || string(buf) != "ping" {
		t.Fatalf("echo: %q %v", buf, err)
	}
}

func TestRoutePlain(t *testing.T) {
	addr, got := startProxy(t, nil)
	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	sendStartup(t, c)
	checkRouted(t, c, got)
}

func TestSSLDeclinedWhenTLSDisabled(t *testing.T) {
	addr, got := startProxy(t, nil)
	c, _ := net.Dial("tcp", addr)
	defer c.Close()
	req, _ := (&pgproto3.SSLRequest{}).Encode(nil)
	c.Write(req)
	var resp [1]byte
	if _, err := io.ReadFull(c, resp[:]); err != nil || resp[0] != 'N' {
		t.Fatalf("want N, got %v %v", resp, err)
	}
	sendStartup(t, c)
	checkRouted(t, c, got)
}

func TestRouteTLS(t *testing.T) {
	certPEM, keyPEM, err := tlsutil.SelfSigned()
	if err != nil {
		t.Fatal(err)
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	addr, got := startProxy(t, func() *tls.Config { return &tls.Config{Certificates: []tls.Certificate{cert}} })
	raw, _ := net.Dial("tcp", addr)
	defer raw.Close()
	req, _ := (&pgproto3.SSLRequest{}).Encode(nil)
	raw.Write(req)
	var resp [1]byte
	if _, err := io.ReadFull(raw, resp[:]); err != nil || resp[0] != 'S' {
		t.Fatalf("want S, got %v %v", resp, err)
	}
	c := tls.Client(raw, &tls.Config{InsecureSkipVerify: true})
	sendStartup(t, c)
	checkRouted(t, c, got)
}

func TestUnknownDeploymentGetsError(t *testing.T) {
	addr, _ := startProxy(t, nil)
	c, _ := net.Dial("tcp", addr)
	defer c.Close()
	st := &pgproto3.StartupMessage{ProtocolVersion: pgproto3.ProtocolVersion30, Parameters: map[string]string{"user": "alice.nope"}}
	buf, _ := st.Encode(nil)
	c.Write(buf)
	c.SetDeadline(time.Now().Add(2 * time.Second))
	var typ [1]byte
	if _, err := io.ReadFull(c, typ[:]); err != nil || typ[0] != 'E' {
		t.Fatalf("want ErrorResponse, got %v %v", typ, err)
	}
}
