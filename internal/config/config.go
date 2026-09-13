package config

import (
	"fmt"
	"math"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/mynameis-nigel/ssh-moonminer/internal/identity"
)

// Config holds server settings from MOONMINER_* environment variables.
type Config struct {
	ListenHost          string
	ListenPort          int
	HostKeyPath         string
	IdleTimeout         time.Duration
	MaxSessionsPerKey   int
	MaxConnections      int
	DefaultSlot         string
	LogLevel            string
	LogFormat           string
	RateLimitPerSecond  float64
	RateLimitBurst      int
	RateLimitMaxEntries int
	DBPath              string
	AutosaveInterval    time.Duration
	SessionPolicy       string
	DataDir             string
	ProxyKeysPath       string
	DevMode             bool
}

// Load reads configuration from the environment.
func Load() (Config, error) {
	var err error
	cfg := Config{
		ListenHost:    envOr("MOONMINER_LISTEN_HOST", "0.0.0.0"),
		HostKeyPath:   envOr("MOONMINER_HOST_KEY_PATH", "var/ssh_host_key"),
		DefaultSlot:   envOr("MOONMINER_DEFAULT_SLOT", "default"),
		LogLevel:      envOr("MOONMINER_LOG_LEVEL", "info"),
		LogFormat:     envOr("MOONMINER_LOG_FORMAT", "text"),
		DBPath:        envOr("MOONMINER_DB_PATH", "var/moonminer.db"),
		SessionPolicy: envOr("MOONMINER_SESSION_POLICY", "takeover"),
		DataDir:       os.Getenv("MOONMINER_DATA_DIR"),
		ProxyKeysPath: os.Getenv("MOONMINER_PROXY_KEYS_PATH"),
	}
	if raw := os.Getenv("MOONMINER_DEV_MODE"); raw != "" {
		cfg.DevMode, err = strconv.ParseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("MOONMINER_DEV_MODE: invalid boolean %q", raw)
		}
	}
	if cfg.ListenPort, err = envIntOr("MOONMINER_LISTEN_PORT", 22); err != nil {
		return Config{}, err
	}
	if cfg.IdleTimeout, err = envDurationOr("MOONMINER_IDLE_TIMEOUT", 30*time.Minute); err != nil {
		return Config{}, err
	}
	if cfg.MaxSessionsPerKey, err = envIntOr("MOONMINER_MAX_SESSIONS_PER_KEY", 2); err != nil {
		return Config{}, err
	}
	if cfg.MaxConnections, err = envIntOr("MOONMINER_MAX_CONNECTIONS", 100); err != nil {
		return Config{}, err
	}
	if cfg.RateLimitPerSecond, err = envFloatOr("MOONMINER_RATE_LIMIT_PER_SECOND", 2); err != nil {
		return Config{}, err
	}
	if cfg.RateLimitBurst, err = envIntOr("MOONMINER_RATE_LIMIT_BURST", 5); err != nil {
		return Config{}, err
	}
	if cfg.RateLimitMaxEntries, err = envIntOr("MOONMINER_RATE_LIMIT_MAX_IPS", 1000); err != nil {
		return Config{}, err
	}
	if cfg.AutosaveInterval, err = envDurationOr("MOONMINER_AUTOSAVE_INTERVAL", 30*time.Second); err != nil {
		return Config{}, err
	}

	if cfg.ListenPort < 1 || cfg.ListenPort > 65535 {
		return Config{}, fmt.Errorf("MOONMINER_LISTEN_PORT must be 1-65535, got %d", cfg.ListenPort)
	}
	if cfg.MaxSessionsPerKey < 1 {
		return Config{}, fmt.Errorf("MOONMINER_MAX_SESSIONS_PER_KEY must be at least 1")
	}
	if cfg.MaxConnections < 1 {
		return Config{}, fmt.Errorf("MOONMINER_MAX_CONNECTIONS must be at least 1")
	}
	if cfg.RateLimitPerSecond <= 0 || math.IsNaN(cfg.RateLimitPerSecond) || math.IsInf(cfg.RateLimitPerSecond, 0) {
		return Config{}, fmt.Errorf("MOONMINER_RATE_LIMIT_PER_SECOND must be positive")
	}
	if cfg.RateLimitBurst < 1 {
		return Config{}, fmt.Errorf("MOONMINER_RATE_LIMIT_BURST must be at least 1")
	}
	if cfg.RateLimitMaxEntries < 1 {
		return Config{}, fmt.Errorf("MOONMINER_RATE_LIMIT_MAX_IPS must be at least 1")
	}
	if cfg.IdleTimeout <= 0 {
		return Config{}, fmt.Errorf("MOONMINER_IDLE_TIMEOUT must be positive")
	}
	if cfg.AutosaveInterval < time.Second {
		return Config{}, fmt.Errorf("MOONMINER_AUTOSAVE_INTERVAL must be at least 1s")
	}
	if cfg.SessionPolicy != "takeover" && cfg.SessionPolicy != "refuse" {
		return Config{}, fmt.Errorf("MOONMINER_SESSION_POLICY must be takeover or refuse")
	}
	slot := identity.SanitizeSlot(cfg.DefaultSlot)
	if slot == "" {
		return Config{}, fmt.Errorf("MOONMINER_DEFAULT_SLOT must sanitize to 1-32 characters")
	}
	cfg.DefaultSlot = slot
	return cfg, nil
}

func (c Config) ListenAddr() string {
	return net.JoinHostPort(c.ListenHost, strconv.Itoa(c.ListenPort))
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOr(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s: invalid integer %q", key, v)
	}
	return n, nil
}

func envFloatOr(key string, fallback float64) (float64, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: invalid float %q", key, v)
	}
	return f, nil
}

func envDurationOr(key string, fallback time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%s: invalid duration %q", key, v)
	}
	return d, nil
}
