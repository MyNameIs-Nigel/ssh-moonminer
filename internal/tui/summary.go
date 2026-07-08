package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/theme"
)

func (g *Game) keySummary(k string) []tea.Cmd {
	if g.lastOutcome != nil && g.lastOutcome.Kind == sim.OutcomeShipLost {
		// No belt to return to — the ship, its cargo, and its upgrades are
		// gone and the pilot has already respawned at the Sol dock.
		switch k {
		case "enter", " ", "q":
			snap, _ := g.sess.Dock(g.now)
			g.scr = scrChart
			g.lastOutcome = nil
			return g.refreshSnap(snap, nil)
		}
		return nil
	}
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
	if out.Kind == sim.OutcomeShipLost {
		return g.renderShipLostSummary(out)
	}
	st := g.snap.State
	accent := theme.OutcomeAccent(string(out.Kind))
	body := []string{
		theme.OutcomeStyle(string(out.Kind)).Render(out.Label),
		theme.DimStyle.Render(out.Description),
		"",
		theme.Gold.Render("ASTEROID: ") + theme.TxtStyle.Render(out.Record.Asteroid),
		theme.Green.Render("CARGO RECOVERED: ") + theme.Gold.Render(fmt.Sprintf("+%s %d", theme.Glyph("credit", st.Settings.ASCIISafe), out.Record.CargoValueRecovered)),
		theme.Amber.Render(fmt.Sprintf("HULL DELTA: %+d   FUEL DELTA: %+.0f", out.Record.HullDelta, out.Record.FuelDelta)),
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

// renderShipLostSummary is the dock respawn recap shown right after the
// CONNECTION LOST death screen: no verbose recap on the death screen
// itself, so this is where the game explains what was lost.
func (g *Game) renderShipLostSummary(out *sim.RunOutcome) string {
	st := g.snap.State
	accent := theme.OutcomeAccent(string(out.Kind))
	body := []string{
		theme.OutcomeStyle(string(out.Kind)).Render("SHIP LOST"),
		theme.DimStyle.Render(out.Description),
		"",
		theme.Red.Render("LOST: ") + theme.TxtStyle.Render("active ship, installed upgrades, unsold cargo"),
		theme.Red.Render(fmt.Sprintf("CARGO LOST: %s %d", theme.Glyph("credit", st.Settings.ASCIISafe), out.Record.CargoValueLost)),
		theme.Green.Render("KEPT: ") + theme.TxtStyle.Render("banked credits, permits, stations, cosmetics"),
		theme.Gold.Render(fmt.Sprintf("BALANCE: %s %d", theme.Glyph("credit", st.Settings.ASCIISafe), st.Credits)),
		"",
		theme.TxtStyle.Render("A starter Salvage Skiff is waiting at the Sol dock."),
	}
	dockBtn := theme.Button("ENTER", "RESPAWN AT DOCK", "", true, theme.HueGold)
	body = append(body, "", dockBtn)
	content := theme.Panel("RESPAWN — SHIP LOST", g.width-4, 16, strings.Join(body, "\n"), accent)
	g.hitPanelLine(9, 1, dockBtn, "btn:summary:dock", nil)
	hint := "[ENTER] RESPAWN AT DOCK"
	return content + "\n" + g.renderKeybar(hint)
}
