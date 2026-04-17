package metrics

import (
	"encoding/json"
	"net/http"
	"runtime"
	"sync/atomic"
)

type Snapshot struct {
	AllowedRequests uint64 `json:"allowed_requests"`
	BlockedRequests uint64 `json:"blocked_requests"`
	InFlight        int64  `json:"in_flight"`
	Goroutines      int    `json:"goroutines"`
}

type Store struct {
	inFlight atomic.Int64
}

func (s *Store) AddInflight(n int64) { s.inFlight.Add(n) }

func (s *Store) Inflight() int64 { return s.inFlight.Load() }

func (s *Store) Handler(getAllowedBlocked func() (uint64, uint64)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		allowed, blocked := getAllowedBlocked()
		out := Snapshot{
			AllowedRequests: allowed,
			BlockedRequests: blocked,
			InFlight:        s.inFlight.Load(),
			Goroutines:      runtime.NumGoroutine(),
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	}
}
