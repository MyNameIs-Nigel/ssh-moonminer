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
	accent := theme.OutcomeAccent(string(out.Kind))
	body := []string{
		theme.OutcomeStyle(string(out.Kind)).Render(out.Label),
		theme.DimStyle.Render(out.Description),
		"",
		theme.Gold.Render("ORE: ") + theme.TxtStyle.Render(out.Record.Asteroid),
		theme.Green.Render("CARGO BANKED: ") + theme.Gold.Render(fmt.Sprintf("+%s %d", theme.Glyph("credit", st.Settings.ASCIISafe), out.Record.Banked)),
		theme.Cyan.Render(fmt.Sprintf("DRILL: %d%%", out.Record.DrillPct)),
		theme.Gold.Render(fmt.Sprintf("BALANCE: %s %d", theme.Glyph("credit", st.Settings.ASCIISafe), st.Credits)),
	}
	continueBtn := theme.Button("ENTER", "RETURN TO BELT", "", true, theme.HueCyan)
	dockBtn := theme.Button("Q", "DOCK AT PORT", "", true, theme.HueGold)
	body = append(body, "", continueBtn, dockBtn)
	content := theme.Panel("RUN SUMMARY", g.width-4, 16, strings.Join(body, "\n"), accent)
	g.hitPanelLine(8, 1, continueBtn, "btn:summary:continue", nil)
	g.hitPanelLine(9, 1, dockBtn, "btn:summary:dock", nil)
	hint := "[ENTER] RETURN TO BELT · [Q] DOCK AT PORT"
	return content + "\n" + g.renderKeybar(hint)
}
