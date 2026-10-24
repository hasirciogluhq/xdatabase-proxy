// Package postgres routes PostgreSQL client connections to a backend chosen
// from the username: "user.deployment[.pool]".
package postgres

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/hasirciogluhq/xdatabase-proxy/pkg/proxy"
	"github.com/jackc/pgx/v5/pgproto3"
)

const startupTimeout = 10 * time.Second

// Resolver finds the backend address for a deployment.
type Resolver interface {
	Resolve(ctx context.Context, deployment string, pooled bool) (string, error)
}

// Handler implements proxy.Handler for PostgreSQL.
type Handler struct {
	// TLS is nil when TLS is disabled; SSLRequests are then declined.
	TLS         func() *tls.Config
	Resolver    Resolver
	DialTimeout time.Duration
	Log         *slog.Logger
}

func (h *Handler) Handle(ctx context.Context, client net.Conn) {
	defer client.Close()
	log := h.Log.With("remote", client.RemoteAddr().String())

	client.SetDeadline(time.Now().Add(startupTimeout))
	client, st, err := h.startup(client)
	if err != nil {
		log.Debug("startup failed", "error", err)
		return
	}
	client.SetDeadline(time.Time{})

	name, deployment, pooled := ParseUser(st.Parameters["user"])
	log = log.With("deployment", deployment, "pooled", pooled)

	rctx, cancel := context.WithTimeout(ctx, h.DialTimeout)
	defer cancel()
	addr, err := h.Resolver.Resolve(rctx, deployment, pooled)
	if err != nil {
		log.Warn("resolve failed", "error", err)
		writeError(client, "no backend for this user; expected user.deployment[.pool]")
		return
	}
	backend, err := (&net.Dialer{}).DialContext(rctx, "tcp", addr)
	if err != nil {
		log.Error("dial failed", "backend", addr, "error", err)
		writeError(client, "backend unavailable")
		return
	}
	defer backend.Close()

	// psql defaults the database to the full "user.deployment" login; use "postgres" instead.
	if db := st.Parameters["database"]; db == "" || db == st.Parameters["user"] {
		st.Parameters["database"] = "postgres"
	}
	st.Parameters["user"] = name
	startup, err := st.Encode(nil)
	if err == nil {
		_, err = backend.Write(startup)
	}
	if err != nil {
		log.Error("forward startup failed", "error", err)
		return
	}
	log.Debug("session started", "backend", addr)
	proxy.Pipe(client, backend)
}

// startup negotiates SSL/GSS requests and returns the StartupMessage.
func (h *Handler) startup(conn net.Conn) (net.Conn, *pgproto3.StartupMessage, error) {
	encrypted := false
	for range 4 {
		msg, err := pgproto3.NewBackend(conn, conn).ReceiveStartupMessage()
		if err != nil {
			return nil, nil, err
		}
		switch m := msg.(type) {
		case *pgproto3.SSLRequest:
			var cfg *tls.Config
			if h.TLS != nil {
				cfg = h.TLS()
			}
			if encrypted || cfg == nil {
				if _, err := conn.Write([]byte{'N'}); err != nil {
					return nil, nil, err
				}
				continue
			}
			if _, err := conn.Write([]byte{'S'}); err != nil {
				return nil, nil, err
			}
			tc := tls.Server(conn, cfg)
			if err := tc.Handshake(); err != nil {
				return nil, nil, fmt.Errorf("tls handshake: %w", err)
			}
			conn, encrypted = tc, true
		case *pgproto3.GSSEncRequest:
			if _, err := conn.Write([]byte{'N'}); err != nil {
				return nil, nil, err
			}
		case *pgproto3.StartupMessage:
			return conn, m, nil
		default:
			return nil, nil, fmt.Errorf("unsupported startup request %T", msg)
		}
	}
	return nil, nil, errors.New("too many negotiation requests")
}

func writeError(w io.Writer, msg string) {
	buf, _ := (&pgproto3.ErrorResponse{Severity: "FATAL", Code: "08001", Message: msg}).Encode(nil)
	w.Write(buf)
}

// ParseUser splits "name.deployment[.pool]". Without a deployment part it
// returns an empty deployment, which no route matches.
func ParseUser(user string) (name, deployment string, pooled bool) {
	parts := strings.Split(user, ".")
	if n := len(parts); n >= 3 && parts[n-1] == "pool" {
		return strings.Join(parts[:n-2], "."), parts[n-2], true
	} else if n >= 2 && parts[n-1] != "pool" {
		return strings.Join(parts[:n-1], "."), parts[n-1], false
	}
	return user, "", false
}
