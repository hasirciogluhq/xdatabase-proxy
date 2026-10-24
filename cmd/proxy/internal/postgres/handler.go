// Package postgres routes PostgreSQL client connections to a backend chosen
// from the username: "user.deployment[.pool]".
package postgres

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"time"

	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/route"
	"github.com/hasirciogluhq/xdatabase-proxy/pkg/proxy"
	"github.com/jackc/pgx/v5/pgproto3"
)

const startupTimeout = 10 * time.Second

// Handler implements proxy.Handler for PostgreSQL.
type Handler struct {
	// TLS is nil when TLS is disabled; SSLRequests are then declined.
	TLS         func() *tls.Config
	Resolver    route.Resolver
	DialTimeout time.Duration
	Log         *slog.Logger
}

func (h *Handler) Handle(ctx context.Context, client net.Conn) {
	defer client.Close()
	log := h.Log.With("remote", client.RemoteAddr().String())

	client.SetDeadline(time.Now().Add(startupTimeout))
	client, st, serverName, err := h.startup(client)
	if err != nil {
		log.Debug("startup failed", "error", err)
		return
	}
	client.SetDeadline(time.Time{})

	name, key := route.ParseLogin(st.Parameters["user"])
	via := route.Username
	if key.Deployment == "" && serverName != "" {
		// No deployment in the login: use the TLS server name (db1[.pool].example.com).
		// The resolver only accepts this when the Service label enables sni.
		key, via = route.ParseHost(serverName), route.SNI
	}
	log = log.With("deployment", key.Deployment, "pooled", key.Pooled, "via", via)

	rctx, cancel := context.WithTimeout(ctx, h.DialTimeout)
	defer cancel()
	addr, err := h.Resolver.Resolve(rctx, key, via)
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

// bufConn replays bytes peeked from the connection.
type bufConn struct {
	net.Conn
	r *bufio.Reader
}

func (c *bufConn) Read(p []byte) (int, error) { return c.r.Read(p) }

// startup negotiates SSL/GSS requests (and PostgreSQL 17 direct TLS) and
// returns the StartupMessage plus the TLS server name, if any.
func (h *Handler) startup(raw net.Conn) (conn net.Conn, st *pgproto3.StartupMessage, serverName string, err error) {
	var cfg *tls.Config
	if h.TLS != nil {
		cfg = h.TLS()
	}
	br := bufio.NewReader(raw)
	conn = &bufConn{raw, br}
	encrypted := false

	// A TLS record header (0x16) as first byte is a direct TLS connection (sslnegotiation=direct).
	if first, perr := br.Peek(1); perr != nil {
		return nil, nil, "", perr
	} else if first[0] == 0x16 {
		if cfg == nil {
			return nil, nil, "", errors.New("direct TLS requested but TLS is disabled")
		}
		direct := cfg.Clone()
		direct.NextProtos = []string{"postgresql"}
		tc := tls.Server(conn, direct)
		if err := tc.Handshake(); err != nil {
			return nil, nil, "", fmt.Errorf("tls handshake: %w", err)
		}
		conn, encrypted, serverName = tc, true, tc.ConnectionState().ServerName
	}

	for range 4 {
		msg, err := pgproto3.NewBackend(conn, conn).ReceiveStartupMessage()
		if err != nil {
			return nil, nil, "", err
		}
		switch m := msg.(type) {
		case *pgproto3.SSLRequest:
			if encrypted || cfg == nil {
				if _, err := conn.Write([]byte{'N'}); err != nil {
					return nil, nil, "", err
				}
				continue
			}
			if _, err := conn.Write([]byte{'S'}); err != nil {
				return nil, nil, "", err
			}
			tc := tls.Server(conn, cfg)
			if err := tc.Handshake(); err != nil {
				return nil, nil, "", fmt.Errorf("tls handshake: %w", err)
			}
			conn, encrypted, serverName = tc, true, tc.ConnectionState().ServerName
		case *pgproto3.GSSEncRequest:
			if _, err := conn.Write([]byte{'N'}); err != nil {
				return nil, nil, "", err
			}
		case *pgproto3.StartupMessage:
			return conn, m, serverName, nil
		default:
			return nil, nil, "", fmt.Errorf("unsupported startup request %T", msg)
		}
	}
	return nil, nil, "", errors.New("too many negotiation requests")
}

func writeError(w io.Writer, msg string) {
	buf, _ := (&pgproto3.ErrorResponse{Severity: "FATAL", Code: "08001", Message: msg}).Encode(nil)
	w.Write(buf)
}

// ParseUser splits "name.deployment[.pool]". Without a deployment part it
// returns an empty deployment, which no route matches.
func ParseUser(user string) (name, deployment string, pooled bool) {
	name, k := route.ParseLogin(user)
	return name, k.Deployment, k.Pooled
}
