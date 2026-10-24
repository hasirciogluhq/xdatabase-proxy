// Package health serves liveness and readiness probes.
package health

import (
	"net/http"
	"sync/atomic"
	"time"
)

// Server answers /health (liveness) and /ready (readiness).
type Server struct {
	*http.Server
	ready atomic.Bool
}

func New(addr string) *Server {
	s := &Server{}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	mux.HandleFunc("/ready", func(w http.ResponseWriter, _ *http.Request) {
		if !s.ready.Load() {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ready"))
	})
	s.Server = &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	return s
}

func (s *Server) SetReady(v bool) { s.ready.Store(v) }
