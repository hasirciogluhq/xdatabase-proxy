// Package sni routes TLS connections of any protocol by the server name in the
// ClientHello ("deployment[.pool].domain") and forwards the bytes untouched.
// The backend terminates TLS, so the proxy needs no certificate.
package sni

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/hasirciogluhq/xdatabase-proxy/pkg/proxy"
)

const helloTimeout = 10 * time.Second

var errStop = errors.New("sni captured")

// Resolver finds the backend address for a deployment.
type Resolver interface {
	Resolve(ctx context.Context, deployment string, pooled bool) (string, error)
}

// Handler implements proxy.Handler.
type Handler struct {
	Resolver    Resolver
	DialTimeout time.Duration
	Log         *slog.Logger
}

func (h *Handler) Handle(ctx context.Context, client net.Conn) {
	defer client.Close()
	log := h.Log.With("remote", client.RemoteAddr().String())

	client.SetDeadline(time.Now().Add(helloTimeout))
	rec := &recorder{Conn: client}
	var host string
	// The handshake only runs until the ClientHello was parsed, then we abort it.
	tls.Server(rec, &tls.Config{GetConfigForClient: func(hi *tls.ClientHelloInfo) (*tls.Config, error) {
		host = hi.ServerName
		return nil, errStop
	}}).Handshake()
	if host == "" {
		log.Debug("no SNI in client hello")
		return
	}
	client.SetDeadline(time.Time{})

	deployment, pooled := ParseHost(host)
	log = log.With("deployment", deployment, "pooled", pooled)

	rctx, cancel := context.WithTimeout(ctx, h.DialTimeout)
	defer cancel()
	addr, err := h.Resolver.Resolve(rctx, deployment, pooled)
	if err != nil {
		log.Warn("resolve failed", "error", err)
		return
	}
	backend, err := (&net.Dialer{}).DialContext(rctx, "tcp", addr)
	if err != nil {
		log.Error("dial failed", "backend", addr, "error", err)
		return
	}
	defer backend.Close()

	if _, err := backend.Write(rec.buf); err != nil {
		log.Error("forward client hello failed", "error", err)
		return
	}
	log.Debug("session started", "backend", addr)
	proxy.Pipe(client, backend)
}

// ParseHost maps "db1.example.com" to ("db1", false) and "db1.pool.example.com" to ("db1", true).
func ParseHost(host string) (deployment string, pooled bool) {
	labels := strings.Split(strings.ToLower(strings.TrimSuffix(host, ".")), ".")
	return labels[0], len(labels) > 2 && labels[1] == "pool"
}

// recorder remembers everything read and swallows writes, so the aborted
// handshake sends no alert to the client and the hello can be replayed.
type recorder struct {
	net.Conn
	buf []byte
}

func (r *recorder) Read(p []byte) (int, error) {
	n, err := r.Conn.Read(p)
	r.buf = append(r.buf, p[:n]...)
	return n, err
}

func (r *recorder) Write(p []byte) (int, error) { return len(p), nil }
