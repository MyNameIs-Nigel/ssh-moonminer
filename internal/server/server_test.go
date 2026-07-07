package server

import (
	"bufio"
	"context"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"charm.land/wish/v2/testsession"

	"github.com/mynameis-nigel/ssh-moonminer/internal/config"
	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
	"github.com/mynameis-nigel/ssh-moonminer/internal/game"
	"github.com/mynameis-nigel/ssh-moonminer/internal/identity"
	applog "github.com/mynameis-nigel/ssh-moonminer/internal/log"
	"github.com/mynameis-nigel/ssh-moonminer/internal/store"
	"github.com/mynameis-nigel/ssh-moonminer/internal/version"
)

func testServer(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	cfg := config.Config{
		ListenHost:         "127.0.0.1",
		HostKeyPath:        filepath.Join(dir, "host_key"),
		DBPath:             filepath.Join(dir, "moonminer.db"),
		IdleTimeout:        time.Hour,
		MaxSessionsPerKey:  2,
		MaxConnections:     10,
		RateLimitPerSecond: 100,
		RateLimitBurst:     5,
		DefaultSlot:        "default",
		SessionPolicy:      "takeover",
		AutosaveInterval:   30 * time.Second,
	}

	logger := applog.New("error", "text")

	c, err := content.Load("")
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(context.Background(), cfg.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	games := game.NewManager(st, c, logger, cfg.AutosaveInterval, game.Policy(cfg.SessionPolicy))

	resolver := identity.NewResolver(cfg.DefaultSlot, nil)

	srv, err := New(cfg, logger, games, resolver)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = games.Shutdown(ctx)
		_ = srv.Shutdown(ctx)
	})

	return testsession.Listen(t, srv.ssh)
}

// TestServerVersionBanner: the raw SSH version-exchange banner embeds this
// game's fleet version so the arcade router's health-check prober can read
// it live (see ../../ssh-arcadelobby/docs/03-games-registry-and-health.md)
// instead of a hand-maintained games.toml field. A raw TCP dial (not a full
// SSH handshake) reads the literal bytes the prober itself reads.
func TestServerVersionBanner(t *testing.T) {
	addr := testServer(t)

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	want := "SSH-2.0-" + version.Version
	if !strings.HasPrefix(strings.TrimRight(line, "\r\n"), want) {
		t.Fatalf("banner = %q, want prefix %q", line, want)
	}
}
