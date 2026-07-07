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
	stats := []string{
		fmt.Sprintf("RUNS: %d (clean %d / bail %d / raid %d / strand %d)",
			st.Stats.RunsTotal, st.Stats.RunsClean, st.Stats.RunsBailed,
			st.Stats.RunsRaided, st.Stats.RunsStranded),
		fmt.Sprintf("EARNED: %d  SPENT: %d  LEGENDARIES: %d",
			st.Stats.CreditsEarned, st.Stats.CreditsSpent, st.Stats.LegendariesMined),
	}
	var runs []string
	for i, r := range st.RunLog {
		if i < g.logScroll {
			continue
		}
		runs = append(runs, fmt.Sprintf("%s %s %s +%d (%s)",
			r.World, r.Asteroid, theme.OutcomeStyle(r.Outcome).Render(strings.ToUpper(r.Outcome)), r.Banked, r.Outcome))
		if len(runs) > 8 {
			break
		}
	}
	body := strings.Join(append([]string{theme.Bright.Render("SERVICE RECORD"), strings.Join(stats, "\n"), "", "RECENT RUNS"}, runs...), "\n")
	hint := "ESC BACK TO CHART"
	return theme.Panel("SHIP'S LOG", g.width-4, g.height-6, body) + "\n" + g.renderKeybar(hint)
}

func (g *Game) renderOverlay() string {
	switch g.overlay {
	case ovHelp:
		return theme.Panel("HELP", 50, 10, "↑↓ navigate · ENTER confirm · MOUSE supported\n? close help")
	case ovTweaks:
		return theme.Panel("TWEAKS", 40, 8, "T toggles — aggression/view in save settings")
	case ovKicked:
		return theme.Panel("NOTICE", 50, 5, g.kickReason+"\nPress any key.")
	case ovOnboard:
		pages := []string{
			"Welcome, pilot. Mine the belts, bank credits, upgrade your ship.",
			"Chart → Belt → Mine → Summary. Bail early or drill to 100%.",
			"Keys: arrows, Enter, F/H at port, Space overdrive, B bail.",
		}
		pg := g.onboardPg
		if pg >= len(pages) {
			pg = len(pages) - 1
		}
		return theme.Panel("ONBOARDING", 55, 6, pages[pg]+"\n\nPress any key.")
	}
	return ""
}
