package server

import (
	"sync"
	"context"
)

// ShutdownHook runs during graceful shutdown.
type ShutdownHook func(ctx context.Context) error

var (
	shutdownMu sync.Mutex
	shutdowns  []ShutdownHook
)

// RegisterShutdownHook adds a shutdown hook.
func RegisterShutdownHook(h ShutdownHook) {
	shutdownMu.Lock()
	defer shutdownMu.Unlock()
	shutdowns = append(shutdowns, h)
}

// RunShutdownHooks executes registered hooks.
func RunShutdownHooks(ctx context.Context) error {
	shutdownMu.Lock()
	hooks := append([]ShutdownHook(nil), shutdowns...)
	shutdownMu.Unlock()
	for _, h := range hooks {
		if err := h(ctx); err != nil {
			return err
		}
	}
	return nil
}
