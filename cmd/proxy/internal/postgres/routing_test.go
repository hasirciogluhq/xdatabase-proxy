package postgres

import (
	"crypto/tls"
	"io"
	"net"
	"testing"
	"time"

	"github.com/hasirciogluhq/xdatabase-proxy/pkg/tlsutil"
	"github.com/jackc/pgx/v5/pgproto3"
)

func testTLS(t *testing.T) func() *tls.Config {
	t.Helper()
	certPEM, keyPEM, err := tlsutil.SelfSigned()
	if err != nil {
		t.Fatal(err)
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	return func() *tls.Config { return &tls.Config{Certificates: []tls.Certificate{cert}} }
}

func writeStartup(t *testing.T, c net.Conn, params map[string]string) {
	t.Helper()
	buf, _ := (&pgproto3.StartupMessage{ProtocolVersion: pgproto3.ProtocolVersion30, Parameters: params}).Encode(nil)
	if _, err := c.Write(buf); err != nil {
		t.Fatal(err)
	}
}

// The login has no deployment, the TLS server name decides: db1.pool.example.com -> db1 pooled.
func TestRouteBySNIAfterSSLRequest(t *testing.T) {
	addr, got := startProxy(t, testTLS(t))
	raw, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	req, _ := (&pgproto3.SSLRequest{}).Encode(nil)
	raw.Write(req)
	var resp [1]byte
	if _, err := io.ReadFull(raw, resp[:]); err != nil || resp[0] != 'S' {
		t.Fatalf("want S, got %v %v", resp, err)
	}
	c := tls.Client(raw, &tls.Config{ServerName: "db1.pool.example.com", InsecureSkipVerify: true})
	writeStartup(t, c, map[string]string{"user": "alice"})
	checkRouted(t, c, got)
}

// PostgreSQL 17 clients with sslnegotiation=direct start with a TLS handshake and ALPN "postgresql".
func TestDirectTLS(t *testing.T) {
	addr, got := startProxy(t, testTLS(t))
	c, err := tls.Dial("tcp", addr, &tls.Config{
		ServerName: "db1.pool.example.com", InsecureSkipVerify: true, NextProtos: []string{"postgresql"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if p := c.ConnectionState().NegotiatedProtocol; p != "postgresql" {
		t.Fatalf("ALPN = %q", p)
	}
	writeStartup(t, c, map[string]string{"user": "alice"}) // deployment from SNI
	checkRouted(t, c, got)
}

func TestDirectTLSRejectedWhenTLSDisabled(t *testing.T) {
	addr, _ := startProxy(t, nil)
	c, err := tls.Dial("tcp", addr, &tls.Config{InsecureSkipVerify: true})
	if err == nil {
		c.Close()
		t.Fatal("handshake should fail")
	}
}

// The login deployment wins over the server name.
func TestLoginWinsOverSNI(t *testing.T) {
	addr, got := startProxy(t, testTLS(t))
	c, err := tls.Dial("tcp", addr, &tls.Config{ServerName: "other.example.com", InsecureSkipVerify: true})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	writeStartup(t, c, map[string]string{"user": "alice.db1.pool", "database": "alice.db1.pool"})
	checkRouted(t, c, got)
}

// Compatible databases rely on extra startup parameters (options, application_name, ...); all must pass through.
func TestExtraStartupParametersAreForwarded(t *testing.T) {
	addr, got := startProxy(t, nil)
	c, _ := net.Dial("tcp", addr)
	defer c.Close()
	writeStartup(t, c, map[string]string{
		"user":             "alice.db1.pool",
		"database":         "app",
		"application_name": "svc",
		"options":          "--cluster=tenant-1",
		"client_encoding":  "UTF8",
	})
	select {
	case st := <-got:
		want := map[string]string{
			"user": "alice", "database": "app", "application_name": "svc",
			"options": "--cluster=tenant-1", "client_encoding": "UTF8",
		}
		for k, v := range want {
			if st.Parameters[k] != v {
				t.Errorf("%s = %q, want %q", k, st.Parameters[k], v)
			}
		}
	case <-time.After(2 * time.Second):
		t.Fatal("backend never received startup")
	}
}
