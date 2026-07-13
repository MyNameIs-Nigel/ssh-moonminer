package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/theme"
)

func (g *Game) keyChart(k string) []tea.Cmd {
	st := g.snap.State
	n := len(g.content.Worlds)
	switch k {
	case "up", "k":
		g.worldSel = (g.worldSel - 1 + n) % n
	case "down", "j":
		g.worldSel = (g.worldSel + 1) % n
	case "enter", " ":
		if g.openPermitPrompt() {
			return nil
		}
		snap, err := g.sess.Depart(g.now, g.worldSel)
		if err == nil {
			g.scr = scrBelt
			g.rockSel = 0
		}
		return g.refreshSnap(snap, err)
	case "f":
		snap, err := g.sess.Refuel(g.now)
		return g.refreshSnap(snap, err)
	case "r":
		snap, err := g.sess.Repair(g.now)
		return g.refreshSnap(snap, err)
	case "c":
		snap, err := g.sess.SellCargo(g.now)
		return g.refreshSnap(snap, err)
	case "s":
		g.scr = scrShipyard
		g.shipyardPane = 0
	case "l":
		g.scr = scrLog
	case "i":
		if sim.InsuranceEligible(&st, g.content) {
			snap, err := g.sess.Insurance(g.now)
			return g.refreshSnap(snap, err)
		}
	case "t":
		if g.overlay == ovTweaks {
			g.overlay = ovNone
		} else {
			g.tweaksSel = 0
			g.overlay = ovTweaks
		}
	case "q":
		return []tea.Cmd{tea.Quit}
	}
	return nil
}

func (g *Game) renderScreen() string {
	switch g.scr {
	case scrChart:
		return g.renderChart()
	case scrShipyard:
		return g.renderShipyard()
	case scrBelt:
		return g.renderBelt()
	case scrMining:
		return g.renderMining()
	case scrSummary:
		return g.renderSummary()
	case scrLog:
		return g.renderLog()
	}
	return ""
}

func (g *Game) renderChart() string {
	st := g.snap.State
	w := g.contentWidth()
	leftW := w / 2
	rightW := w - leftW
	rightX := leftW + 1
	panelH := g.height - 5
	panelBodyH := panelH - 2
	accent := theme.Accent(theme.HueCyan)
	leftLines := []string{}
	for _, system := range g.content.Systems {
		leftLines = append(leftLines, theme.Violet.Render("◇ "+system.Name))
		for i, w := range g.content.Worlds {
			if w.SystemID != system.ID {
				continue
			}
			marker := "  "
			sel := i == g.worldSel
			if sel {
				marker = "▸ "
			}
			lock := sim.RouteLockReason(&st, g.content, i)
			fuel := fmt.Sprintf("%s %d", theme.Glyph("fuel", st.Settings.ASCIISafe), w.TravelFuel)
			if sim.RemainingCargoCapacity(&st, g.content) <= 0 {
				fuel = theme.Red.Render("HOLD FULL — SELL CARGO")
			} else if lock != "" {
				fuel = theme.Red.Render("LOCK " + lock)
			} else if !sim.CanDepart(&st, g.content, i) {
				fuel = theme.Red.Render(fuel)
			} else {
				fuel = theme.Amber.Render(fuel)
			}
			namePart := fmt.Sprintf("%s%s  %s", marker, w.Name, w.Sub)
			line := theme.OptionHC(theme.HueCyan, sel, st.Settings.HighContrast).Render(namePart) + "  " + fuel
			line = ansi.Truncate(line, leftW-2, "…")
			leftLines = append(leftLines, line)
			g.hitPanelLine(len(leftLines)-1, 1, line, fmt.Sprintf("world:%d", i), i)
		}
	}
	leftFooter := []string{}
	if g.worldSel >= 0 && g.worldSel < len(g.content.Worlds) {
		leftFooter = g.chartFooterLines(g.content.Worlds[g.worldSel].Desc, leftW-2, panelBodyH-len(leftLines))
	}
	leftBody := strings.Join(bottomAlignPanelLines(leftLines, leftFooter, panelBodyH), "\n")

	refuelCost := fmt.Sprintf("%s %d", theme.Glyph("credit", st.Settings.ASCIISafe), sim.RefuelCost(&st, g.content))
	repairCost := fmt.Sprintf("%s %d", theme.Glyph("credit", st.Settings.ASCIISafe), sim.RepairCost(&st, g.content))
	fuelAmount := sim.FuelAmount(&st, g.content)
	fuelPct := fuelAmount / sim.TankSize(&st, g.content) * 100
	hullPct := sim.HullPct(&st, g.content)
	fuelLabel := "FUEL " + theme.FuelBar(fuelPct, 20) + theme.FuelStyle(fuelPct).Render(fmt.Sprintf(" %.0f/%.0f  %.0f%%", fuelAmount, sim.TankSize(&st, g.content), fuelPct))
	hullLabel := "HULL " + theme.HullBar(hullPct, 20) + theme.HullStyle(hullPct).Render(fmt.Sprintf(" %.0f%%", hullPct))
	shieldLabel := g.renderShieldStatus(&st, 8)

	rightLines := []string{fuelLabel}
	refuelBtn := theme.Button("F", "REFUEL", refuelCost, sim.RefuelCost(&st, g.content) > 0 && st.Credits >= g.content.Port.RefuelPerPoint, theme.HueGreen)
	rightLines = append(rightLines, refuelBtn)
	g.hitPanelLine(1, rightX, refuelBtn, "svc:refuel", nil)

	rightLines = append(rightLines, hullLabel)
	repairBtn := theme.Button("R", "REPAIR — FULL", repairCost, st.Credits >= sim.RepairCost(&st, g.content) && hullPct < 100, theme.HueGreen)
	rightLines = append(rightLines, repairBtn)
	g.hitPanelLine(3, rightX, repairBtn, "svc:repair", nil)

	rightLines = append(rightLines, shieldLabel)
	rightLines = append(rightLines, g.renderDockShieldService(&st))
	cargoLabel := fmt.Sprintf("CARGO %.0f/%.0f  %s %d", st.CargoUnits, sim.CargoCapacityUnits(&st, g.content), theme.Glyph("credit", st.Settings.ASCIISafe), st.CargoValue)
	rightLines = append(rightLines, cargoLabel)
	sellBtn := theme.Button("C", "SELL CARGO", fmt.Sprintf("%s %d", theme.Glyph("credit", st.Settings.ASCIISafe), st.CargoValue), st.CargoValue > 0, theme.HueGold)
	rightLines = append(rightLines, sellBtn)
	g.hitPanelLine(len(rightLines)-1, rightX, sellBtn, "svc:sell", nil)
	if sim.InsuranceEligible(&st, g.content) {
		insBtn := theme.Amber.Render("[I] INSURANCE ADVANCE")
		rightLines = append(rightLines, insBtn)
		g.hitPanelLine(len(rightLines)-1, rightX, insBtn, "svc:insurance", nil)
	}
	shipyardBtn := theme.Button("S", "SHIPYARD", "", true, theme.HueViolet)
	rightLines = append(rightLines, shipyardBtn)
	g.hitPanelLine(len(rightLines)-1, rightX, shipyardBtn, "btn:shipyard", nil)

	logBtn := theme.Button("L", "SHIP'S LOG", "", true, theme.HueCyan)
	rightLines = append(rightLines, logBtn)
	g.hitPanelLine(len(rightLines)-1, rightX, logBtn, "btn:log", nil)

	rightFooter := g.chartFooterLines("Bigger rocks pay more but attract pirates.", rightW-2, panelBodyH-len(rightLines))
	rightBody := strings.Join(bottomAlignPanelLines(rightLines, rightFooter, panelBodyH), "\n")

	chartTitle := "STAR CHART"
	if system := g.content.SystemByID(st.SystemID); system != nil {
		chartTitle += " — " + system.Name
	}
	body := lipgloss.JoinHorizontal(lipgloss.Top,
		theme.Panel(chartTitle, leftW, panelH, leftBody, accent),
		theme.Panel("PORT SERVICES", rightW, panelH, rightBody, accent),
	)
	hint := "↑/↓ SELECT · ENTER FLY · C SELL · S YARD · L LOG · T TWEAKS · ? HELP · Q QUIT"
	return body + "\n" + g.renderKeybar(hint)
}

const doubleClickMS = 400

func (g *Game) updateClick(m tea.MouseClickMsg) []tea.Cmd {
	if b, ok := g.hits.At(m.X, m.Y); ok {
		// Buttons fire on single click.
		switch b.ID {
		case "svc:refuel":
			return g.keyChart("f")
		case "svc:repair":
			return g.keyChart("r")
		case "svc:insurance":
			return g.keyChart("i")
		case "svc:sell":
			return g.keyChart("c")
		case "btn:shipyard":
			return g.keyChart("s")
		case "btn:shipyard:remove":
			return g.keyShipyard("x")
		case "btn:log":
			return g.keyChart("l")
		case "btn:scan":
			return g.keyBelt("s")
		case "btn:bail":
			return g.keyMining(tea.KeyPressMsg{Code: 'b', Text: "b"})
		case "btn:skillcheck":
			return g.keyMining(tea.KeyPressMsg{Code: ' ', Text: " "})
		case "btn:tribute:accept":
			return g.keyMining(tea.KeyPressMsg{Code: 'd', Text: "d"})
		case "btn:tribute:refuse":
			return g.keyMining(tea.KeyPressMsg{Code: 'r', Text: "r"})
		case "btn:summary:continue":
			return g.keySummary("enter")
		case "btn:summary:dock":
			return g.keySummary("q")
		}

		// Lists: single click selects, double click activates.
		now := g.now * 1000
		if b.ID == g.lastClickID && now-g.lastClickAt < doubleClickMS {
			switch {
			case strings.HasPrefix(b.ID, "world:"):
				return g.keyChart("enter")
			case strings.HasPrefix(b.ID, "belt:"):
				return g.keyBelt("enter")
			case strings.HasPrefix(b.ID, "hangar:"):
				return g.keyShipyard("enter")
			case strings.HasPrefix(b.ID, "loadout:"):
				return g.keyShipyard("enter")
			}
		}
		g.lastClickID = b.ID
		g.lastClickAt = now
		switch {
		case strings.HasPrefix(b.ID, "world:"):
			if idx, ok := b.Data.(int); ok {
				g.worldSel = idx
			}
		case strings.HasPrefix(b.ID, "belt:"):
			if idx, ok := b.Data.(int); ok {
				g.rockSel = idx
			}
		case strings.HasPrefix(b.ID, "hangar:"):
			if idx, ok := b.Data.(int); ok {
				g.shipyardHangarSel = idx
				g.shipyardPane = shipyardPaneHangar
			}
		case strings.HasPrefix(b.ID, "loadout:"):
			if idx, ok := b.Data.(int); ok {
				g.shipyardRowSel = idx
				g.shipyardPane = shipyardPaneLoadout
			}
		}
	}
	return nil
}

func (g *Game) updateWheel(m tea.MouseWheelMsg) []tea.Cmd {
	if g.overlay != ovNone {
		return nil
	}
	switch g.scr {
	case scrChart:
		if m.Y > 0 {
			return g.keyChart("up")
		}
		if m.Y < 0 {
			return g.keyChart("down")
		}
	case scrShipyard:
		if m.Y > 0 {
			return g.keyShipyard("up")
		}
		if m.Y < 0 {
			return g.keyShipyard("down")
		}
	case scrBelt:
		if m.Y > 0 {
			return g.keyBelt("up")
		}
		if m.Y < 0 {
			return g.keyBelt("down")
		}
	}
	return nil
}
