package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/theme"
)

const upgradeTrackCount = 5

func (g *Game) keyChart(k string) []tea.Cmd {
	st := g.snap.State
	n := len(g.content.Worlds)
	switch k {
	case "up", "k":
		if g.scr == scrShipyard {
			g.upgradeSel = (g.upgradeSel - 1 + upgradeTrackCount) % upgradeTrackCount
		} else {
			g.worldSel = (g.worldSel - 1 + n) % n
		}
	case "down", "j":
		if g.scr == scrShipyard {
			g.upgradeSel = (g.upgradeSel + 1) % upgradeTrackCount
		} else {
			g.worldSel = (g.worldSel + 1) % n
		}
	case "enter", " ":
		if g.scr == scrShipyard {
			snap, err := g.sess.BuyUpgrade(g.now, sim.UpgradeTrack(g.upgradeSel))
			return g.refreshSnap(snap, err)
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
	case "h":
		snap, err := g.sess.Repair(g.now)
		return g.refreshSnap(snap, err)
	case "u":
		g.scr = scrShipyard
		g.upgradeSel = 0
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
	case "esc":
		if g.scr == scrShipyard {
			g.scr = scrChart
		}
	}
	return nil
}

func (g *Game) renderScreen() string {
	switch g.scr {
	case scrChart, scrShipyard:
		return g.renderChart()
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

func (g *Game) chartAccent() string {
	if g.scr == scrShipyard {
		return theme.Accent(theme.HueViolet)
	}
	return theme.Accent(theme.HueCyan)
}

func (g *Game) renderChart() string {
	st := g.snap.State
	leftW := g.width/2 - 1
	rightW := g.width - leftW - 1
	rightX := leftW + 1
	accent := g.chartAccent()
	leftLines := []string{}
	for i, w := range g.content.Worlds {
		marker := "  "
		sel := i == g.worldSel
		if sel {
			marker = "▸ "
		}
		fuel := fmt.Sprintf("%s %d", theme.Glyph("fuel", st.Settings.ASCIISafe), w.TravelFuel)
		if !sim.CanDepart(&st, g.content, i) {
			fuel = theme.Red.Render(fuel)
		} else {
			fuel = theme.Amber.Render(fuel)
		}
		namePart := fmt.Sprintf("%s%s  %s", marker, w.Name, w.Sub)
		line := theme.OptionHC(theme.HueCyan, sel, st.Settings.HighContrast).Render(namePart) + "  " + fuel
		leftLines = append(leftLines, line)
		g.hitPanelLine(len(leftLines)-1, 1, line, fmt.Sprintf("world:%d", i), i)
	}
	if g.worldSel < len(g.content.Worlds) {
		leftLines = append(leftLines, "", theme.DimStyle.Render(g.content.Worlds[g.worldSel].Desc))
	}
	leftBody := strings.Join(leftLines, "\n")

	refuelCost := fmt.Sprintf("%s %d", theme.Glyph("credit", st.Settings.ASCIISafe), sim.RefuelCost(&st, g.content))
	repairCost := fmt.Sprintf("%s %d", theme.Glyph("credit", st.Settings.ASCIISafe), sim.RepairCost(&st, g.content))
	fuelPct := st.Fuel / sim.TankSize(&st, g.content) * 100
	fuelLabel := "FUEL " + theme.FuelBar(fuelPct, 20) + theme.FuelStyle(fuelPct).Render(fmt.Sprintf(" %.0f%%", fuelPct))
	hullLabel := "HULL " + theme.HullBar(float64(st.Hull), 20) + theme.HullStyle(float64(st.Hull)).Render(fmt.Sprintf(" %d%%", st.Hull))

	rightLines := []string{fuelLabel}
	refuelBtn := theme.Button("F", "REFUEL TO 100%", refuelCost, st.Credits > 0 && fuelPct < 100, theme.HueGreen)
	rightLines = append(rightLines, refuelBtn)
	g.hitPanelLine(1, rightX, refuelBtn, "svc:refuel", nil)

	rightLines = append(rightLines, hullLabel)
	repairBtn := theme.Button("H", "REPAIR — FULL", repairCost, st.Credits >= sim.RepairCost(&st, g.content) && st.Hull < 100, theme.HueGreen)
	rightLines = append(rightLines, repairBtn)
	g.hitPanelLine(3, rightX, repairBtn, "svc:repair", nil)

	if sim.InsuranceEligible(&st, g.content) {
		insBtn := theme.Amber.Render("[I] INSURANCE ADVANCE")
		rightLines = append(rightLines, insBtn)
		g.hitPanelLine(len(rightLines)-1, rightX, insBtn, "svc:insurance", nil)
	}
	shipyardBtn := theme.Button("U", "SHIPYARD", "", true, theme.HueViolet)
	rightLines = append(rightLines, shipyardBtn)
	g.hitPanelLine(len(rightLines)-1, rightX, shipyardBtn, "btn:shipyard", nil)

	logBtn := theme.Button("L", "SHIP'S LOG", "", true, theme.HueCyan)
	rightLines = append(rightLines, logBtn)
	g.hitPanelLine(len(rightLines)-1, rightX, logBtn, "btn:log", nil)

	rightLines = append(rightLines, "", theme.DimStyle.Render("Bigger rocks pay more but attract pirates."))

	if g.scr == scrShipyard {
		rightLines = append(rightLines, "", theme.Violet.Render("◇ SHIPYARD"))
		for t := 0; t < upgradeTrackCount; t++ {
			lvl := sim.UpgradeLevel(&st, sim.UpgradeTrack(t))
			filled := strings.Repeat(theme.Violet.Render("●"), lvl)
			empty := strings.Repeat(theme.DimStyle.Render("○"), g.content.Upgrades.MaxLevel-lvl)
			price := sim.UpgradePrice(g.content, sim.UpgradeTrack(t), lvl)
			priceStr := fmt.Sprintf("%d cr", price)
			if lvl >= g.content.Upgrades.MaxLevel {
				priceStr = "MAX"
			}
			sel := t == g.upgradeSel
			prefix := "  "
			if sel {
				prefix = "▸ "
			}
			trackPart := fmt.Sprintf("%s%s", prefix, sim.TrackName(sim.UpgradeTrack(t)))
			dotsPart := filled + empty
			pricePart := theme.Gold.Render(priceStr)
			line := theme.OptionHC(theme.HueViolet, sel, st.Settings.HighContrast).Render(trackPart) + " " + dotsPart + " " + pricePart
			rightLines = append(rightLines, line)
			g.hitPanelLine(len(rightLines)-1, rightX, line, fmt.Sprintf("upg:%d", t), t)
		}
	}
	rightBody := strings.Join(rightLines, "\n")

	body := lipgloss.JoinHorizontal(lipgloss.Top,
		theme.Panel("STAR CHART", leftW, g.height-5, leftBody, accent),
		theme.Panel("PORT SERVICES", rightW, g.height-5, rightBody, accent),
	)
	hint := "↑/↓ SELECT · ENTER DEPART · F REFUEL · H REPAIR · U SHIPYARD · L LOG · T TWEAKS · ? HELP · Q QUIT"
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
			return g.keyChart("h")
		case "svc:insurance":
			return g.keyChart("i")
		case "btn:shipyard":
			return g.keyChart("u")
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
			case strings.HasPrefix(b.ID, "upg:"):
				return g.keyChart("enter")
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
		case strings.HasPrefix(b.ID, "upg:"):
			if idx, ok := b.Data.(int); ok {
				g.upgradeSel = idx
			}
		}
	}
	return nil
}

func (g *Game) updateWheel(m tea.MouseWheelMsg) []tea.Cmd {
	if g.overlay != ovNone {
		return nil
	}
	if g.scr == scrChart || g.scr == scrShipyard {
		if m.Y < 0 {
			return g.keyChart("up")
		}
		return g.keyChart("down")
	}
	if g.scr == scrBelt {
		if m.Y < 0 {
			return g.keyBelt("up")
		}
		return g.keyBelt("down")
	}
	return nil
}
