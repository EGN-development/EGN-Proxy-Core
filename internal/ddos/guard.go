package ddos

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

type Guard struct {
	trustedProxyHeader string
	globalLimiter      *rate.Limiter
	perIPRate          int
	perIPBurst         int
	banThreshold       int64
	banDuration        time.Duration

	ips sync.Map // map[string]*ipState

	allowed uint64
	blocked uint64
}

type ipState struct {
	limiter         *rate.Limiter
	lastSeenUnixSec atomic.Int64
	windowStartUnix atomic.Int64
	windowCount     atomic.Int64
	bannedUntilUnix atomic.Int64
}

func NewGuard(trustedProxyHeader string, globalRPS, globalBurst, perIPRPS, perIPBurst int, banThresholdPerMin int, banDuration time.Duration) *Guard {
	return &Guard{
		trustedProxyHeader: trustedProxyHeader,
		globalLimiter:      rate.NewLimiter(rate.Limit(globalRPS), globalBurst),
		perIPRate:          perIPRPS,
		perIPBurst:         perIPBurst,
		banThreshold:       int64(banThresholdPerMin),
		banDuration:        banDuration,
	}
}

func (g *Guard) Allow(r *http.Request) bool {
	ip := g.extractIP(r)
	now := time.Now()
	unix := now.Unix()

	v, _ := g.ips.LoadOrStore(ip, &ipState{limiter: rate.NewLimiter(rate.Limit(g.perIPRate), g.perIPBurst)})
	st := v.(*ipState)
	st.lastSeenUnixSec.Store(unix)

	if bannedUntil := st.bannedUntilUnix.Load(); bannedUntil > unix {
		atomic.AddUint64(&g.blocked, 1)
		return false
	}

	winStart := st.windowStartUnix.Load()
	if unix-winStart >= 60 {
		st.windowStartUnix.Store(unix)
		st.windowCount.Store(0)
	}

	if !g.globalLimiter.Allow() || !st.limiter.Allow() {
		cnt := st.windowCount.Add(1)
		if cnt >= g.banThreshold {
			st.bannedUntilUnix.Store(unix + int64(g.banDuration.Seconds()))
		}
		atomic.AddUint64(&g.blocked, 1)
		return false
	}

	atomic.AddUint64(&g.allowed, 1)
	return true
}

func (g *Guard) Cleanup(ttl, every time.Duration, stop <-chan struct{}) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			now := time.Now().Unix()
			g.ips.Range(func(key, value any) bool {
				st := value.(*ipState)
				if now-st.lastSeenUnixSec.Load() >= int64(ttl.Seconds()) {
					g.ips.Delete(key)
				}
				return true
			})
		}
	}
}

func (g *Guard) Stats() (allowed, blocked uint64) {
	return atomic.LoadUint64(&g.allowed), atomic.LoadUint64(&g.blocked)
}

func (g *Guard) extractIP(r *http.Request) string {
	if g.trustedProxyHeader != "" {
		xff := r.Header.Get(g.trustedProxyHeader)
		if xff != "" {
			parts := strings.Split(xff, ",")
			return strings.TrimSpace(parts[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}
