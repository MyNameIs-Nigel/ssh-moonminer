package tui

import (
	"strings"

	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/theme"
	"github.com/mynameis-nigel/ssh-moonminer/internal/version"
)

const helpPanelW = 58
const helpPanelH = 16
const helpScrollVisible = 9

func (g *Game) helpScreenName() string {
	switch g.scr {
	case scrShipyard:
		return "SHIPYARD"
	case scrBelt:
		return "ASTEROID BELT"
	case scrMining:
		return "MINING"
	case scrDeath:
		return "CONNECTION LOST"
	case scrSummary:
		return "RUN SUMMARY"
	case scrLog:
		return "SHIP'S LOG"
	default:
		return "STAR CHART"
	}
}

func (g *Game) helpLines() []string {
	global := []string{
		theme.Bright.Render("GLOBAL"),
		"? toggle help · Esc or Q close help",
		"Mouse: click to select · double-click to activate",
		"Wheel: scroll lists and help text",
		"",
	}
	if g.devMode {
		global = append(global, theme.Red.Render("Ctrl+D dev tools (this is a dev build)"), "")
	}
	var screen []string
	switch g.scr {
	case scrShipyard:
		screen = []string{
			theme.Bright.Render("SHIPYARD"),
			"Tab switch HANGAR/LOADOUT focus",
			"↑/↓ select ship, track, or slot",
			"Enter buy ship/track or open the item picker for a slot",
			"X or Backspace remove — then choose store or sell for 95%",
			"Esc or Q return to star chart",
		}
	case scrBelt:
		screen = []string{
			theme.Bright.Render("ASTEROID BELT"),
			"↑/↓/←/→ cycle contacts",
			"Enter lock target and fly",
			"S scan selected asteroid",
			"V cycle belt view mode",
			"Q dock at port",
		}
	case scrMining:
		screen = []string{
			theme.Bright.Render("MINING"),
			"B or Esc BAIL while resources remain",
			"Enter DEPART once the asteroid is depleted",
			"Space or click the gauge to hit a drill calibration check",
			"D drop cargo / R refuse if pirates demand tribute",
			"No input once escaping — the ship flees or dies",
			"Watch resource, hull, fuel, and the pirate radar/ETA widget",
		}
	case scrDeath:
		screen = []string{
			theme.Bright.Render("CONNECTION LOST"),
			"Any key returns to the dock respawn summary",
		}
	case scrSummary:
		screen = []string{
			theme.Bright.Render("RUN SUMMARY"),
			"Enter return to belt",
			"Q dock at port",
		}
	case scrLog:
		screen = []string{
			theme.Bright.Render("SHIP'S LOG"),
			"↑/↓ scroll run history",
			"Esc, Q, or L back to chart",
		}
	default:
		screen = []string{
			theme.Bright.Render("STAR CHART"),
			"↑/↓ select destination",
			"Enter depart to asteroid belt",
			"C sell cargo · F refuel · R repair hull",
			"I insurance (when eligible)",
			"S shipyard · L ship's log",
			"T tweaks · Q quit",
		}
	}
	return append(global, screen...)
}

func (g *Game) helpScrollMax() int {
	lines := g.helpLines()
	if len(lines) <= helpScrollVisible {
		return 0
	}
	return len(lines) - helpScrollVisible
}

func (g *Game) renderHelpOverlay() string {
	lines := g.helpLines()
	if g.helpScroll > g.helpScrollMax() {
		g.helpScroll = g.helpScrollMax()
	}
	if g.helpScroll < 0 {
		g.helpScroll = 0
	}
	view := make([]string, 0, helpScrollVisible)
	for i := 0; i < helpScrollVisible; i++ {
		idx := g.helpScroll + i
		if idx < len(lines) {
			view = append(view, lines[idx])
		} else {
			view = append(view, "")
		}
	}
	hint := theme.DimStyle.Render("↑/↓ scroll · ?, Esc, or Q close")
	if g.helpScrollMax() == 0 {
		hint = theme.DimStyle.Render("?, Esc, or Q close")
	}
	ver := theme.DimStyle.Render("MOON MINER v" + version.Version + " (" + version.Channel + ")")
	view = append(view, "", hint, ver)
	body := strings.Join(view, "\n")
	return theme.Panel("HELP — "+g.helpScreenName(), helpPanelW, helpPanelH, body, theme.Accent(theme.HueCyan))
}

func (g *Game) helpScrollBy(delta int) {
	g.helpScroll += delta
	if g.helpScroll < 0 {
		g.helpScroll = 0
	}
	if g.helpScroll > g.helpScrollMax() {
		g.helpScroll = g.helpScrollMax()
	}
}
