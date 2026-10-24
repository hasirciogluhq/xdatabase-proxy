// Package gateway routes connections of databases other than PostgreSQL.
//
// A TLS connection is first matched by its server name (SNI). If the operator
// enabled that route it is passed through untouched and the backend terminates
// TLS. Otherwise, when the protocol has a Login reader and the proxy has a
// certificate, the proxy terminates TLS itself and routes by the username in
// the first client message. Plaintext connections are routed by username only.
package gateway

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log/slog"
	"net"
	"time"

	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/route"
	"github.com/hasirciogluhq/xdatabase-proxy/pkg/proxy"
)

const (
	helloTimeout = 10 * time.Second
	bufSize      = 20 << 10 // holds the largest TLS record (16 KiB) plus its header
	maxRecord    = 16<<10 + 256
)

var errStop = errors.New("sni captured")

// Login reads the first client message of a protocol. It returns the login
// user and a function that rebuilds that message for the backend with the
// routing suffix removed from the user.
type Login interface {
	Read(r *bufio.Reader) (user string, forward func(plain string) []byte, err error)
}

// Handler implements proxy.Handler.
type Handler struct {
	Resolver route.Resolver
	// Login enables username routing. Nil means SNI routing only.
	Login Login
	// TLS returns the configuration used to terminate client TLS for username
	// routing. Nil means the proxy never terminates TLS.
	TLS         func() *tls.Config
	DialTimeout time.Duration
	Log         *slog.Logger
}

func (h *Handler) Handle(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	log := h.Log.With("remote", conn.RemoteAddr().String())
	conn.SetDeadline(time.Now().Add(helloTimeout))

	br := bufio.NewReaderSize(conn, bufSize)
	client := &bufConn{Conn: conn, r: br}
	hdr, err := br.Peek(5)
	if err != nil {
		return
	}

	if isTLS(hdr) {
		n := 5 + (int(hdr[3])<<8 | int(hdr[4]))
		if n > maxRecord+5 {
			return
		}
		hello, err := br.Peek(n)
		if err != nil {
			return
		}
		if host := serverName(hello); host != "" {
			key := route.ParseHost(host)
			addr, err := h.Resolver.Resolve(ctx, key, route.SNI)
			if err == nil {
				h.passthrough(ctx, client, addr, log.With("via", route.SNI, "deployment", key.Deployment, "pooled", key.Pooled))
				return
			}
			log.Debug("sni route unavailable", "host", host, "error", err)
		}
		if h.Login == nil || h.TLS == nil {
			return
		}
		tc := tls.Server(client, h.TLS())
		if err := tc.Handshake(); err != nil {
			log.Debug("tls handshake failed", "error", err)
			return
		}
		client = &bufConn{Conn: tc, r: bufio.NewReader(tc)}
	} else if h.Login == nil {
		log.Debug("plaintext connection but protocol is routed by SNI only")
		return
	}

	user, forward, err := h.Login.Read(client.r)
	if err != nil {
		log.Debug("login read failed", "error", err)
		h.fail(client, "unsupported first message: "+err.Error())
		return
	}
	name, key := route.ParseLogin(user)
	log = log.With("via", route.Username, "deployment", key.Deployment, "pooled", key.Pooled)
	if key.Deployment == "" {
		h.fail(client, "no backend for this user; expected user.deployment[.pool]")
		return
	}
	rctx, cancel := context.WithTimeout(ctx, h.DialTimeout)
	defer cancel()
	addr, err := h.Resolver.Resolve(rctx, key, route.Username)
	if err != nil {
		log.Warn("resolve failed", "error", err)
		h.fail(client, "no backend for this user; expected user.deployment[.pool]")
		return
	}
	backend, err := (&net.Dialer{}).DialContext(rctx, "tcp", addr)
	if err != nil {
		log.Error("dial failed", "backend", addr, "error", err)
		h.fail(client, "backend unavailable")
		return
	}
	defer backend.Close()
	if _, err := backend.Write(forward(name)); err != nil {
		log.Error("forward login failed", "error", err)
		return
	}
	conn.SetDeadline(time.Time{})
	log.Debug("session started", "backend", addr)
	proxy.Pipe(client, backend)
}

// passthrough forwards the buffered ClientHello and then all bytes unchanged.
func (h *Handler) passthrough(ctx context.Context, client *bufConn, addr string, log *slog.Logger) {
	rctx, cancel := context.WithTimeout(ctx, h.DialTimeout)
	defer cancel()
	backend, err := (&net.Dialer{}).DialContext(rctx, "tcp", addr)
	if err != nil {
		log.Error("dial failed", "backend", addr, "error", err)
		return
	}
	defer backend.Close()
	buffered, _ := client.r.Peek(client.r.Buffered())
	if _, err := backend.Write(buffered); err != nil {
		log.Error("forward client hello failed", "error", err)
		return
	}
	client.r.Discard(len(buffered))
	client.Conn.SetDeadline(time.Time{})
	log.Debug("session started", "backend", addr)
	proxy.Pipe(client, backend)
}

func (h *Handler) fail(w io.Writer, msg string) {
	if e, ok := h.Login.(interface{ WriteError(io.Writer, string) }); ok {
		e.WriteError(w, msg)
	}
}

// isTLS recognises a TLS handshake record header.
func isTLS(hdr []byte) bool { return hdr[0] == 0x16 && hdr[1] == 3 && hdr[2] <= 4 }

// serverName extracts the SNI from a buffered ClientHello using crypto/tls
// itself: the handshake is aborted as soon as the hello is parsed.
func serverName(hello []byte) string {
	var name string
	tls.Server(replayConn{bytes.NewReader(hello)}, &tls.Config{
		GetConfigForClient: func(hi *tls.ClientHelloInfo) (*tls.Config, error) {
			name = hi.ServerName
			return nil, errStop
		},
	}).Handshake()
	return name
}

// bufConn reads through a bufio.Reader so peeked bytes are not lost.
type bufConn struct {
	net.Conn
	r *bufio.Reader
}

func (c *bufConn) Read(p []byte) (int, error) { return c.r.Read(p) }

// replayConn serves fixed bytes and swallows writes (no alert reaches the client).
type replayConn struct{ r io.Reader }

func (c replayConn) Read(p []byte) (int, error)       { return c.r.Read(p) }
func (c replayConn) Write(p []byte) (int, error)      { return len(p), nil }
func (c replayConn) Close() error                     { return nil }
func (c replayConn) LocalAddr() net.Addr              { return &net.TCPAddr{} }
func (c replayConn) RemoteAddr() net.Addr             { return &net.TCPAddr{} }
func (c replayConn) SetDeadline(time.Time) error      { return nil }
func (c replayConn) SetReadDeadline(time.Time) error  { return nil }
func (c replayConn) SetWriteDeadline(time.Time) error { return nil }
