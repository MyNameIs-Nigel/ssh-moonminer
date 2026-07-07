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
		"? toggle help · Esc close help",
		"Mouse: click to select · double-click to activate",
		"Wheel: scroll lists and help text",
		"",
	}
	var screen []string
	switch g.scr {
	case scrShipyard:
		screen = []string{
			theme.Bright.Render("SHIPYARD"),
			"↑/↓ select upgrade track",
			"Enter buy selected upgrade",
			"Esc return to star chart",
			"U shipyard · F refuel · H repair",
			"L ship's log · T tweaks · Q quit",
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
			"Space tap overdrive burst",
			"Click OVERDRIVE for burst",
			"B or Esc bail and bank cargo",
			"Watch drill, fuel, and pirate gauges",
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
			"↑/↓ select destination world",
			"Enter depart to asteroid belt",
			"F refuel · H repair hull",
			"I insurance (when eligible)",
			"U shipyard · L ship's log",
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
	hint := theme.DimStyle.Render("↑/↓ scroll · ? or Esc close")
	if g.helpScrollMax() == 0 {
		hint = theme.DimStyle.Render("? or Esc close")
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
