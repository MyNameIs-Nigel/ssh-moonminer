package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/hitbox"
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
	title := fmt.Sprintf("◇ DRILLING — %s (%s %s)", name, g.content.Tiers.Glyphs[tier], g.content.Tiers.Labels[tier])
	if run.Pirate >= 75 {
		title = theme.Red.Render("☠ CONTACT IMMINENT — ") + title
	}
	fuelPct := st.Fuel / sim.TankSize(&st, g.content) * 100
	lines := []string{
		theme.Bright.Render(title),
		"",
		fmt.Sprintf("⛏ DRILL  %s %3.0f%%", theme.RampBar(run.Drill, 30, false), run.Drill),
		fmt.Sprintf("⛽ FUEL   %s %3.0f%%", theme.RampBar(fuelPct, 30, true), fuelPct),
		fmt.Sprintf("☠ PIRATE %s %3.0f%%", theme.RampBar(run.Pirate, 30, true), run.Pirate),
		"",
		fmt.Sprintf("CARGO VALUE  %s %d of %s %d", theme.Glyph("credit", st.Settings.ASCIISafe), run.Yield,
			theme.Glyph("credit", st.Settings.ASCIISafe), value),
		"",
	}
	od := ""
	if g.overdriveOn || run.Overdrive {
		od = theme.Amber.Render(" ▮▮ OVERDRIVE")
	}
	lines = append(lines,
		"[SPACE] OVERDRIVE"+od,
		"[B] BAIL & BANK CARGO",
	)
	g.hits.Add(hitbox.Box{Y: 8, X: 1, W: 20, H: 1, ID: "btn:overdrive"})
	g.hits.Add(hitbox.Box{Y: 9, X: 1, W: 22, H: 1, ID: "btn:bail"})
	hint := "SPACE OVERDRIVE · B BAIL"
	if fuelPct < 15 {
		hint = theme.Red.Render("▼ FUEL CRITICAL — BAIL NOW?")
	}
	return strings.Join(lines, "\n") + "\n" + g.renderKeybar(hint)
}
