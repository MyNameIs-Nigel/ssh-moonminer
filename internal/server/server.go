package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"charm.land/wish/v2"
	"charm.land/wish/v2/bubbletea"
	"charm.land/wish/v2/logging"
	"charm.land/wish/v2/ratelimiter"
	"github.com/charmbracelet/ssh"
	"golang.org/x/time/rate"

	"github.com/mynameis-nigel/ssh-moonminer/internal/config"
	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
	"github.com/mynameis-nigel/ssh-moonminer/internal/game"
	"github.com/mynameis-nigel/ssh-moonminer/internal/identity"
	"github.com/mynameis-nigel/ssh-moonminer/internal/tui"
)

type sessionKey struct{}

type sessionState struct {
	id  identity.SessionIdentity
	res game.AttachResult
}

// SaveManager opens player saves.
type SaveManager interface {
	Attach(ctx context.Context, id identity.SessionIdentity, publicKey string, now int64, kick func(reason string)) (game.AttachResult, error)
	Shutdown(ctx context.Context) error
	Content() *content.Content
}

// Server wraps the Wish SSH server.
type Server struct {
	cfg      config.Config
	logger   *slog.Logger
	ssh      *ssh.Server
	games    SaveManager
	identity *identity.Resolver
}

// New constructs the SSH server.
func New(cfg config.Config, logger *slog.Logger, games SaveManager, resolver *identity.Resolver) (*Server, error) {
	if err := ensureHostKeyDir(cfg.HostKeyPath); err != nil {
		return nil, err
	}
	limits := NewSessionLimits(cfg.MaxConnections, cfg.MaxSessionsPerKey, resolver)
	rl := ratelimiter.NewRateLimiter(rate.Limit(cfg.RateLimitPerSecond), cfg.RateLimitBurst, cfg.RateLimitMaxEntries)
	srv := &Server{cfg: cfg, logger: logger, games: games, identity: resolver}
	s, err := wish.NewServer(
		wish.WithAddress(cfg.ListenAddr()),
		wish.WithHostKeyPath(cfg.HostKeyPath),
		wish.WithIdleTimeout(cfg.IdleTimeout),
		wish.WithPublicKeyAuth(func(_ ssh.Context, key ssh.PublicKey) bool { return key != nil }),
		wish.WithMiddleware(
			bubbletea.MiddlewareWithProgramHandler(srv.newTeaProgram),
			srv.attachSave(),
			RequirePTY(),
			limits.Middleware(),
			ratelimiter.Middleware(rl),
			logging.Middleware(),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create ssh server: %w", err)
	}
	srv.ssh = s
	return srv, nil
}

func (srv *Server) attachSave() func(ssh.Handler) ssh.Handler {
	return func(next ssh.Handler) ssh.Handler {
		return func(s ssh.Session) {
			resolved, err := srv.identity.Resolve(s)
			if err != nil {
				if errors.Is(err, identity.ErrProxiedIdentity) {
					_, _ = io.WriteString(s, identity.ProxiedIdentityMessage())
				} else {
					_, _ = io.WriteString(s, "Could not verify identity. Please try again.\r\n")
				}
				s.Exit(1)
				return
			}
			id := resolved.Identity
			res, err := srv.games.Attach(s.Context(), id, resolved.PublicKey, time.Now().Unix(), nil)
			if err != nil {
				if errors.Is(err, game.ErrSaveBusy) {
					_, _ = io.WriteString(s, "This pilot is already open in another session.\r\n")
				} else {
					_, _ = io.WriteString(s, "Could not open your save. Please try again.\r\n")
				}
				s.Exit(1)
				return
			}
			defer res.Session.Detach()
			s.Context().SetValue(sessionKey{}, &sessionState{id: id, res: res})
			next(s)
		}
	}
}

func (srv *Server) teaHandler(s ssh.Session) (tui.Model, []tui.ProgramOption) {
	state, _ := s.Context().Value(sessionKey{}).(*sessionState)
	if state == nil {
		return tui.NewErrScreen(), nil
	}
	width, height := 80, 24
	if pty, _, ok := s.Pty(); ok {
		width, height = pty.Window.Width, pty.Window.Height
	}
	idleSecs := int64(srv.cfg.IdleTimeout / time.Second)
	return tui.NewGame(state.id, state.res, srv.games.Content(), width, height, time.Now().Unix(), idleSecs), nil
}

func (srv *Server) ListenAndServe() error {
	srv.logger.Info("ssh server listening", "addr", srv.cfg.ListenAddr())
	return srv.ssh.ListenAndServe()
}

func (srv *Server) Shutdown(ctx context.Context) error {
	return srv.ssh.Shutdown(ctx)
}

func ensureHostKeyDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o700)
}
