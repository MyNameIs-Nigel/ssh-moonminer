package tui

import (
	"strings"

	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/theme"
	"github.com/mynameis-nigel/ssh-moonminer/internal/version"
)

const helpPanelW = 58
const helpPanelH = 16

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
			"B or Esc BAIL while asteroid ore remains",
			"Enter DEPART once the asteroid is depleted",
			"Space or click a lit pressure point to fracture the asteroid",
			"D drop cargo / R refuse if pirates demand tribute",
			"F fight (armed ships only) instead of dropping or running",
			"In combat: autocannons fire continuously; F fires Pulse Lasers on the arc; G fires guided missiles; B/Enter starts escape",
			"No input once escaping unarmed — the ship flees or dies",
			"Cargo stays in the top nav; watch the right-hand pirate scanner",
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
			"C sell cargo + bounty vouchers · F refuel · R repair hull",
			"I insurance (when eligible)",
			"S shipyard · L ship's log",
			"T tweaks · Q quit",
		}
	}
	return append(global, screen...)
}

func (g *Game) helpScrollVisible(panelH int) int {
	visible := panelH - 5
	if visible < 1 {
		return 1
	}
	return visible
}

func (g *Game) helpScrollMax() int {
	_, panelH := g.layoutOverlaySize(helpPanelW, helpPanelH)
	lines := g.helpLines()
	visible := g.helpScrollVisible(panelH)
	if len(lines) <= visible {
		return 0
	}
	return len(lines) - visible
}

func (g *Game) renderHelpOverlay() string {
	lines := g.helpLines()
	panelW, panelH := g.layoutOverlaySize(helpPanelW, helpPanelH)
	panelBodyH := panelH - 2
	innerW := panelW - 4
	if g.helpScroll > g.helpScrollMax() {
		g.helpScroll = g.helpScrollMax()
	}
	if g.helpScroll < 0 {
		g.helpScroll = 0
	}
	hint := theme.DimStyle.Render("↑/↓ scroll · ?, Esc, or Q close")
	if g.helpScrollMax() == 0 {
		hint = theme.DimStyle.Render("?, Esc, or Q close")
	}
	ver := theme.DimStyle.Render("MOON MINER v" + version.Version + " (" + version.Channel + ")")
	footer := []string{"", hint, ver}
	scrollRows := panelBodyH - len(footer)
	view := scrollWindowLines(lines, g.helpScroll, scrollRows)
	bodyLines := bottomAlignPanelLines(view, footer, panelBodyH)
	for i := range bodyLines {
		bodyLines[i] = clipFrameLine(bodyLines[i], innerW)
	}
	body := strings.Join(bodyLines, "\n")
	return theme.Panel("HELP — "+g.helpScreenName(), panelW, panelH, body, theme.Accent(theme.HueCyan))
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
