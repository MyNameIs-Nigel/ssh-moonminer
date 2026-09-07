package config

import (
	"strings"
	"testing"
)

// Clear every supported variable so the developer's own server settings do
// not change test results. These tests cannot run in parallel with Setenv.
func cleanEnvironment(t *testing.T) {
	t.Helper()
	for _, key := range []string{"LISTEN_HOST", "LISTEN_PORT", "HOST_KEY_PATH", "IDLE_TIMEOUT", "MAX_SESSIONS_PER_KEY", "MAX_CONNECTIONS", "DEFAULT_SLOT", "LOG_LEVEL", "LOG_FORMAT", "RATE_LIMIT_PER_SECOND", "RATE_LIMIT_BURST", "RATE_LIMIT_MAX_IPS", "DB_PATH", "AUTOSAVE_INTERVAL", "SESSION_POLICY", "DATA_DIR", "PROXY_KEYS_PATH", "DEV_MODE"} {
		t.Setenv("MOONMINER_"+key, "")
	}
}

func TestLoadDefaults(t *testing.T) {
	cleanEnvironment(t)
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.ListenAddr() != "0.0.0.0:22" || c.DefaultSlot != "default" || c.DevMode {
		t.Fatalf("unexpected defaults: %+v", c)
	}
}

func TestLoadRejectsInvalidLimits(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"RATE_LIMIT_PER_SECOND", "NaN"}, {"RATE_LIMIT_PER_SECOND", "+Inf"}, {"RATE_LIMIT_PER_SECOND", "-Inf"},
		{"RATE_LIMIT_BURST", "0"}, {"RATE_LIMIT_BURST", "-1"}, {"RATE_LIMIT_MAX_IPS", "0"}, {"IDLE_TIMEOUT", "0s"}, {"IDLE_TIMEOUT", "-1s"},
		{"DEV_MODE", "maybe"}, {"LISTEN_PORT", "65536"}, {"AUTOSAVE_INTERVAL", "1ms"}, {"SESSION_POLICY", "invalid"},
	} {
		t.Run(tc.key+"/"+tc.value, func(t *testing.T) {
			cleanEnvironment(t)
			key := "MOONMINER_" + tc.key
			t.Setenv(key, tc.value)
			if _, err := Load(); err == nil || !strings.Contains(err.Error(), key) {
				t.Fatalf("got %v, want error naming %s", err, key)
			}
		})
	}
}

func TestLoadDevModeBoolean(t *testing.T) {
	for _, v := range []string{"false", "0", "true", "1"} {
		t.Run(v, func(t *testing.T) {
			cleanEnvironment(t)
			t.Setenv("MOONMINER_DEV_MODE", v)
			c, err := Load()
			if err != nil {
				t.Fatal(err)
			}
			if c.DevMode != (v == "true" || v == "1") {
				t.Fatalf("DEV_MODE=%s enabled=%v", v, c.DevMode)
			}
		})
	}
}

func TestListenAddrIPv6(t *testing.T) {
	c := Config{ListenHost: "::1", ListenPort: 2222}
	if got := c.ListenAddr(); got != "[::1]:2222" {
		t.Fatalf("got %q", got)
	}
}
