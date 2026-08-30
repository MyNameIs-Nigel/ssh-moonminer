package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/theme"
)

// The Dev overlay is a debug panel for local testing only: it is wired up
// by cmd/dev-moonminer (MOONMINER_DEV_MODE=1) and lets a developer jump
// straight to any resource level or screen instead of grinding a run out
// manually. It never ships enabled in production (server.go only passes
// devMode through from config.Config.DevMode, which defaults to false).

const (
	devCredits = iota
	devFuel
	devHull
	devGodMode
	devMaxUpgrades
	devJumpScreen
	devRowCount
)

var devJumpScreens = []screen{scrChart, scrShipyard, scrBelt, scrMining, scrSummary, scrLog, scrDeath}
var devJumpLabels = []string{"chart", "shipyard", "belt", "mining", "summary", "shiplog", "death"}

const devPanelW = 46
const devPanelH = 13

func (g *Game) devLabel(i int) string {
	switch i {
	case devCredits:
		return "CREDITS"
	case devFuel:
		return "FUEL"
	case devHull:
		return "HULL"
	case devGodMode:
		return "GOD MODE"
	case devMaxUpgrades:
		return "MAX SHIP (enter)"
	case devJumpScreen:
		return "JUMP TO SCREEN (enter)"
	default:
		return ""
	}
}

func (g *Game) devJumpIndex() int {
	for i, s := range devJumpScreens {
		if s == g.scr {
			return i
		}
	}
	return 0
}

func (g *Game) devValue(i int) string {
	st := g.snap.State
	switch i {
	case devCredits:
		return fmt.Sprintf("%d", st.Credits)
	case devFuel:
		return fmt.Sprintf("%.0f / %.0f", st.Fuel, sim.TankSize(&st, g.content))
	case devHull:
		return fmt.Sprintf("%d / %d", st.Hull, sim.MaxHull(&st, g.content))
	case devGodMode:
		return boolLabel(st.DevGodMode)
	case devMaxUpgrades:
		return ""
	case devJumpScreen:
		return devJumpLabels[g.devJumpIndex()]
	default:
		return ""
	}
}

// devDelta applies a left(-1)/right(+1) nudge to the selected dev row.
func (g *Game) devDelta(delta int) []tea.Cmd {
	st := g.snap.State
	switch g.devSel {
	case devCredits:
		snap, err := g.sess.DevSetCredits(g.now, st.Credits+delta*100)
		return g.refreshSnap(snap, err)
	case devFuel:
		snap, err := g.sess.DevSetFuel(g.now, st.Fuel+float64(delta)*10)
		return g.refreshSnap(snap, err)
	case devHull:
		snap, err := g.sess.DevSetHull(g.now, st.Hull+delta*10)
		return g.refreshSnap(snap, err)
	case devGodMode:
		if delta != 0 {
			snap, err := g.sess.DevSetGodMode(g.now, !st.DevGodMode)
			return g.refreshSnap(snap, err)
		}
	case devMaxUpgrades:
		if delta != 0 {
			snap, err := g.sess.DevMaxShip(g.now)
			return g.refreshSnap(snap, err)
		}
	case devJumpScreen:
		if delta != 0 {
			n := len(devJumpScreens)
			idx := ((g.devJumpIndex()+delta)%n + n) % n
			g.scr = devJumpScreens[idx]
		}
	}
	return nil
}

// devActivate runs the row's default action on enter/space (same as a
// right-nudge for toggles/actions, a no-op for numeric rows).
func (g *Game) devActivate() []tea.Cmd {
	switch g.devSel {
	case devMaxUpgrades, devJumpScreen, devGodMode:
		return g.devDelta(1)
	}
	return nil
}

func (g *Game) renderDevOverlay() string {
	hc := g.snap.State.Settings.HighContrast
	panelW, panelH := g.layoutOverlaySize(devPanelW, devPanelH)
	panelBodyH := panelH - 2
	innerW := panelW - 4
	lines := []string{
		theme.LabelStyle(hc).Render("DEV ONLY · not present in production"),
		theme.LabelStyle(hc).Render("Arrow keys change values · Ctrl+D or Esc closes"),
		"",
	}
	devLineIdx := 3
	for i := 0; i < devRowCount; i++ {
		marker := "  "
		sel := i == g.devSel
		if sel {
			marker = "▸ "
		}
		label := g.devLabel(i)
		val := g.devValue(i)
		left := marker + label
		pad := innerW - lipgloss.Width(left) - lipgloss.Width(val)
		if pad < 1 {
			pad = 1
		}
		line := theme.OptionHC(theme.HueRed, sel, hc).Render(left+strings.Repeat(" ", pad)) + theme.Bright.Render(val)
		line = clipFrameLine(line, innerW)
		lines = append(lines, line)
	}
	scroll := 0
	if len(lines) > panelBodyH {
		scroll = centeredScroll(g.devSel+devLineIdx, len(lines), panelBodyH)
	}
	visible := scrollWindowLines(lines, scroll, panelBodyH)
	for i, line := range visible {
		src := scroll + i
		if src >= devLineIdx && src < devLineIdx+devRowCount {
			g.devHitLine(i, line, src-devLineIdx, panelW, panelH)
		}
	}
	body := strings.Join(visible, "\n")
	return theme.Panel("DEV TOOLS", panelW, panelH, body, theme.Accent(theme.HueRed))
}

func (g *Game) devHitLine(lineIdx int, label string, devIdx, panelW, panelH int) {
	g.overlayHitLine(panelW, panelH, lineIdx, label, "dev", devIdx)
}

func (g *Game) updateDevOverlay(k string) []tea.Cmd {
	switch k {
	case "esc", "ctrl+d":
		g.overlay = ovNone
	case "up", "k":
		g.devSel = (g.devSel - 1 + devRowCount) % devRowCount
	case "down", "j":
		g.devSel = (g.devSel + 1) % devRowCount
	case "left", "h":
		return g.devDelta(-1)
	case "right", "l":
		return g.devDelta(1)
	case "enter", " ":
		return g.devActivate()
	}
	return nil
}
