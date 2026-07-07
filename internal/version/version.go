// Package version pins the game's release version.
//
// The whole ssharcade fleet shares one scheme, <channel>.<major>.<minor>:
// the leading number is the release channel (1 = alpha, 2 = beta), the
// second is the major release, and the third is the minor patch/hotfix.
// Keep this in sync with the `version` key of this game's entry in
// ssh-arcadelobby's games.toml — the lobby menu renders that key, the help
// overlay renders this constant, and players see both.
package version

// Version is the game's current release. Moon Miner is in alpha.
const Version = "1.0.0"

// Channel is the human-readable release channel derived from Version's
// leading component.
const Channel = "alpha"
