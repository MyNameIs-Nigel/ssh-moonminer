// Package app holds the shared server boot/shutdown sequence used by both
// cmd/ssh-moonminer (production) and cmd/dev-moonminer (local dev, with
// MOONMINER_DEV_MODE and friends pre-set before Run is called).
package app

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/ssh"

	"github.com/mynameis-nigel/ssh-moonminer/internal/config"
	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
	"github.com/mynameis-nigel/ssh-moonminer/internal/game"
	"github.com/mynameis-nigel/ssh-moonminer/internal/identity"
	applog "github.com/mynameis-nigel/ssh-moonminer/internal/log"
	"github.com/mynameis-nigel/ssh-moonminer/internal/server"
	"github.com/mynameis-nigel/ssh-moonminer/internal/store"
)

// Run loads config from the environment and serves until an interrupt or
// terminate signal, exiting the process on unrecoverable startup errors.
func Run() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "error", err)
		os.Exit(1)
	}
	logger := applog.New(cfg.LogLevel, cfg.LogFormat)
	slog.SetDefault(logger)

	c, err := content.Load(cfg.DataDir)
	if err != nil {
		logger.Error("content load failed", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()
	st, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		logger.Error("store open failed", "error", err)
		os.Exit(1)
	}
	defer st.Close()

	proxyKeys, err := identity.LoadProxyKeys(cfg.ProxyKeysPath)
	if err != nil {
		logger.Error("proxy keys load failed", "error", err)
		os.Exit(1)
	}
	resolver := identity.NewResolver(cfg.DefaultSlot, proxyKeys)

	games := game.NewManager(st, c, logger, cfg.AutosaveInterval, game.Policy(cfg.SessionPolicy))
	server.RegisterShutdownHook(games.Shutdown)

	srv, err := server.New(cfg, logger, games, resolver)
	if err != nil {
		logger.Error("server init failed", "error", err)
		os.Exit(1)
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			logger.Error("listen failed", "error", err)
			done <- syscall.SIGTERM
		}
	}()

	if cfg.DevMode {
		logger.Warn("dev mode enabled — dev tools overlay (Ctrl+D) is active; never set MOONMINER_DEV_MODE in production")
	}

	sig := <-done
	logger.Info("shutdown signal", "signal", sig.String())

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := server.RunShutdownHooks(shutdownCtx); err != nil {
		logger.Error("shutdown hook failed", "error", err)
	}
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
		logger.Error("shutdown failed", "error", err)
		os.Exit(1)
	}
	logger.Info("shutdown complete")
}
