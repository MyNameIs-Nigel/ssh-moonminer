package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/theme"
)

func (g *Game) keyLog(k string) []tea.Cmd {
	switch k {
	case "esc", "q", "l":
		g.scr = scrChart
	case "up", "k":
		if g.logScroll > 0 {
			g.logScroll--
		}
	case "down", "j":
		g.logScroll++
	}
	return nil
}

func (g *Game) renderLog() string {
	st := g.snap.State
	accent := theme.Accent(theme.HueCyan)
	stats := []string{
		fmt.Sprintf("RUNS: %s (clean %s / bail %s / raid %s / strand %s)",
			theme.Bright.Render(fmt.Sprintf("%d", st.Stats.RunsTotal)),
			theme.Gold.Render(fmt.Sprintf("%d", st.Stats.RunsClean)),
			theme.Cyan.Render(fmt.Sprintf("%d", st.Stats.RunsBailed)),
			theme.Red.Render(fmt.Sprintf("%d", st.Stats.RunsRaided)),
			theme.Amber.Render(fmt.Sprintf("%d", st.Stats.RunsStranded))),
		fmt.Sprintf("EARNED: %s  SPENT: %s  LEGENDARIES: %s",
			theme.Gold.Render(fmt.Sprintf("%d", st.Stats.CreditsEarned)),
			theme.Red.Render(fmt.Sprintf("%d", st.Stats.CreditsSpent)),
			theme.Violet.Render(fmt.Sprintf("%d", st.Stats.LegendariesMined))),
	}
	var runs []string
	for i, r := range st.RunLog {
		if i < g.logScroll {
			continue
		}
		outcome := theme.OutcomeStyle(r.Outcome).Render(strings.ToUpper(r.Outcome))
		line := fmt.Sprintf("%s %s %s +%s (%s)",
			theme.DimStyle.Render(r.World), theme.TxtStyle.Render(r.Asteroid),
			outcome, theme.Gold.Render(fmt.Sprintf("%d", r.Banked)), r.Outcome)
		runs = append(runs, line)
		if len(runs) > 8 {
			break
		}
	}
	body := strings.Join(append([]string{theme.Cyan.Render("SERVICE RECORD"), strings.Join(stats, "\n"), "", theme.Bright.Render("RECENT RUNS")}, runs...), "\n")
	hint := "ESC BACK TO CHART"
	return theme.Panel("SHIP'S LOG", g.width-4, g.height-6, body, accent) + "\n" + g.renderKeybar(hint)
}

var onboardPages = []string{
	theme.TxtStyle.Render("Welcome, pilot. Mine the belts, bank credits, upgrade your ship."),
	theme.TxtStyle.Render("Chart → Belt → Mine → Summary. Bail early or drill to 100%."),
	theme.TxtStyle.Render("Keys: arrows, Enter, F/H at port, Space overdrive, B bail."),
}
