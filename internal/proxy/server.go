package proxy

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"sync"
	"time"

	"egn-proxy-core/internal/balancer"
	"egn-proxy-core/internal/ddos"
	"egn-proxy-core/internal/metrics"
)

type Server struct {
	Balancer    *balancer.Balancer
	DDoS        *ddos.Guard
	Metrics     *metrics.Store
	MaxInflight int64
	Logger      *slog.Logger

	mu      sync.RWMutex
	reverse map[*balancer.Backend]*httputil.ReverseProxy
}

func NewServer(b *balancer.Balancer, g *ddos.Guard, m *metrics.Store, maxInflight int64, logger *slog.Logger, backends []*balancer.Backend) *Server {
	rev := map[*balancer.Backend]*httputil.ReverseProxy{}
	for _, be := range backends {
		target := be.Target
		rp := httputil.NewSingleHostReverseProxy(target)
		rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte("upstream unavailable"))
		}
		rev[be] = rp
	}
	return &Server{Balancer: b, DDoS: g, Metrics: m, MaxInflight: maxInflight, Logger: logger, reverse: rev}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/ready", s.ready)
	mux.HandleFunc("/live", s.live)
	mux.Handle("/", s)
	return mux
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.Metrics.Inflight() >= s.MaxInflight {
		http.Error(w, "busy", http.StatusServiceUnavailable)
		return
	}

	if !s.DDoS.Allow(r) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
		return
	}

	be := s.Balancer.Next(r.Host)
	if be == nil {
		http.Error(w, "no healthy backend", http.StatusServiceUnavailable)
		return
	}

	s.Metrics.AddInflight(1)
	be.InFlight.Add(1)
	be.TotalReq.Add(1)
	defer s.Metrics.AddInflight(-1)
	defer be.InFlight.Add(-1)

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	r = r.WithContext(ctx)

	s.mu.RLock()
	rp := s.reverse[be]
	s.mu.RUnlock()
	if rp == nil {
		be.TotalError.Add(1)
		http.Error(w, "proxy not found", http.StatusInternalServerError)
		return
	}
	rp.ServeHTTP(w, r)
}

func (s *Server) ready(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) live(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("alive"))
}
