package server

import (
	"fmt"
	"io"
	"sync"

	"github.com/charmbracelet/ssh"
	"github.com/mynameis-nigel/ssh-moonminer/internal/identity"
)

// SessionLimits tracks concurrent sessions per resolved fingerprint.
type SessionLimits struct {
	mu        sync.Mutex
	global    int
	perKey    map[string]int
	maxGlobal int
	maxPerKey int
	resolver  *identity.Resolver
}

// NewSessionLimits creates a limiter keyed on resolved identity.
func NewSessionLimits(maxGlobal, maxPerKey int, resolver *identity.Resolver) *SessionLimits {
	return &SessionLimits{
		perKey: make(map[string]int), maxGlobal: maxGlobal,
		maxPerKey: maxPerKey, resolver: resolver,
	}
}

// Middleware rejects over-limit connections.
func (l *SessionLimits) Middleware() func(ssh.Handler) ssh.Handler {
	return func(next ssh.Handler) ssh.Handler {
		return func(s ssh.Session) {
			resolved, err := l.resolver.Resolve(s)
			if err != nil {
				if identity.IsProxiedIdentityError(err) {
					_, _ = io.WriteString(s, identity.ProxiedIdentityMessage())
				} else {
					_, _ = io.WriteString(s, "Public key authentication is required.\r\n")
				}
				s.Exit(1)
				return
			}
			fp := resolved.Identity.Fingerprint
			if !l.acquire(fp) {
				_, _ = io.WriteString(s, fmt.Sprintf(
					"Too many active sessions (global max %d, per-key max %d).\r\n",
					l.maxGlobal, l.maxPerKey))
				s.Exit(1)
				return
			}
			defer l.release(fp)
			next(s)
		}
	}
}

func (l *SessionLimits) acquire(fp string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.global >= l.maxGlobal || l.perKey[fp] >= l.maxPerKey {
		return false
	}
	l.global++
	l.perKey[fp]++
	return true
}

func (l *SessionLimits) release(fp string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.global--
	if l.perKey[fp] > 0 {
		l.perKey[fp]--
		if l.perKey[fp] == 0 {
			delete(l.perKey, fp)
		}
	}
}
