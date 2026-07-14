package tui

import (
	"fmt"
	"math"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/theme"
)

// combatIntroDoneMsg fires once, ~2s after the COMBAT MODE interstitial
// opens, unless it's dismissed sooner by a keypress.
type combatIntroDoneMsg struct{}

const combatIntroHold = 2 * time.Second

func combatIntroCmd() tea.Cmd {
	return tea.Tick(combatIntroHold, func(time.Time) tea.Msg { return combatIntroDoneMsg{} })
}

// syncCombatEntry checks the just-refreshed snapshot for a fresh transition
// into PhaseCombat and, if found, arms the COMBAT MODE interstitial. Call
// after every place g.snap is assigned from a live sim snapshot (the
// snapMsg handler and refreshSnap) so both server-driven and player-driven
// entries (Fight, Refuse, an immediate attack arriving) trigger it exactly
// once per engagement.
func (g *Game) syncCombatEntry() []tea.Cmd {
	run := g.snap.State.Run
	inCombat := run != nil && run.Phase == sim.PhaseCombat
	if !inCombat {
		g.combatEntered = false
		return nil
	}
	if g.combatEntered {
		return nil
	}
	g.combatEntered = true
	if g.snap.State.Settings.ReducedMotion {
		return nil
	}
	g.combatIntroActive = true
	return []tea.Cmd{combatIntroCmd()}
}

const (
	combatBg  = "#3a2208" // theme's hueColors(HueAmber) background
	combatFg  = "#ffae3f" // theme's amberC
	combatDim = "#a66a1a" // theme's amberDimC
)

// renderCombatIntro draws the full-screen "COMBAT MODE ENGAGED" flood card
// — the death screen's renderDeathFinal pattern in amber instead of red, a
// solid pop with no flicker, held for combatIntroHold before the tactical
// scope takes over.
func (g *Game) renderCombatIntro() string {
	bgStyle := lipgloss.NewStyle().Background(lipgloss.Color(combatBg))
	titleStyle := bgStyle.Foreground(lipgloss.Color(combatFg)).Bold(true)
	subStyle := bgStyle.Foreground(lipgloss.Color(combatDim))

	body := titleStyle.Render("⚠ COMBAT MODE ENGAGED ⚠")
	if run := g.snap.State.Run; run != nil && run.Combat != nil {
		st := g.snap.State
		sub := fmt.Sprintf("%s — BOUNTY %s %d", run.Combat.PirateName,
			theme.Glyph("credit", st.Settings.ASCIISafe), run.Combat.Bounty)
		body += "\n" + subStyle.Render(sub)
	}
	centered := lipgloss.Place(g.width, g.height-2, lipgloss.Center, lipgloss.Center, body,
		lipgloss.WithWhitespaceStyle(bgStyle))
	hint := subStyle.Render("brace for impact")
	hintLine := lipgloss.Place(g.width, 1, lipgloss.Center, lipgloss.Top, hint,
		lipgloss.WithWhitespaceStyle(bgStyle))
	return centered + "\n" + hintLine
}

func (g *Game) keyCombat(m tea.KeyPressMsg) []tea.Cmd {
	k := m.String()
	run := g.snap.State.Run
	if run == nil || run.Phase != sim.PhaseCombat {
		return nil
	}
	switch k {
	case "f":
		if !sim.HasManualWeapon(&g.snap.State) {
			return nil
		}
		snap, err := g.sess.FireWeapons(g.now)
		return g.refreshSnap(snap, err)
	case "b", "esc", "enter":
		snap, err := g.sess.CombatEscape(g.now)
		return g.refreshSnap(snap, err)
	}
	return nil
}

const combatScopeW = 34
const combatScopeH = 10

// renderCombatScope draws the tactical scope: a fixed player anchor at the
// firing arc's center, the arc itself as a highlighted band, and the
// pirate blip plotted from the sim-authoritative Bearing/Range each frame.
func (g *Game) renderCombatScope(cs *sim.CombatState) string {
	w, h := combatScopeW, combatScopeH
	cc := g.content.Combat
	arcCenter := clampF(cc.ArcCenter, 0, 1)
	arcHalf := cc.ArcHalfWidth
	if arcHalf <= 0 {
		arcHalf = 0.18
	}
	anchorCol := clampInt(int(arcCenter*float64(w-1)), 0, w-1)
	anchorRow := h - 1
	blipCol := clampInt(int(cs.Bearing*float64(w-1)), 0, w-1)
	blipRow := clampInt(int((1-cs.Range)*float64(h-2)), 0, h-2)

	lines := make([]string, 0, h)
	for r := 0; r < h; r++ {
		cells := make([]string, w)
		for col := 0; col < w; col++ {
			bearing := float64(col) / float64(w-1)
			d := math.Abs(bearing - arcCenter)
			if d > 0.5 {
				d = 1 - d
			}
			switch {
			case col == blipCol && r == blipRow:
				cells[col] = theme.Red.Render("●")
			case col == anchorCol && r == anchorRow:
				cells[col] = theme.Cyan.Render("▲")
			case d <= arcHalf:
				cells[col] = theme.Amber.Faint(true).Render("┆")
			default:
				cells[col] = theme.DimStyle.Render("·")
			}
		}
		lines = append(lines, strings.Join(cells, ""))
	}
	return strings.Join(lines, "\n")
}

// renderCombat draws the tactical-scope combat screen shown for the
// duration of ActiveRun.Phase == PhaseCombat, replacing the escape-burn
// screen. Autocannons track and damage the pirate continuously; mass drivers
// and pulse lasers keep the manual firing controls. Weaponless ships see only
// the escape-burn option.
func (g *Game) renderCombat(st *sim.State, run *sim.ActiveRun) string {
	cs := run.Combat
	if cs == nil {
		return g.renderBelt()
	}
	manualArmed := sim.HasManualWeapon(st)
	autocannonDPS := sim.AutocannonDamagePerSecond(st, g.content)

	titleText := fmt.Sprintf("%s %s — BOUNTY %s %d — EST. ODDS %d%%",
		theme.Glyph("skull", st.Settings.ASCIISafe), cs.PirateName,
		theme.Glyph("credit", st.Settings.ASCIISafe), cs.Bounty, cs.OddsPct)
	title := theme.Red.Bold(true).Render(titleText)

	scope := theme.Panel("TACTICAL SCOPE", combatScopeW+2, combatScopeH+2,
		g.renderCombatScope(cs), theme.Accent(theme.HueAmber))

	hullPct := sim.HullPct(st, g.content)
	pirateHullPct := 0.0
	if cs.PirateMaxHull > 0 {
		pirateHullPct = clampF(100*cs.PirateHull/cs.PirateMaxHull, 0, 100)
	}
	heatCap := sim.HeatCapacity(st, g.content)
	heatPct := 0.0
	if heatCap > 0 {
		heatPct = clampF(100*cs.Heat/heatCap, 0, 100)
	}
	heatLine := fmt.Sprintf("HEAT %s %3.0f%%", theme.RampBar(heatPct, 16, true), heatPct)
	if cs.LockRemaining > 0 {
		heatLine = theme.Red.Render(fmt.Sprintf("HEAT %s LOCKED %.1fs", theme.RampBar(heatPct, 16, true), cs.LockRemaining))
	}

	statLines := []string{
		theme.Bright.Render("YOU"),
		g.renderHullLine(st, hullPct, 16),
	}
	if manualArmed {
		statLines = append(statLines, heatLine)
	}
	if autocannonDPS > 0 {
		statLines = append(statLines, theme.Amber.Render(fmt.Sprintf("AUTOCANNON %.1f DPS", autocannonDPS)))
	}
	statLines = append(statLines,
		"",
		theme.Red.Render(cs.PirateName),
		fmt.Sprintf("HULL   %s %3.0f%%", theme.HullBar(pirateHullPct, 16), pirateHullPct),
		theme.DimStyle.Render(fmt.Sprintf("RANGE ~%.0fm", 120+cs.Range*680)),
	)
	rightBody := strings.Join(statLines, "\n")
	scopeRow := lipgloss.JoinHorizontal(lipgloss.Top, scope, "  "+strings.ReplaceAll(rightBody, "\n", "\n  "))

	lines := []string{title, "", scopeRow, ""}

	if autocannonDPS > 0 {
		lines = append(lines, theme.Amber.Render(fmt.Sprintf("AUTOCANNON ONLINE — CONSTANT DAMAGE %.1f DPS", autocannonDPS)))
	}
	if manualArmed {
		solutionText := fmt.Sprintf("SOLUTION %3.0f%%", cs.Solution*100)
		fireLabel := "[F] FIRE"
		switch {
		case cs.LockRemaining > 0:
			fireLabel = theme.DimStyle.Render(fireLabel + " — LOCKED")
		case !st.Settings.ReducedMotion && g.tickCount%2 == 0:
			fireLabel = theme.Red.Bold(true).Render(fireLabel)
		default:
			fireLabel = theme.Red.Render(fireLabel)
		}
		fireLine := fireLabel + "   " + theme.TxtStyle.Render(solutionText)
		lines = append(lines, fireLine)
		g.hitBodyLine(len(lines)-1, 1, fireLine, "btn:combat:fire", nil)
	} else if autocannonDPS <= 0 {
		banner := "NO WEAPONS — ESCAPE BURN RUNNING"
		if !cs.EscapeStarted {
			banner = "NO WEAPONS — PRESS B TO RUN"
		}
		if !st.Settings.ReducedMotion && g.tickCount%2 == 0 {
			lines = append(lines, theme.Amber.Bold(true).Render(banner))
		} else {
			lines = append(lines, theme.Amber.Render(banner))
		}
	}
	lines = append(lines, "")

	if len(cs.Log) > 0 {
		n := min(3, len(cs.Log))
		for i := 0; i < n; i++ {
			lines = append(lines, theme.DimStyle.Render("> "+cs.Log[i]))
		}
		lines = append(lines, "")
	}

	if cs.EscapeStarted {
		pct := 0.0
		if run.EscapeSecondsRequired > 0 {
			pct = clampF(run.EscapeSecondsElapsed/run.EscapeSecondsRequired*100, 0, 100)
		}
		remaining := run.EscapeSecondsRequired - run.EscapeSecondsElapsed
		if remaining < 0 {
			remaining = 0
		}
		lines = append(lines, fmt.Sprintf("%s ESCAPE VECTOR %s %s (%.0fs left)",
			theme.Glyph("drill", st.Settings.ASCIISafe),
			theme.DrillBar(pct, 26),
			theme.DrillStyle(pct).Render(fmt.Sprintf("%3.0f%%", pct)), remaining))
		lines = append(lines, "")
	}

	depleted := false
	if ast, _ := sim.FindAsteroid(st, run.AsteroidID); ast != nil {
		depleted = sim.RunDepleted(st, g.content, ast, run)
	}
	var escBtn string
	switch {
	case cs.EscapeStarted && depleted:
		escBtn = theme.DimStyle.Render("ESCAPE BURN RUNNING (DEPART)")
	case cs.EscapeStarted:
		escBtn = theme.DimStyle.Render("ESCAPE BURN RUNNING (BAIL)")
	case depleted:
		escBtn = theme.Button("ENTER", "DEPART", "", true, theme.HueGreen)
	default:
		escBtn = theme.Button("B", "BAIL", "", true, theme.HueAmber)
	}
	lines = append(lines, escBtn)
	if !cs.EscapeStarted {
		g.hitBodyLine(len(lines)-1, 1, escBtn, "btn:combat:escape", nil)
	}

	hint := "B BAIL"
	if manualArmed {
		hint = "F FIRE · B BAIL"
	}
	if cs.EscapeStarted {
		hint = "ESCAPE BURN RUNNING"
		if manualArmed {
			hint = "F FIRE · ESCAPE BURN RUNNING"
		}
	}
	return strings.Join(lines, "\n") + "\n" + g.renderKeybar(hint)
}
