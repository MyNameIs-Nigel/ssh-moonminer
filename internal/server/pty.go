package server

import (
	"io"

	"github.com/charmbracelet/ssh"
)

const noPTYMessage = "ssh-moonminer needs an interactive terminal.\r\n" +
	"Connect with: ssh -t user@host\r\n"

// RequirePTY rejects sessions without a PTY.
func RequirePTY() func(ssh.Handler) ssh.Handler {
	return func(next ssh.Handler) ssh.Handler {
		return func(s ssh.Session) {
			if _, _, ok := s.Pty(); !ok {
				_, _ = io.WriteString(s, noPTYMessage)
				s.Exit(0)
				return
			}
			next(s)
		}
	}
}
