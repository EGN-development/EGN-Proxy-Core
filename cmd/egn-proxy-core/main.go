package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"egn-proxy-core/internal/balancer"
	"egn-proxy-core/internal/config"
	"egn-proxy-core/internal/ddos"
	"egn-proxy-core/internal/health"
	"egn-proxy-core/internal/metrics"
	"egn-proxy-core/internal/proxy"
)

func main() {
	cfgFile := flag.String("config", "", "path to JSON config")
	flag.Parse()

	cfg, err := config.Load(*cfgFile)
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	backends := make([]*balancer.Backend, 0, len(cfg.Backends))
	for _, b := range cfg.Backends {
		target, err := url.Parse(b.URL)
		if err != nil {
			logger.Error("invalid backend url", "name", b.Name, "url", b.URL, "error", err)
			os.Exit(1)
		}
		hosts := map[string]struct{}{}
		for _, h := range b.Hostnames {
			hosts[strings.ToLower(h)] = struct{}{}
		}
		backends = append(backends, &balancer.Backend{
			Name:      b.Name,
			Target:    target,
			Weight:    b.Weight,
			Hostnames: hosts,
		})
	}

	lb, err := balancer.New(backends)
	if err != nil {
		logger.Error("init balancer", "error", err)
		os.Exit(1)
	}

	guard := ddos.NewGuard(
		cfg.TrustedProxyHeader,
		cfg.DDoS.GlobalRPSLimit,
		cfg.DDoS.GlobalBurst,
		cfg.DDoS.PerIPRPSLimit,
		cfg.DDoS.PerIPBurst,
		cfg.DDoS.BanThresholdPerMinute,
		cfg.DDoS.BanDuration,
	)
	m := &metrics.Store{}
	ps := proxy.NewServer(lb, guard, m, int64(cfg.MaxInflight), logger, backends)

	root := http.NewServeMux()
	root.Handle("/", ps.Handler())
	root.Handle("/metrics", m.Handler(guard.Stats))

	srv := &http.Server{
		Addr:           cfg.ListenAddr,
		Handler:        root,
		ReadTimeout:    cfg.ReadTimeout,
		WriteTimeout:   cfg.WriteTimeout,
		IdleTimeout:    cfg.IdleTimeout,
		MaxHeaderBytes: cfg.MaxHeaderBytes,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go (&health.Checker{
		Backends: backends,
		Path:     cfg.Health.Path,
		Interval: cfg.Health.CheckInterval,
		Timeout:  cfg.Health.Timeout,
	}).Start(ctx)

	stopDDoS := make(chan struct{})
	go guard.Cleanup(10*time.Minute, cfg.DDoS.CleanupInterval, stopDDoS)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Info("proxy started", "addr", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("listen failed", "error", err)
			os.Exit(1)
		}
	}()

	<-sigCh
	logger.Info("shutdown requested")
	close(stopDDoS)
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("graceful shutdown failed", "error", err)
	}
}
