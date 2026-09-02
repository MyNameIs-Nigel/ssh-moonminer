package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

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
	panelH := g.bodyHeight()
	panelW := g.contentWidth()
	panelBodyH := panelH - 2
	innerW := panelW - 4
	lines := g.logLines(&st, innerW)
	clampScroll(&g.logScroll, len(lines), panelBodyH)
	viewport := scrollWindowLines(lines, g.logScroll, panelBodyH)
	body := strings.Join(viewport, "\n")
	hint := "↑/↓ SCROLL · ESC BACK TO CHART"
	return theme.Panel("SHIP'S LOG", panelW, panelH, body, accent) + "\n" + g.renderKeybar(hint)
}

func (g *Game) logLines(st *sim.State, innerW int) []string {
	stats := []string{
		fmt.Sprintf("RUNS: %s (departed %s / bailed %s / tribute %s / underfire %s / lost %s)",
			theme.Bright.Render(fmt.Sprintf("%d", st.Stats.RunsTotal)),
			theme.Green.Render(fmt.Sprintf("%d", st.Stats.RunsDeparted)),
			theme.Amber.Render(fmt.Sprintf("%d", st.Stats.RunsBailed)),
			theme.Amber.Render(fmt.Sprintf("%d", st.Stats.RunsTributePaid)),
			theme.Red.Render(fmt.Sprintf("%d", st.Stats.RunsEscapedUnderFire)),
			theme.Red.Render(fmt.Sprintf("%d", st.Stats.ShipsLost))),
		fmt.Sprintf("EARNED: %s  SOLD: %s  SPENT: %s  LEGENDARIES: %s",
			theme.Gold.Render(fmt.Sprintf("%d", st.Stats.CreditsEarned)),
			theme.Green.Render(fmt.Sprintf("%d", st.Stats.CargoValueSold)),
			theme.Red.Render(fmt.Sprintf("%d", st.Stats.CreditsSpent)),
			theme.Violet.Render(fmt.Sprintf("%d", st.Stats.LegendariesMined))),
	}
	lines := []string{
		theme.Cyan.Render("SERVICE RECORD"),
		strings.Join(stats, "\n"),
		"",
		theme.Bright.Render("RECENT RUNS"),
	}
	for _, r := range st.RunLog {
		lines = append(lines, g.formatRunLogRecord(st, r, innerW)...)
	}
	return lines
}

func (g *Game) formatRunLogRecord(st *sim.State, r sim.RunRecord, innerW int) []string {
	outcome := strings.ToUpper(r.Outcome)
	amount := r.CargoValueRecovered
	sign := "+"
	if r.CargoValueSold > 0 {
		amount = r.CargoValueSold
		sign = "$"
	} else if r.Outcome == string(sim.OutcomeShipLost) {
		amount = r.CargoValueLost
		sign = "-"
	}
	if st.Settings.WrapLongText {
		plain := fmt.Sprintf("%s %s %s %s%d", r.World, r.Asteroid, outcome, sign, amount)
		wrapped := wrapChartText(plain, innerW)
		out := make([]string, len(wrapped))
		for i, line := range wrapped {
			out[i] = g.styleRunLogPlainLine(line, r, outcome, sign, amount)
		}
		if r.CargoValueJettisoned > 0 {
			out = append(out, theme.Amber.Render(fmt.Sprintf("  TRIBUTE — dropped %d, retained %d", r.CargoValueJettisoned, r.CargoValueRecovered)))
		}
		return out
	}
	line := fmt.Sprintf("%s %s %s %s%d",
		theme.DimStyle.Render(r.World), theme.TxtStyle.Render(r.Asteroid),
		theme.OutcomeStyle(r.Outcome).Render(outcome), sign, amount)
	out := []string{ansi.Truncate(line, innerW, "…")}
	if r.CargoValueJettisoned > 0 {
		out = append(out, theme.Amber.Render(fmt.Sprintf("  TRIBUTE — dropped %d, retained %d", r.CargoValueJettisoned, r.CargoValueRecovered)))
	}
	return out
}

func (g *Game) styleRunLogPlainLine(plain string, r sim.RunRecord, outcome, sign string, amount int) string {
	worldPrefix := r.World + " "
	if strings.HasPrefix(plain, worldPrefix) {
		rest := strings.TrimPrefix(plain, worldPrefix)
		return theme.DimStyle.Render(r.World) + " " + g.styleRunLogRest(rest, r, outcome, sign, amount)
	}
	return g.styleRunLogRest(plain, r, outcome, sign, amount)
}

func (g *Game) styleRunLogRest(rest string, r sim.RunRecord, outcome, sign string, amount int) string {
	tail := fmt.Sprintf("%s %s%d", outcome, sign, amount)
	if idx := strings.LastIndex(rest, tail); idx >= 0 {
		asteroid := strings.TrimSpace(rest[:idx])
		return theme.TxtStyle.Render(asteroid) + " " +
			theme.OutcomeStyle(r.Outcome).Render(outcome) + " " +
			fmt.Sprintf("%s%d", sign, amount)
	}
	return theme.TxtStyle.Render(rest)
}

var onboardPages = []string{
	theme.TxtStyle.Render("Welcome, pilot. Mine the belts, bank credits, upgrade your ship."),
	theme.TxtStyle.Render("Chart → Belt → Mine → Escape → Summary. You must leave manually."),
	theme.TxtStyle.Render("B bails early (yellow); Enter departs once depleted (green)."),
	theme.TxtStyle.Render("Pirates may demand tribute or attack — fleeing takes time, and"),
	theme.TxtStyle.Render("hull loss can destroy your ship. Ships are lives — mine wisely."),
}
