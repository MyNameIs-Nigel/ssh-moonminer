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

// renderCombatIntro draws the "COMBAT MODE ENGAGED" flood card inside the
// shared frame shell — the death screen's renderDeathFinal pattern in amber
// instead of red, held for combatIntroHold before the tactical scope takes
// over.
func (g *Game) renderCombatIntro() string {
	layout := g.frameLayout()
	bgStyle := lipgloss.NewStyle().Background(lipgloss.Color(combatBg))
	titleStyle := bgStyle.Foreground(lipgloss.Color(combatFg)).Bold(true)
	subStyle := bgStyle.Foreground(lipgloss.Color(combatDim))

	msg := titleStyle.Render("⚠ COMBAT MODE ENGAGED ⚠")
	if run := g.snap.State.Run; run != nil && run.Combat != nil {
		st := g.snap.State
		sub := fmt.Sprintf("%s — BOUNTY %s %d", run.Combat.PirateName,
			theme.Glyph("credit", st.Settings.ASCIISafe), run.Combat.Bounty)
		msg += "\n" + subStyle.Render(sub)
	}

	chrome := strings.Join([]string{
		bgStyle.Render(strings.Repeat(" ", layout.Width)),
		bgStyle.Render(strings.Repeat(" ", layout.Width)),
		bgStyle.Render(strings.Repeat(" ", layout.Width)),
	}, "\n")

	body := lipgloss.Place(layout.Width, layout.BodyH, lipgloss.Center, lipgloss.Center, msg,
		lipgloss.WithWhitespaceStyle(bgStyle))

	hint := subStyle.Render("brace for impact")
	keybar := lipgloss.Place(layout.Width, keybarH, lipgloss.Center, lipgloss.Bottom, hint,
		lipgloss.WithWhitespaceStyle(bgStyle))

	return chrome + "\n" + body + "\n" + keybar
}

func (g *Game) keyCombat(m tea.KeyPressMsg) []tea.Cmd {
	k := m.String()
	run := g.snap.State.Run
	if run == nil || run.Phase != sim.PhaseCombat {
		return nil
	}
	switch k {
	case "f":
		if !sim.HasPulseLaser(&g.snap.State) {
			return nil
		}
		snap, err := g.sess.FireWeapons(g.now)
		return g.refreshSnap(snap, err)
	case "g":
		if !sim.HasMissileLauncher(&g.snap.State) {
			return nil
		}
		snap, err := g.sess.FireMissile(g.now)
		return g.refreshSnap(snap, err)
	case "b", "esc", "enter":
		snap, err := g.sess.CombatEscape(g.now)
		return g.refreshSnap(snap, err)
	}
	return nil
}

// renderCombatScope draws the tactical scope: a fixed player anchor at the
// firing arc's center, the arc itself as a highlighted band, and the
// pirate blip plotted from the sim-authoritative Bearing/Range each frame.
func (g *Game) renderCombatScope(cs *sim.CombatState, w, h int) string {
	if w < 1 || h < 1 {
		return ""
	}
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
			bearing := float64(col) / float64(max(1, w-1))
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
// screen. Autocannons track continuously; Pulse Lasers use F and guided
// Missile Launchers use G. Weaponless ships see only the escape-burn option.
func (g *Game) renderCombat(st *sim.State, run *sim.ActiveRun) string {
	cs := run.Combat
	if cs == nil {
		return g.renderBelt()
	}
	pulseArmed := sim.HasPulseLaser(st)
	missileFitted := sim.HasMissileLauncher(st)
	missileAmmo := sim.MissileAmmo(st)
	missileCapacity := sim.MissileCapacityFor(st, g.content, st.ActiveShipID)
	autocannonDPS := sim.AutocannonDamagePerSecond(st, g.content)

	titleText := fmt.Sprintf("%s %s — BOUNTY %s %d — EST. ODDS %d%%",
		theme.Glyph("skull", st.Settings.ASCIISafe), cs.PirateName,
		theme.Glyph("credit", st.Settings.ASCIISafe), cs.Bounty, cs.OddsPct)
	title := theme.Red.Bold(true).Render(titleText)

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

	leftLines := []string{title, "", theme.Bright.Render("YOU"), g.renderHullLine(st, hullPct, 16)}
	if pulseArmed {
		leftLines = append(leftLines, heatLine)
	}
	if autocannonDPS > 0 {
		leftLines = append(leftLines, theme.Amber.Render(fmt.Sprintf("AUTOCANNON %.1f DPS", autocannonDPS)))
	}
	if missileFitted {
		leftLines = append(leftLines, theme.Red.Render(fmt.Sprintf("MISSILES %d/%d", missileAmmo, missileCapacity)))
	}
	leftLines = append(leftLines,
		"",
		theme.Red.Render(cs.PirateName),
		fmt.Sprintf("HULL   %s %3.0f%%", theme.HullBar(pirateHullPct, 16), pirateHullPct),
		theme.DimStyle.Render(fmt.Sprintf("RANGE ~%.0fm", 120+cs.Range*680)),
		"",
	)

	if autocannonDPS > 0 {
		leftLines = append(leftLines, theme.Amber.Render(fmt.Sprintf("AUTOCANNON ONLINE — CONSTANT DAMAGE %.1f DPS", autocannonDPS)))
	}
	fireRow := -1
	if pulseArmed {
		solutionText := fmt.Sprintf("SOLUTION %3.0f%%", cs.Solution*100)
		fireLabel := "[F] FIRE PULSE"
		switch {
		case cs.LockRemaining > 0:
			fireLabel = theme.DimStyle.Render(fireLabel + " — LOCKED")
		case !st.Settings.ReducedMotion && g.tickCount%2 == 0:
			fireLabel = theme.Red.Bold(true).Render(fireLabel)
		default:
			fireLabel = theme.Red.Render(fireLabel)
		}
		fireLine := fireLabel + "   " + theme.TxtStyle.Render(solutionText)
		fireRow = len(leftLines)
		leftLines = append(leftLines, fireLine)
	}
	missileRow := -1
	if missileFitted {
		missileLabel := "[G] FIRE MISSILE"
		status := fmt.Sprintf("GUIDED 100%% · %d/%d", missileAmmo, missileCapacity)
		switch {
		case missileAmmo == 0:
			missileLabel = theme.DimStyle.Render(missileLabel + " — RELOAD AT DOCK")
		case cs.MissileCooldown > 0:
			missileLabel = theme.DimStyle.Render(fmt.Sprintf("%s — COOLDOWN %.1fs", missileLabel, cs.MissileCooldown))
		default:
			missileLabel = theme.Red.Render(missileLabel)
		}
		missileLine := missileLabel + "   " + theme.TxtStyle.Render(status)
		missileRow = len(leftLines)
		leftLines = append(leftLines, missileLine)
	}
	if !pulseArmed && !missileFitted && autocannonDPS <= 0 {
		banner := "NO WEAPONS — ESCAPE BURN RUNNING"
		if !cs.EscapeStarted {
			banner = "NO WEAPONS — PRESS B TO RUN"
		}
		if !st.Settings.ReducedMotion && g.tickCount%2 == 0 {
			leftLines = append(leftLines, theme.Amber.Bold(true).Render(banner))
		} else {
			leftLines = append(leftLines, theme.Amber.Render(banner))
		}
	}
	leftLines = append(leftLines, "")

	if len(cs.Log) > 0 {
		n := min(3, len(cs.Log))
		for i := 0; i < n; i++ {
			leftLines = append(leftLines, theme.DimStyle.Render("> "+cs.Log[i]))
		}
		leftLines = append(leftLines, "")
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
		leftLines = append(leftLines, fmt.Sprintf("%s ESCAPE VECTOR %s %s (%.0fs left)",
			theme.Glyph("drill", st.Settings.ASCIISafe),
			theme.DrillBar(pct, 26),
			theme.DrillStyle(pct).Render(fmt.Sprintf("%3.0f%%", pct)), remaining))
		leftLines = append(leftLines, "")
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
	escapeRow := len(leftLines)

	pinned := []string{}
	hits := []miningHit{}
	pinnedRow := 0
	if fireRow >= 0 {
		pinned = append(pinned, leftLines[fireRow])
		hits = append(hits, miningHit{pinned: true, row: pinnedRow, line: leftLines[fireRow], id: "btn:combat:fire", data: nil})
		pinnedRow++
	}
	if missileRow >= 0 {
		pinned = append(pinned, leftLines[missileRow])
		if missileAmmo > 0 && cs.MissileCooldown <= 0 {
			hits = append(hits, miningHit{pinned: true, row: pinnedRow, line: leftLines[missileRow], id: "btn:combat:missile", data: nil})
		}
		pinnedRow++
	}
	pinned = append(pinned, escBtn)
	if !cs.EscapeStarted {
		hits = append(hits, miningHit{pinned: true, row: pinnedRow, line: escBtn, id: "btn:combat:escape", data: nil})
	}
	scrollTop := make([]string, 0, escapeRow)
	for i, line := range leftLines[:escapeRow] {
		if i == fireRow || i == missileRow {
			continue
		}
		scrollTop = append(scrollTop, line)
	}

	_, rightW := g.miningColumnWidths()
	panelH := g.bodyHeight()
	scopeInnerW := max(1, rightW-2)
	scopeInnerH := max(1, panelH-2)
	scope := theme.Panel("TACTICAL SCOPE", rightW, panelH,
		g.renderCombatScope(cs, scopeInnerW, scopeInnerH), theme.Accent(theme.HueAmber))

	hint := "B BAIL"
	if pulseArmed {
		hint = "F PULSE · B BAIL"
	}
	if missileFitted {
		if pulseArmed {
			hint = "F PULSE · G MISSILE · B BAIL"
		} else {
			hint = "G MISSILE · B BAIL"
		}
	}
	if cs.EscapeStarted {
		hint = "ESCAPE BURN RUNNING"
		if pulseArmed && missileFitted {
			hint = "F PULSE · G MISSILE · ESCAPE BURN RUNNING"
		} else if pulseArmed {
			hint = "F PULSE · ESCAPE BURN RUNNING"
		} else if missileFitted {
			hint = "G MISSILE · ESCAPE BURN RUNNING"
		}
	}

	return g.renderMiningRunSplit(scrollTop, pinned, scope, hint, hits, nil)
}
