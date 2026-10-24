// Package proxy provides a protocol-agnostic TCP server with a connection
// limit and graceful shutdown, plus a bidirectional pipe.
package proxy

import (
	"context"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"
)

// Handler owns a connection for its whole lifetime and must close it.
type Handler interface {
	Handle(ctx context.Context, conn net.Conn)
}

// Server accepts connections and hands each to Handler in its own goroutine.
type Server struct {
	Handler Handler
	// MaxConns limits concurrent connections; 0 means unlimited.
	MaxConns int
	// ShutdownTimeout is how long Serve waits for sessions to finish after ctx
	// is cancelled before closing them forcibly.
	ShutdownTimeout time.Duration
	Log             *slog.Logger

	mu    sync.Mutex
	conns map[net.Conn]struct{}
}

// Serve blocks until ctx is cancelled (returns nil) or Accept fails.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	s.conns = map[net.Conn]struct{}{}
	var wg sync.WaitGroup
	var slots chan struct{}
	if s.MaxConns > 0 {
		slots = make(chan struct{}, s.MaxConns)
	}

	go func() { <-ctx.Done(); ln.Close() }()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return s.drain(&wg)
			}
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			return err
		}
		if slots != nil {
			select {
			case slots <- struct{}{}:
			default:
				s.Log.Warn("connection limit reached, rejecting", "remote", conn.RemoteAddr())
				conn.Close()
				continue
			}
		}
		s.track(conn, true)
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer s.track(conn, false)
			if slots != nil {
				defer func() { <-slots }()
			}
			s.Handler.Handle(ctx, conn)
		}()
	}
}

func (s *Server) track(c net.Conn, add bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if add {
		s.conns[c] = struct{}{}
	} else {
		delete(s.conns, c)
	}
}

func (s *Server) drain(wg *sync.WaitGroup) error {
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(s.ShutdownTimeout):
		s.Log.Warn("shutdown timeout, closing remaining connections")
		s.mu.Lock()
		for c := range s.conns {
			c.Close()
		}
		s.mu.Unlock()
		<-done
	}
	return nil
}

// Pipe copies data both ways until either side finishes, then closes both.
func Pipe(a, b net.Conn) {
	done := make(chan struct{}, 2)
	cp := func(dst, src net.Conn) { io.Copy(dst, src); done <- struct{}{} }
	go cp(a, b)
	go cp(b, a)
	<-done
	a.Close()
	b.Close()
	<-done
}
