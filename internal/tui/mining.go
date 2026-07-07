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
	switch k {
	case " ":
		g.overdriveOn = true
		g.overdriveUntil = g.now*1000 + 250
		_, _ = g.sess.SetOverdrive(true)
	case "b", "esc":
		snap, out, err := g.sess.Bail(g.now)
		if err != nil {
			return g.refreshSnap(snap, err)
		}
		if out != nil {
			g.lastOutcome = out
			g.scr = scrSummary
		}
		return g.refreshSnap(snap, nil)
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
	title := fmt.Sprintf("%s DRILLING — %s (%s %s)", theme.Glyph("diamond", st.Settings.ASCIISafe),
		name, g.content.Tiers.Glyphs[tier], g.content.Tiers.Labels[tier])
	if run.Pirate >= 75 {
		title = theme.Red.Render(theme.Glyph("skull", st.Settings.ASCIISafe)+" CONTACT IMMINENT — ") + theme.Amber.Render(title)
	} else {
		title = theme.Amber.Render(title)
	}
	fuelPct := st.Fuel / sim.TankSize(&st, g.content) * 100
	lines := []string{
		title,
		"",
		fmt.Sprintf("%s DRILL  %s %s",
			theme.Glyph("drill", st.Settings.ASCIISafe),
			theme.DrillBar(run.Drill, 30),
			theme.DrillStyle(run.Drill).Render(fmt.Sprintf("%3.0f%%", run.Drill))),
		fmt.Sprintf("%s FUEL   %s %s",
			theme.Glyph("fuel", st.Settings.ASCIISafe),
			theme.FuelBar(fuelPct, 30),
			theme.FuelStyle(fuelPct).Render(fmt.Sprintf("%3.0f%%", fuelPct))),
		fmt.Sprintf("%s PIRATE %s %s",
			theme.Glyph("skull", st.Settings.ASCIISafe),
			theme.RampBar(run.Pirate, 30, true),
			theme.GaugeStyle(run.Pirate, true).Render(fmt.Sprintf("%3.0f%%", run.Pirate))),
		"",
		fmt.Sprintf("CARGO VALUE  %s %s of %s %s",
			theme.Glyph("credit", st.Settings.ASCIISafe), theme.Gold.Render(fmt.Sprintf("%d", run.Yield)),
			theme.Glyph("credit", st.Settings.ASCIISafe), theme.TxtStyle.Render(fmt.Sprintf("%d", value))),
		"",
	}
	od := ""
	if g.overdriveOn || run.Overdrive {
		if st.Settings.ReducedMotion {
			od = theme.Amber.Render(" ▮▮ OVERDRIVE")
		} else if g.tickCount%2 == 0 {
			od = theme.Amber.Render(" ▮▮ OVERDRIVE")
		} else {
			od = theme.Gold.Render(" ▮▮ OVERDRIVE")
		}
	}
	overdriveLabel := "OVERDRIVE" + od
	overdriveLine := theme.Button("SPACE", overdriveLabel, "", true, theme.HueAmber)
	lines = append(lines, overdriveLine)
	g.hitBodyLine(len(lines)-1, 1, overdriveLine, "btn:overdrive", nil)

	bailLine := theme.Button("B", "BAIL & BANK CARGO", "", true, theme.HueRed)
	lines = append(lines, bailLine)
	g.hitBodyLine(len(lines)-1, 1, bailLine, "btn:bail", nil)

	hint := "SPACE OVERDRIVE · B BAIL"
	if fuelPct < 15 {
		hint = theme.Red.Render("▼ FUEL CRITICAL — BAIL NOW?")
	}
	return strings.Join(lines, "\n") + "\n" + g.renderKeybar(hint)
}
