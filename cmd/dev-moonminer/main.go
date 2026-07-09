// Command dev-moonminer runs the same server as cmd/ssh-moonminer with
// dev-friendly defaults: a fixed local port, its own var/dev/ save directory
// (never touches a real pilot's var/moonminer.db), verbose logging, and the
// in-game dev tools overlay (Ctrl+D) enabled — set/get credits, fuel, hull,
// god mode, and jump straight to any screen without grinding out a run.
//
//	go run ./cmd/dev-moonminer
//	ssh -p 2222 -o StrictHostKeyChecking=accept-new localhost
//
// Every default below is only applied if the corresponding MOONMINER_* env
// var isn't already set, so `FOO=bar go run ./cmd/dev-moonminer` still works
// for one-off overrides.
package main

import (
	"os"

	"github.com/mynameis-nigel/ssh-moonminer/internal/app"
)

func main() {
	setDefaultEnv("MOONMINER_LISTEN_PORT", "2222")
	setDefaultEnv("MOONMINER_DB_PATH", "var/dev/moonminer-dev.db")
	setDefaultEnv("MOONMINER_HOST_KEY_PATH", "var/dev/ssh_host_key")
	setDefaultEnv("MOONMINER_LOG_LEVEL", "debug")
	setDefaultEnv("MOONMINER_DEV_MODE", "1")
	setDefaultEnv("MOONMINER_IDLE_TIMEOUT", "8760h") // dev sessions shouldn't idle-kick

	app.Run()
}

func setDefaultEnv(key, val string) {
	if _, ok := os.LookupEnv(key); !ok {
		os.Setenv(key, val)
	}
}
