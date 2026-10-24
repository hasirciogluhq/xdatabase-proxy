package proxy

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

type echo struct{ active atomic.Int32 }

func (e *echo) Handle(_ context.Context, c net.Conn) {
	e.active.Add(1)
	defer e.active.Add(-1)
	defer c.Close()
	io.Copy(c, c)
}

var log = slog.New(slog.DiscardHandler)

func serve(t *testing.T, s *Server) (addr string, stop func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Serve(ctx, ln); close(done) }()
	return ln.Addr().String(), func() { cancel(); <-done }
}

func TestMaxConnsRejectsExtraConnections(t *testing.T) {
	h := &echo{}
	addr, stop := serve(t, &Server{Handler: h, MaxConns: 1, ShutdownTimeout: time.Second, Log: log})
	defer stop()

	first, _ := net.Dial("tcp", addr)
	defer first.Close()
	first.Write([]byte("x"))
	first.Read(make([]byte, 1)) // the first connection is being served

	second, _ := net.Dial("tcp", addr)
	defer second.Close()
	second.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := second.Read(make([]byte, 1)); err == nil {
		t.Fatal("second connection should be closed by the limit")
	}
}

func TestShutdownClosesRemainingConnectionsAfterTimeout(t *testing.T) {
	h := &echo{}
	addr, stop := serve(t, &Server{Handler: h, ShutdownTimeout: 100 * time.Millisecond, Log: log})
	c, _ := net.Dial("tcp", addr)
	defer c.Close()
	c.Write([]byte("x"))
	c.Read(make([]byte, 1))

	finished := make(chan struct{})
	go func() { stop(); close(finished) }()
	select {
	case <-finished:
	case <-time.After(3 * time.Second):
		t.Fatal("Serve did not return after the shutdown timeout")
	}
	if h.active.Load() != 0 {
		t.Fatal("handler still running")
	}
}

// flaky fails Accept a few times with a temporary error, then behaves normally.
type flaky struct {
	net.Listener
	fails atomic.Int32
}

func (f *flaky) Accept() (net.Conn, error) {
	if f.fails.Add(-1) >= 0 {
		return nil, errors.New("accept: too many open files")
	}
	return f.Listener.Accept()
}

func TestAcceptErrorsDoNotStopTheServer(t *testing.T) {
	inner, _ := net.Listen("tcp", "127.0.0.1:0")
	f := &flaky{Listener: inner}
	f.fails.Store(3)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	s := &Server{Handler: &echo{}, ShutdownTimeout: time.Second, Log: log}
	go func() { done <- s.Serve(ctx, f) }()

	c, err := net.Dial("tcp", inner.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(2 * time.Second))
	c.Write([]byte("ok"))
	buf := make([]byte, 2)
	if _, err := io.ReadFull(c, buf); err != nil || string(buf) != "ok" {
		t.Fatalf("server stopped serving after accept errors: %q %v", buf, err)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Serve returned %v on shutdown", err)
	}
}
