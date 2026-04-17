package health

import (
	"context"
	"io"
	"net/http"
	"time"

	"egn-proxy-core/internal/balancer"
)

type Checker struct {
	Backends []*balancer.Backend
	Path     string
	Interval time.Duration
	Timeout  time.Duration
	Client   *http.Client
}

func (c *Checker) Start(ctx context.Context) {
	if c.Client == nil {
		c.Client = &http.Client{Timeout: c.Timeout}
	}
	ticker := time.NewTicker(c.Interval)
	defer ticker.Stop()

	c.checkAll()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.checkAll()
		}
	}
}

func (c *Checker) checkAll() {
	for _, be := range c.Backends {
		url := be.Target.String() + c.Path
		ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		resp, err := c.Client.Do(req)
		cancel()
		if err != nil {
			be.Healthy.Store(false)
			continue
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		be.Healthy.Store(resp.StatusCode >= 200 && resp.StatusCode <= 399)
	}
}
