package balancer

import (
	"errors"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
)

type Backend struct {
	Name       string
	Target     *url.URL
	Weight     int
	Hostnames  map[string]struct{}
	Healthy    atomic.Bool
	InFlight   atomic.Int64
	TotalReq   atomic.Uint64
	TotalError atomic.Uint64
}

type Balancer struct {
	mu      sync.RWMutex
	ring    []*Backend
	counter atomic.Uint64
}

func New(backends []*Backend) (*Balancer, error) {
	if len(backends) == 0 {
		return nil, errors.New("no backends provided")
	}
	b := &Balancer{}
	b.SetBackends(backends)
	return b, nil
}

func (b *Balancer) SetBackends(backends []*Backend) {
	ring := make([]*Backend, 0, len(backends)*4)
	for _, be := range backends {
		if be.Weight <= 0 {
			be.Weight = 1
		}
		be.Healthy.Store(true)
		for i := 0; i < be.Weight; i++ {
			ring = append(ring, be)
		}
	}
	b.mu.Lock()
	b.ring = ring
	b.mu.Unlock()
}

func (b *Balancer) Next(host string) *Backend {
	host = normalizeHost(host)

	b.mu.RLock()
	ring := b.ring
	b.mu.RUnlock()
	if len(ring) == 0 {
		return nil
	}

	start := int(b.counter.Add(1) % uint64(len(ring)))
	for i := 0; i < len(ring); i++ {
		idx := (start + i) % len(ring)
		candidate := ring[idx]
		if !candidate.Healthy.Load() {
			continue
		}
		if len(candidate.Hostnames) > 0 {
			if _, ok := candidate.Hostnames["*"]; !ok {
				if _, ok := candidate.Hostnames[host]; !ok {
					continue
				}
			}
		}
		return candidate
	}
	return nil
}

func normalizeHost(host string) string {
	host = strings.ToLower(host)
	if i := strings.IndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	return strings.TrimSpace(host)
}
