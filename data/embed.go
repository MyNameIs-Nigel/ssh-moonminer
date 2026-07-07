// Package data embeds game content TOML files for self-contained builds.
package data

import "embed"

//go:embed *.toml
var FS embed.FS
