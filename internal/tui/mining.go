package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

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
		case "b", "esc", "enter", " ":
			snap, err := g.sess.BailOrDepart(g.now)
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
	resourcePct := 0.0
	if ast != nil && ast.Volume > 0 {
		resourcePct = clampF(100*(1-run.MinedUnits/float64(ast.Volume)), 0, 100)
	}
	depleted := ast == nil || run.MinedUnits >= float64(ast.Volume)

	title := fmt.Sprintf("%s MINING SITE — %s (%s %s)", theme.Glyph("diamond", st.Settings.ASCIISafe),
		name, g.content.Tiers.Glyphs[tier], g.content.Tiers.Labels[tier])
	title = theme.Amber.Render(title)

	fuelPct := st.Fuel / sim.TankSize(st, g.content) * 100
	pirateLabel := fmt.Sprintf("%3.0f%%", 100-run.PirateDistance)

	lines := []string{title}
	if badge := g.renderEventBadge(st, run); badge != "" {
		lines = append(lines, badge)
	}
	lines = append(lines,
		"",
		fmt.Sprintf("%s RESOURCE LEFT %s %s",
			theme.Glyph("drill", st.Settings.ASCIISafe),
			theme.DrillBar(resourcePct, 26),
			theme.TxtStyle.Render(fmt.Sprintf("%3.0f%%", resourcePct))),
		fmt.Sprintf("%s HULL         %s %s",
			theme.Glyph("skull", st.Settings.ASCIISafe),
			theme.HullBar(float64(st.Hull), 26),
			theme.HullStyle(float64(st.Hull)).Render(fmt.Sprintf("%3d%%", st.Hull))),
		fmt.Sprintf("%s FUEL         %s %s",
			theme.Glyph("fuel", st.Settings.ASCIISafe),
			theme.FuelBar(fuelPct, 26),
			theme.FuelStyle(fuelPct).Render(fmt.Sprintf("%3.0f%%", fuelPct))),
		fmt.Sprintf("%s RADAR        %s %s",
			theme.Glyph("skull", st.Settings.ASCIISafe),
			theme.RampBar(100-run.PirateDistance, 26, true),
			theme.GaugeStyle(100-run.PirateDistance, true).Render(pirateLabel)),
		"",
		fmt.Sprintf("CARGO VALUE IN HOLD  %s %s of %s %s (not sold)",
			theme.Glyph("credit", st.Settings.ASCIISafe), theme.Gold.Render(fmt.Sprintf("%d", run.CargoValue)),
			theme.Glyph("credit", st.Settings.ASCIISafe), theme.TxtStyle.Render(fmt.Sprintf("%d", value))),
		"",
	)

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
	if fuelPct <= 0 {
		hint = theme.Red.Render("▼ TANKS DRY — DRILL PAUSED, DECIDE NOW")
	}
	return strings.Join(lines, "\n") + "\n" + g.renderKeybar(hint)
}

func (g *Game) renderTribute(st *sim.State, run *sim.ActiveRun, name string, tier, value int) string {
	title := theme.Red.Render(theme.Glyph("skull", st.Settings.ASCIISafe) + " PIRATE TRANSMISSION")
	demand := int(float64(run.CargoValue) * g.content.Mining.TributeDemandPct)
	remaining := g.content.Mining.TributeDecisionSeconds - run.TributeSecondsElapsed
	if remaining < 0 {
		remaining = 0
	}
	lines := []string{
		title,
		"",
		theme.TxtStyle.Render(fmt.Sprintf(`"Drop %s %d from this rock or we open your hull."`,
			theme.Glyph("credit", st.Settings.ASCIISafe), demand)),
		"",
		fmt.Sprintf("ASTEROID  %s (%s %s)", theme.Gold.Render(name), g.content.Tiers.Glyphs[tier], g.content.Tiers.Labels[tier]),
		fmt.Sprintf("CARGO ABOARD  %s %d", theme.Glyph("credit", st.Settings.ASCIISafe), run.CargoValue),
		theme.DimStyle.Render(fmt.Sprintf("decision timeout: %.0fs", remaining)),
		"",
	}
	dropBtn := theme.Button("D", "DROP CARGO", "", true, theme.HueAmber)
	refuseBtn := theme.Button("R", "REFUSE / RUN", "", true, theme.HueRed)
	lines = append(lines, dropBtn, refuseBtn)
	g.hitBodyLine(len(lines)-2, 1, dropBtn, "btn:tribute:accept", nil)
	g.hitBodyLine(len(lines)-1, 1, refuseBtn, "btn:tribute:refuse", nil)
	hint := "D DROP CARGO · R REFUSE / RUN"
	return strings.Join(lines, "\n") + "\n" + g.renderKeybar(hint)
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
	fuelPct := st.Fuel / sim.TankSize(st, g.content) * 100

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
		fmt.Sprintf("%s HULL          %s %s",
			theme.Glyph("skull", st.Settings.ASCIISafe),
			theme.HullBar(float64(st.Hull), 26),
			theme.HullStyle(float64(st.Hull)).Render(fmt.Sprintf("%3d%%", st.Hull))),
		fmt.Sprintf("%s FUEL          %s %s",
			theme.Glyph("fuel", st.Settings.ASCIISafe),
			theme.FuelBar(fuelPct, 26),
			theme.FuelStyle(fuelPct).Render(fmt.Sprintf("%3.0f%%", fuelPct))),
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
	return strings.Join(lines, "\n") + "\n" + g.renderKeybar(hint)
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
