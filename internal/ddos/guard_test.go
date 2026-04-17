package ddos

import (
	"net/http"
	"testing"
	"time"
)

func TestAllowAndBlock(t *testing.T) {
	g := NewGuard("", 1000, 1000, 3, 3, 100, 5*time.Second)
	req, _ := http.NewRequest(http.MethodGet, "http://x", nil)
	req.RemoteAddr = "10.0.0.1:1000"

	allowed := 0
	blocked := 0
	for i := 0; i < 10; i++ {
		if g.Allow(req) {
			allowed++
		} else {
			blocked++
		}
	}
	if allowed == 0 || blocked == 0 {
		t.Fatalf("expected both allowed and blocked, allowed=%d blocked=%d", allowed, blocked)
	}
}
