package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/theme"
)

func (g *Game) keySummary(k string) []tea.Cmd {
	switch k {
	case "enter", " ":
		g.scr = scrBelt
		g.lastOutcome = nil
	case "q":
		snap, _ := g.sess.Dock(g.now)
		g.scr = scrChart
		g.lastOutcome = nil
		return g.refreshSnap(snap, nil)
	}
	return nil
}

func (g *Game) renderSummary() string {
	out := g.lastOutcome
	if out == nil {
		return g.renderBelt()
	}
	st := g.snap.State
	body := []string{
		theme.OutcomeStyle(string(out.Kind)).Render(out.Label),
		theme.DimStyle.Render(out.Description),
		"",
		fmt.Sprintf("ORE: %s", out.Record.Asteroid),
		fmt.Sprintf("CARGO BANKED: +%s %d", theme.Glyph("credit", st.Settings.ASCIISafe), out.Record.Banked),
		fmt.Sprintf("DRILL: %d%%", out.Record.DrillPct),
		fmt.Sprintf("BALANCE: %s %d", theme.Glyph("credit", st.Settings.ASCIISafe), st.Credits),
	}
	content := theme.Panel("RUN SUMMARY", g.width-4, 14, strings.Join(body, "\n"))
	hint := "[ENTER] RETURN TO BELT · [Q] DOCK AT PORT"
	return content + "\n" + g.renderKeybar(hint)
}
