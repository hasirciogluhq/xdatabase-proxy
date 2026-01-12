package api

import (
	"context"
	"net/http"
	"sync/atomic"

	"github.com/hasirciogluhq/xdatabase-proxy/cmd/proxy/internal/logger"
)

type HealthServer struct {
	server *http.Server
	ready  atomic.Bool
}

func NewHealthServer(addr string) *HealthServer {
	mux := http.NewServeMux()
	hs := &HealthServer{
		server: &http.Server{
			Addr:    addr,
			Handler: mux,
		},
	}

	// Default to not ready until explicitly set
	hs.ready.Store(false)

	mux.HandleFunc("/health", hs.handleHealth)
	mux.HandleFunc("/ready", hs.handleReady)

	return hs
}

func (s *HealthServer) Start() {
	go func() {
		logger.Info("Health server listening", "addr", s.server.Addr)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Health server error", "error", err)
		}
	}()
}

func (s *HealthServer) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func (s *HealthServer) SetReady(ready bool) {
	s.ready.Store(ready)
}

func (s *HealthServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (s *HealthServer) handleReady(w http.ResponseWriter, r *http.Request) {
	if s.ready.Load() {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("not ready"))
	}
}
