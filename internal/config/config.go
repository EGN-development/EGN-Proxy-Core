package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type Config struct {
	ListenAddr         string        `json:"listen_addr"`
	ReadTimeout        time.Duration `json:"read_timeout"`
	WriteTimeout       time.Duration `json:"write_timeout"`
	IdleTimeout        time.Duration `json:"idle_timeout"`
	MaxHeaderBytes     int           `json:"max_header_bytes"`
	MaxInflight        int           `json:"max_inflight"`
	TrustedProxyHeader string        `json:"trusted_proxy_header"`
	Backends           []Backend     `json:"backends"`
	DDoS               DDoSConfig    `json:"ddos"`
	Health             HealthConfig  `json:"health"`
}

type Backend struct {
	Name      string   `json:"name"`
	URL       string   `json:"url"`
	Weight    int      `json:"weight"`
	Hostnames []string `json:"hostnames"`
}

type DDoSConfig struct {
	GlobalRPSLimit        int           `json:"global_rps_limit"`
	PerIPRPSLimit         int           `json:"per_ip_rps_limit"`
	PerIPBurst            int           `json:"per_ip_burst"`
	GlobalBurst           int           `json:"global_burst"`
	BanThresholdPerMinute int           `json:"ban_threshold_per_minute"`
	BanDuration           time.Duration `json:"ban_duration"`
	CleanupInterval       time.Duration `json:"cleanup_interval"`
}

type HealthConfig struct {
	CheckInterval time.Duration `json:"check_interval"`
	Path          string        `json:"path"`
	Timeout       time.Duration `json:"timeout"`
}

func Default() Config {
	return Config{
		ListenAddr:         ":8080",
		ReadTimeout:        5 * time.Second,
		WriteTimeout:       15 * time.Second,
		IdleTimeout:        60 * time.Second,
		MaxHeaderBytes:     1 << 20,
		MaxInflight:        50000,
		TrustedProxyHeader: "X-Forwarded-For",
		Backends: []Backend{
			{Name: "app-1", URL: "http://127.0.0.1:9001", Weight: 3, Hostnames: []string{"*"}},
			{Name: "app-2", URL: "http://127.0.0.1:9002", Weight: 2, Hostnames: []string{"*"}},
		},
		DDoS: DDoSConfig{
			GlobalRPSLimit:        75000,
			PerIPRPSLimit:         200,
			PerIPBurst:            500,
			GlobalBurst:           12000,
			BanThresholdPerMinute: 6000,
			BanDuration:           15 * time.Minute,
			CleanupInterval:       30 * time.Second,
		},
		Health: HealthConfig{
			CheckInterval: 2 * time.Second,
			Path:          "/health",
			Timeout:       1200 * time.Millisecond,
		},
	}
}

func Load(path string) (Config, error) {
	if path == "" {
		return Default(), nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	cfg := Default()
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config json: %w", err)
	}
	if len(cfg.Backends) == 0 {
		return Config{}, fmt.Errorf("backends cannot be empty")
	}
	for i := range cfg.Backends {
		if cfg.Backends[i].Weight <= 0 {
			cfg.Backends[i].Weight = 1
		}
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if c.MaxInflight <= 0 {
		return fmt.Errorf("max_inflight must be > 0")
	}
	if c.DDoS.GlobalRPSLimit <= 0 || c.DDoS.PerIPRPSLimit <= 0 {
		return fmt.Errorf("ddos limits must be > 0")
	}
	if c.DDoS.GlobalBurst <= 0 || c.DDoS.PerIPBurst <= 0 {
		return fmt.Errorf("ddos burst values must be > 0")
	}
	if c.Health.CheckInterval <= 0 || c.Health.Timeout <= 0 {
		return fmt.Errorf("health timings must be > 0")
	}
	for _, b := range c.Backends {
		if b.Name == "" || b.URL == "" {
			return fmt.Errorf("backend name and url are required")
		}
	}
	return nil
}
