package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/theme"
)

func (g *Game) keyMining(m tea.KeyPressMsg) []tea.Cmd {
	k := m.String()
	run := g.snap.State.Run
	if run == nil {
		return nil
	}
	switch run.Phase {
	case sim.PhaseMining:
		switch k {
		case "b", "esc", "enter":
			snap, err := g.sess.BailOrDepart(g.now)
			return g.refreshSnap(snap, err)
		}
		if isSkillCheckKey(k) {
			if run.SkillCheck == nil {
				return nil
			}
			snap, err := g.sess.AttemptSkillCheck(g.now)
			return g.refreshSnap(snap, err)
		}
	case sim.PhaseTribute:
		switch k {
		case "d":
			snap, err := g.sess.AcceptTribute(g.now)
			return g.refreshSnap(snap, err)
		case "r", "esc":
			snap, err := g.sess.RefuseTribute(g.now)
			return g.refreshSnap(snap, err)
		}
	case sim.PhaseEscaping:
		// No menu actions are accepted while fleeing — the ship either
		// escapes or dies.
	}
	return nil
}

// Bubble Tea identifies physical space-bar presses as "space". Keep the
// literal-space form too because the shipyard mouse hitbox synthesizes it.
func isSkillCheckKey(k string) bool { return k == "space" || k == " " }

func (g *Game) renderMining() string {
	st := g.snap.State
	run := st.Run
	if run == nil {
		return g.renderBelt()
	}
	ast, _ := sim.FindAsteroid(&st, run.AsteroidID)
	name := "???"
	tier := 0
	value := 0
	if ast != nil {
		name, tier, value = ast.Name, ast.Tier, ast.Value
	}

	switch run.Phase {
	case sim.PhaseTribute:
		return g.renderTribute(&st, run, name, tier, value)
	case sim.PhaseEscaping:
		return g.renderEscape(&st, run, name, tier)
	default:
		return g.renderDrilling(&st, run, ast, name, tier, value)
	}
}

func (g *Game) renderEventBadge(st *sim.State, run *sim.ActiveRun) string {
	if run.ActiveEvent == nil {
		return ""
	}
	switch run.ActiveEvent.Kind {
	case sim.EventPowerOutage:
		return theme.DimStyle.Render("░░ POWER OUTAGE — SYSTEMS DARK ░░")
	case sim.EventRadarBlackout:
		return theme.Red.Render("▓▓ RADAR BLACKOUT — SIGNAL LOST ▓▓")
	case sim.EventLifeSupportFailure:
		if run.LifeSupportBreached {
			return theme.Red.Render("‼ LIFE SUPPORT FAILURE — LEAVE NOW ‼")
		}
		return theme.Amber.Render(fmt.Sprintf("⚠ LIFE SUPPORT ALARM — %.0fs", run.ActiveEvent.Remaining))
	case sim.EventCargoShift:
		return theme.Amber.Render("▦ CARGO SHIFT — HOLD UNSTABLE, ESCAPE SLOWER")
	case sim.EventReactorSurge:
		return theme.Amber.Render("☀ REACTOR SURGE — FUEL BURN SPIKING")
	}
	return ""
}

func (g *Game) renderDrilling(st *sim.State, run *sim.ActiveRun, ast *sim.Asteroid, name string, tier, value int) string {
	// "Resource left" tracks whichever cap ends the run sooner — the rock's
	// own volume, or the ship's cargo hold filling up first — so the bar
	// always reaches 0% exactly when DEPART goes green.
	resourcePct := 0.0
	if ast != nil && ast.Volume > 0 {
		cap_ := sim.RunMiningCapacity(st, g.content, ast, run)
		if cap_ > 0 {
			resourcePct = clampF(100*(1-sim.RunExtractedUnits(run)/cap_), 0, 100)
		}
	}
	depleted := sim.RunDepleted(st, g.content, ast, run)

	title := fmt.Sprintf("%s MINING SITE — %s (%s %s)", theme.Glyph("diamond", st.Settings.ASCIISafe),
		name, g.content.Tiers.Glyphs[tier], g.content.Tiers.Labels[tier])
	title = theme.Amber.Render(title)

	fuelAmount := sim.FuelAmount(st, g.content)
	fuelPct := fuelAmount / sim.TankSize(st, g.content) * 100
	hullPct := sim.HullPct(st, g.content)
	cargoUnits := st.CargoUnits + sim.RunHeldUnits(run)
	cargoCapacity := sim.CargoCapacityUnits(st, g.content)

	lines := []string{title}
	if badge := g.renderEventBadge(st, run); badge != "" {
		lines = append(lines, badge)
	}
	if run.EMPActive {
		lines = append(lines, theme.Cyan.Render(fmt.Sprintf("EMP LAUNCHER DEPLOYED — PIRATES STALLED %.0fs", run.EMPRemaining)))
	}
	lines = append(lines,
		"",
		fmt.Sprintf("%s RESOURCE LEFT %s %s",
			theme.Glyph("drill", st.Settings.ASCIISafe),
			theme.DrillBar(resourcePct, 26),
			theme.TxtStyle.Render(fmt.Sprintf("%3.0f%%", resourcePct))),
		g.renderHullLine(st, hullPct, 26),
		fmt.Sprintf("%s FUEL         %s %s",
			theme.Glyph("fuel", st.Settings.ASCIISafe),
			theme.FuelBar(fuelPct, 26),
			theme.FuelStyle(fuelPct).Render(fmt.Sprintf("%.0f/%.0f %3.0f%%", fuelAmount, sim.TankSize(st, g.content), fuelPct))),
		"",
		fmt.Sprintf("CARGO %.0f/%.0f  VALUE IN HOLD  %s %s of %s %s (not sold)",
			cargoUnits, cargoCapacity,
			theme.Glyph("credit", st.Settings.ASCIISafe), theme.Gold.Render(fmt.Sprintf("%d", run.CargoValue)),
			theme.Glyph("credit", st.Settings.ASCIISafe), theme.TxtStyle.Render(fmt.Sprintf("%d", value))),
		"",
	)

	if sc := run.SkillCheck; sc != nil {
		remainingPct := clampF(100*(1-sc.Elapsed/sc.Window), 0, 100)
		remainingSecs := sc.Window - sc.Elapsed
		if remainingSecs < 0 {
			remainingSecs = 0
		}
		gauge := theme.RampBar(remainingPct, 26, false)
		label := "[SPACE] STABILIZE DRILL"
		if !st.Settings.ReducedMotion && g.tickCount%2 == 0 {
			label = theme.Gold.Bold(true).Render(label)
		} else {
			label = theme.Gold.Render(label)
		}
		gaugeLine := fmt.Sprintf("%s %s %s %s",
			theme.Glyph("gear", st.Settings.ASCIISafe), gauge,
			theme.GaugeStyle(remainingPct, false).Render(fmt.Sprintf("%.1fs", remainingSecs)), label)
		lines = append(lines, gaugeLine)
		g.hitBodyLine(len(lines)-1, 1, gaugeLine, "btn:skillcheck", nil)
		lines = append(lines, "")
	}

	var actionLine string
	if depleted {
		actionLine = theme.Button("ENTER", "DEPART", "", true, theme.HueGreen)
	} else if st.Settings.ReducedMotion {
		actionLine = theme.Amber.Render("[B] BAIL !")
	} else if g.tickCount%2 == 0 {
		actionLine = theme.Amber.Render("[B] BAIL")
	} else {
		actionLine = theme.Gold.Render("[B] BAIL")
	}
	lines = append(lines, actionLine)
	g.hitBodyLine(len(lines)-1, 1, actionLine, "btn:bail", nil)

	hint := "B BAIL"
	if depleted {
		hint = "ENTER DEPART"
	}
	if run.SkillCheck != nil {
		hint = "SPACE STABILIZE · " + hint
	}
	if fuelPct <= 0 {
		hint = theme.Red.Render("▼ TANKS DRY — DRILL PAUSED, DECIDE NOW")
	}

	leftBody := strings.Join(lines, "\n")
	rightBody := g.renderPirateRadar(run)
	body := lipgloss.JoinHorizontal(lipgloss.Bottom, leftBody, "  "+strings.ReplaceAll(rightBody, "\n", "\n  "))
	return g.renderBottomKeybar(body, hint)
}

const pirateRadarW = 16
const pirateRadarH = 6

// renderPirateRadar draws the bottom-right mining-screen radar scope: a
// fixed player anchor and a red pirate blip that approaches from a
// cosmetic, per-run fixed bearing as PirateDistance closes. The fuzzed ETA
// range above it is the only arrival-time information ever shown — the
// true PirateDistance/rate never appear as an exact number.
func (g *Game) renderPirateRadar(run *sim.ActiveRun) string {
	blackout := run.ActiveEvent != nil && run.ActiveEvent.Kind == sim.EventRadarBlackout
	etaText := fmt.Sprintf("PIRATE ETA ~%.0f-%.0fs", run.PirateETAMin, run.PirateETAMax)
	if blackout {
		etaText = "CONTACT LOST — NO ETA"
	}
	if run.EMPActive {
		etaText = fmt.Sprintf("EMP DELAY ~%.0fs", run.EMPRemaining)
	}
	etaLine := theme.GaugeStyle(100-run.PirateDistance, true).Render(etaText)

	w, h := pirateRadarW, pirateRadarH
	anchorCol, anchorRow := w/2, h-1
	pirateCol := clampInt(int(run.PirateBearing*float64(w-1)), 0, w-1)
	distPct := clampF(run.PirateDistance, 0, 100) / 100
	pirateRow := clampInt(int((1-distPct)*float64(h-2)), 0, h-2)

	lines := make([]string, 0, h+1)
	lines = append(lines, etaLine)
	for r := 0; r < h; r++ {
		cells := make([]string, w)
		for c := 0; c < w; c++ {
			cells[c] = theme.DimStyle.Render("·")
		}
		if r == pirateRow {
			cells[pirateCol] = theme.Red.Render("●")
		}
		if r == anchorRow {
			cells[anchorCol] = theme.Cyan.Render("▲")
		}
		lines = append(lines, strings.Join(cells, ""))
	}
	return strings.Join(lines, "\n")
}

func (g *Game) renderTribute(st *sim.State, run *sim.ActiveRun, name string, tier, value int) string {
	title := theme.Red.Render(theme.Glyph("skull", st.Settings.ASCIISafe) + " PIRATE TRANSMISSION")
	totalCargo := st.CargoValue + run.CargoValue
	demand := int(float64(totalCargo) * g.content.Mining.TributeDemandPct)
	if demand > totalCargo {
		demand = totalCargo
	}
	remaining := g.content.Mining.TributeDecisionSeconds - run.TributeSecondsElapsed
	if remaining < 0 {
		remaining = 0
	}
	lines := []string{
		title,
		"",
		theme.TxtStyle.Render(fmt.Sprintf(`"Drop %s %d from all cargo aboard or we open your hull."`,
			theme.Glyph("credit", st.Settings.ASCIISafe), demand)),
		"",
		fmt.Sprintf("ASTEROID  %s (%s %s)", theme.Gold.Render(name), g.content.Tiers.Glyphs[tier], g.content.Tiers.Labels[tier]),
		fmt.Sprintf("CARGO ABOARD  %s %d", theme.Glyph("credit", st.Settings.ASCIISafe), totalCargo),
		fmt.Sprintf("RETAIN AFTER DROP  %s %d", theme.Glyph("credit", st.Settings.ASCIISafe), totalCargo-demand),
		theme.DimStyle.Render(fmt.Sprintf("decision timeout: %.0fs", remaining)),
		"",
	}
	dropBtn := theme.Button("D", "DROP CARGO", "", true, theme.HueAmber)
	refuseBtn := theme.Button("R", "REFUSE / RUN", "", true, theme.HueRed)
	lines = append(lines, dropBtn, refuseBtn)
	g.hitBodyLine(len(lines)-2, 1, dropBtn, "btn:tribute:accept", nil)
	g.hitBodyLine(len(lines)-1, 1, refuseBtn, "btn:tribute:refuse", nil)
	hint := "D DROP CARGO · R REFUSE / RUN"
	return g.renderBottomKeybar(strings.Join(lines, "\n"), hint)
}

func (g *Game) renderEscape(st *sim.State, run *sim.ActiveRun, name string, tier int) string {
	title := fmt.Sprintf("%s ESCAPE BURN — %s", theme.Glyph("diamond", st.Settings.ASCIISafe), name)
	if run.UnderAttack {
		title = theme.Glyph("skull", st.Settings.ASCIISafe) + " UNDER FIRE — " + name
		if !st.Settings.ReducedMotion && g.tickCount%2 == 0 {
			title = theme.Red.Bold(true).Render(title)
		} else {
			title = theme.Red.Render(title)
		}
	} else {
		title = theme.Cyan.Render(title)
	}

	pct := 0.0
	if run.EscapeSecondsRequired > 0 {
		pct = clampF(run.EscapeSecondsElapsed/run.EscapeSecondsRequired*100, 0, 100)
	}
	remaining := run.EscapeSecondsRequired - run.EscapeSecondsElapsed
	if remaining < 0 {
		remaining = 0
	}
	fuelAmount := sim.FuelAmount(st, g.content)
	fuelPct := fuelAmount / sim.TankSize(st, g.content) * 100
	hullPct := sim.HullPct(st, g.content)

	lines := []string{title}
	if badge := g.renderEventBadge(st, run); badge != "" {
		lines = append(lines, badge)
	}
	lines = append(lines,
		"",
		fmt.Sprintf("%s ESCAPE VECTOR %s %s (%.0fs left)",
			theme.Glyph("drill", st.Settings.ASCIISafe),
			theme.DrillBar(pct, 26),
			theme.DrillStyle(pct).Render(fmt.Sprintf("%3.0f%%", pct)), remaining),
		g.renderHullLine(st, hullPct, 26),
		fmt.Sprintf("%s FUEL          %s %s",
			theme.Glyph("fuel", st.Settings.ASCIISafe),
			theme.FuelBar(fuelPct, 26),
			theme.FuelStyle(fuelPct).Render(fmt.Sprintf("%.0f/%.0f %3.0f%%", fuelAmount, sim.TankSize(st, g.content), fuelPct))),
		"",
		fmt.Sprintf("CARGO VALUE IN HOLD  %s %s (not sold)",
			theme.Glyph("credit", st.Settings.ASCIISafe), theme.Gold.Render(fmt.Sprintf("%d", run.CargoValue))),
		"",
		theme.DimStyle.Render("Ship is fleeing — no menu actions accepted."),
	)
	hint := "ESCAPING — NO INPUT ACCEPTED"
	if run.UnderAttack {
		hint = theme.Red.Render("UNDER FIRE — HULL FALLING")
	}
	return g.renderBottomKeybar(strings.Join(lines, "\n"), hint)
}

func (g *Game) renderHullLine(st *sim.State, hullPct float64, width int) string {
	hullBar := theme.HullBar(hullPct, width)
	shieldHP, shieldMax, shieldDamaged := sim.ShieldStatus(st, g.content)
	shieldLabel := ""
	if shieldMax > 0 {
		shieldPct := shieldHP / shieldMax * 100
		hullBar = theme.HullShieldBar(hullPct, shieldPct, width)
		shieldLabel = theme.Cyan.Render(fmt.Sprintf(" SHIELD %.0f/%.0f", shieldHP, shieldMax))
		if shieldDamaged {
			shieldLabel += " " + theme.Red.Render("DAMAGED")
		}
	}
	return fmt.Sprintf("%s HULL         %s %s%s",
		theme.Glyph("skull", st.Settings.ASCIISafe), hullBar,
		theme.HullStyle(hullPct).Render(fmt.Sprintf("%3.0f%%", hullPct)), shieldLabel)
}

func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
