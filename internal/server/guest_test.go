package server

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"log/slog"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"charm.land/wish/v2/testsession"
	"github.com/charmbracelet/ssh"
	gossh "golang.org/x/crypto/ssh"

	"github.com/mynameis-nigel/ssh-moonminer/internal/config"
	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
	"github.com/mynameis-nigel/ssh-moonminer/internal/game"
	"github.com/mynameis-nigel/ssh-moonminer/internal/identity"
)

type contextBase struct {
	ssh.Context
	base context.Context
}

func (c contextBase) Value(key any) any { return c.base.Value(key) }
func TestShellStateIsIsolatedBetweenChannels(t *testing.T) {
	type key struct{}
	shared := shellSession{ctx: contextBase{base: t.Context()}}
	var scopes []ssh.Context
	handler := scopeSession()(func(s ssh.Session) { scopes = append(scopes, s.Context()); s.Context().SetValue(key{}, len(scopes)) })
	handler(shared)
	handler(shared)
	if scopes[0].Value(key{}) != 1 || scopes[1].Value(key{}) != 2 {
		t.Fatal("shell state shared")
	}
}
func TestGuestSSHAdmissionNeverReachesStorage(t *testing.T) {
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	proxy, err := gossh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	for _, allowed := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "enabled"}[allowed], func(t *testing.T) {
			cfg, err := config.Load()
			if err != nil {
				t.Fatal(err)
			}
			cfg.HostKeyPath = filepath.Join(t.TempDir(), "hostkey")
			cfg.AllowGuests = allowed
			c, err := content.Load("")
			if err != nil {
				t.Fatal(err)
			}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			m := game.NewManager(nil, c, logger, time.Second, game.PolicyTakeover)
			resolver := identity.NewResolver("default", []ssh.PublicKey{proxy.PublicKey()}, identity.WithGuests(cfg.AllowGuests))
			srv, err := New(cfg, logger, m, resolver)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = m.Shutdown(ctx)
				_ = srv.Shutdown(ctx)
			})
			addr := testsession.Listen(t, srv.ssh)
			conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			conn.SetDeadline(time.Now().Add(5 * time.Second))
			cc, chans, reqs, err := gossh.NewClientConn(conn, addr, &gossh.ClientConfig{User: "guest." + strings.Repeat("a", 32), Auth: []gossh.AuthMethod{gossh.PublicKeys(proxy)}, HostKeyCallback: gossh.InsecureIgnoreHostKey()})
			if err != nil {
				t.Fatal(err)
			}
			client := gossh.NewClient(cc, chans, reqs)
			defer client.Close()
			session, err := client.NewSession()
			if err != nil {
				t.Fatal(err)
			}
			defer session.Close()
			session.RequestPty("xterm-256color", 24, 80, gossh.TerminalModes{})
			out, _ := session.StdoutPipe()
			if err = session.Shell(); err != nil {
				t.Fatal(err)
			}
			var screen strings.Builder
			buf := make([]byte, 8192)
			for {
				n, err := out.Read(buf)
				screen.Write(buf[:n])
				s := screen.String()
				if allowed && strings.Contains(s, "GUEST") && strings.Contains(s, "NOT SAVED") {
					break
				}
				if err != nil {
					break
				}
			}
			if allowed && !strings.Contains(screen.String(), "NOT SAVED") {
				t.Fatal(screen.String())
			}
			if !allowed && !strings.Contains(screen.String(), "Could not verify") {
				t.Fatal(screen.String())
			}
		})
	}
}
