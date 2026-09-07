package server

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"testing"
	"time"

	gossh "golang.org/x/crypto/ssh"
)

func TestServerPublicKeyAndPTYRequirement(t *testing.T) {
	addr := testServer(t)
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := gossh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		auth    gossh.AuthMethod
		allowed bool
	}{
		{"public key", gossh.PublicKeys(signer), true},
		{"password", gossh.Password("not-accepted"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
				t.Fatal(err)
			}
			cc, chans, reqs, err := gossh.NewClientConn(conn, addr, &gossh.ClientConfig{User: "audit", Auth: []gossh.AuthMethod{tc.auth}, HostKeyCallback: gossh.InsecureIgnoreHostKey()})
			if !tc.allowed {
				if err == nil {
					cc.Close()
					t.Fatal("password authentication succeeded")
				}
				return
			}
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
			output, err := session.Output("ignored")
			if err != nil {
				t.Fatal(err)
			}
			if string(output) != noPTYMessage {
				t.Fatalf("non-PTY response = %q, want %q", output, noPTYMessage)
			}
		})
	}
}

func TestSessionLimitsReleaseAndFingerprintIsolation(t *testing.T) {
	limits := NewSessionLimits(2, 1, nil)
	if !limits.acquire("one") || limits.acquire("one") {
		t.Fatal("per-key limit failed")
	}
	if !limits.acquire("two") || limits.acquire("three") {
		t.Fatal("global limit or key isolation failed")
	}
	limits.release("one")
	if !limits.acquire("three") {
		t.Fatal("released capacity not reusable")
	}
	limits.release("two")
	limits.release("three")
	if limits.global != 0 || len(limits.perKey) != 0 {
		t.Fatal("session counters leaked")
	}
}
