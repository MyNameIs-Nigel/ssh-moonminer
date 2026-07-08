package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
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
		fmt.Sprintf("RUNS: %s (departed %s / bailed %s / tribute %s / underfire %s / lost %s)",
			theme.Bright.Render(fmt.Sprintf("%d", st.Stats.RunsTotal)),
			theme.Green.Render(fmt.Sprintf("%d", st.Stats.RunsDeparted)),
			theme.Amber.Render(fmt.Sprintf("%d", st.Stats.RunsBailed)),
			theme.Amber.Render(fmt.Sprintf("%d", st.Stats.RunsTributePaid)),
			theme.Red.Render(fmt.Sprintf("%d", st.Stats.RunsEscapedUnderFire)),
			theme.Red.Render(fmt.Sprintf("%d", st.Stats.ShipsLost))),
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
		amount := r.CargoValueRecovered
		sign := "+"
		if r.Outcome == string(sim.OutcomeShipLost) {
			amount = r.CargoValueLost
			sign = "-"
		}
		line := fmt.Sprintf("%s %s %s %s%d",
			theme.DimStyle.Render(r.World), theme.TxtStyle.Render(r.Asteroid),
			outcome, sign, amount)
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
	theme.TxtStyle.Render("Chart → Belt → Mine → Escape → Summary. You must leave manually."),
	theme.TxtStyle.Render("B bails early (yellow); Enter departs once depleted (green)."),
	theme.TxtStyle.Render("Pirates may demand tribute or attack — fleeing takes time, and"),
	theme.TxtStyle.Render("hull loss can destroy your ship. Ships are lives — mine wisely."),
}
