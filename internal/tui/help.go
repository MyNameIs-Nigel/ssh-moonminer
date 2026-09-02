package tui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/theme"
	"github.com/mynameis-nigel/ssh-moonminer/internal/version"
)

const helpPanelW = 58
const helpPanelH = 16

// helpTextLine pairs a line's plain text with the style it renders in. Text
// and style are kept apart (rather than pre-rendering styled strings) so a
// line can be word-wrapped before the style — and its single opening/closing
// ANSI codes — gets applied per resulting row; wrapping an already-styled
// string would strand the reset code on whichever row got the last word,
// leaking color into the panel border past it.
type helpTextLine struct {
	text  string
	style lipgloss.Style
}

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

func (g *Game) helpEntries() []helpTextLine {
	plain := lipgloss.NewStyle()
	global := []helpTextLine{
		{"GLOBAL", theme.Bright},
		{"? toggle help · Esc or Q close help", plain},
		{"Mouse: click to select · double-click to activate", plain},
		{"Wheel: scroll lists and help text", plain},
		{"", plain},
	}
	if g.devMode {
		global = append(global,
			helpTextLine{"Ctrl+D dev tools (this is a dev build)", theme.Red},
			helpTextLine{"", plain},
		)
	}
	var screen []helpTextLine
	switch g.scr {
	case scrShipyard:
		screen = []helpTextLine{
			{"SHIPYARD", theme.Bright},
			{"Tab switch HANGAR/LOADOUT focus", plain},
			{"↑/↓ select ship, track, or slot", plain},
			{"Enter buy ship/track or open the item picker for a slot", plain},
			{"X or Backspace remove — then choose store or sell for 95%", plain},
			{"Esc or Q return to star chart", plain},
		}
	case scrBelt:
		screen = []helpTextLine{
			{"ASTEROID BELT", theme.Bright},
			{"↑/↓/←/→ cycle contacts", plain},
			{"Enter lock target and fly", plain},
			{"S scan selected asteroid", plain},
			{"V cycle belt view mode", plain},
			{"Q dock at port", plain},
		}
	case scrMining:
		screen = []helpTextLine{
			{"MINING", theme.Bright},
			{"B or Esc BAIL while asteroid ore remains", plain},
			{"Enter DEPART once the asteroid is depleted", plain},
			{"Space or click a lit pressure point to fracture the asteroid", plain},
			{"D drop cargo / R refuse if pirates demand tribute", plain},
			{"F fight (armed ships only) instead of dropping or running", plain},
			{"In combat: autocannons fire continuously; F fires Pulse Lasers on the arc; G fires guided missiles; B/Enter starts escape", plain},
			{"No input once escaping unarmed — the ship flees or dies", plain},
			{"Cargo stays in the top nav; watch the right-hand pirate scanner", plain},
		}
	case scrDeath:
		screen = []helpTextLine{
			{"CONNECTION LOST", theme.Bright},
			{"Any key returns to the dock respawn summary", plain},
		}
	case scrSummary:
		screen = []helpTextLine{
			{"RUN SUMMARY", theme.Bright},
			{"Enter return to belt", plain},
			{"Q dock at port", plain},
		}
	case scrLog:
		screen = []helpTextLine{
			{"SHIP'S LOG", theme.Bright},
			{"↑/↓ scroll run history", plain},
			{"Esc, Q, or L back to chart", plain},
		}
	default:
		screen = []helpTextLine{
			{"STAR CHART", theme.Bright},
			{"↑/↓ select destination", plain},
			{"Enter depart to asteroid belt", plain},
			{"C sell cargo + bounty vouchers · F refuel · R repair hull", plain},
			{"I insurance (when eligible)", plain},
			{"S shipyard · L ship's log", plain},
			{"T tweaks · Q quit", plain},
		}
	}
	return append(global, screen...)
}

// helpWrappedLines word-wraps every entry to innerW and applies its style to
// each resulting row, so long lines break at word boundaries instead of
// being cut mid-word by the panel's fixed-width clip.
func (g *Game) helpWrappedLines(innerW int) []string {
	entries := g.helpEntries()
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.text == "" {
			out = append(out, "")
			continue
		}
		wrapped := wrapChartText(e.text, innerW)
		if len(wrapped) == 0 {
			out = append(out, e.style.Render(e.text))
			continue
		}
		for _, w := range wrapped {
			out = append(out, e.style.Render(w))
		}
	}
	return out
}

func (g *Game) helpScrollVisible(panelH int) int {
	visible := panelH - 5
	if visible < 1 {
		return 1
	}
	return visible
}

func (g *Game) helpScrollMax() int {
	panelW, panelH := g.layoutOverlaySize(helpPanelW, helpPanelH)
	innerW := panelW - 4
	lines := g.helpWrappedLines(innerW)
	visible := g.helpScrollVisible(panelH)
	if len(lines) <= visible {
		return 0
	}
	return len(lines) - visible
}

func (g *Game) renderHelpOverlay() string {
	panelW, panelH := g.layoutOverlaySize(helpPanelW, helpPanelH)
	panelBodyH := panelH - 2
	innerW := panelW - 4
	lines := g.helpWrappedLines(innerW)
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
