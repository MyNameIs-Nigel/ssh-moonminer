package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/theme"
)

const chartServiceButtonW = 20
const chartServiceButtonH = 3

// chartServiceButton is local to the chart: other screens retain their compact
// controls. The returned rows and hit rectangle share these exact dimensions.
func chartServiceButton(key, label string, enabled, ascii bool, hue theme.Hue) []string {
	top, side, bottom := "┌"+strings.Repeat("─", chartServiceButtonW-2)+"┐", "│", "└"+strings.Repeat("─", chartServiceButtonW-2)+"┘"
	if ascii {
		top = "+" + strings.Repeat("-", chartServiceButtonW-2) + "+"
		side = "|"
		bottom = top
	}
	text := "[" + key + "] " + label
	padding := chartServiceButtonW - 2 - lipgloss.Width(text)
	middle := side + strings.Repeat(" ", padding/2) + text + strings.Repeat(" ", padding-padding/2) + side
	style := theme.OptionHC(hue, false, true)
	if !enabled {
		style = theme.DimStyle
	}
	return []string{style.Render(top), style.Render(middle), style.Render(bottom)}
}

// renderChartServices reserves the bottom two rows for progression. The fixed
// 15-row service stack fits even the minimum 17-row panel interior, so actions
// never disappear through the list viewport's selection-driven scrolling.
func (g *Game) renderChartServices(x, width, height int) string {
	st := &g.snap.State
	docked, ascii := st.IsDocked(), st.Settings.ASCIISafe
	detailW := width - chartServiceButtonW - 1
	buttonX := detailW + 1
	details := make([]string, 15)
	credit := theme.Glyph("credit", ascii)
	fuel := sim.FuelAmount(st, g.content)
	tank := sim.TankSize(st, g.content)
	fuelPct := fuel / tank * 100
	hullPct := sim.HullPct(st, g.content)
	refuelCost, repairCost := sim.RefuelCost(st, g.content), sim.RepairCost(st, g.content)

	// Reserve the widest label in terminal cells so both gauges use the
	// smaller available span, regardless of the fuel glyph's display width.
	fuelLabel := theme.Glyph("fuel", ascii)
	barStart := max(lipgloss.Width(fuelLabel), lipgloss.Width("HULL")) + 1
	barWidth := detailW - barStart
	gauge := func(row int, label, bar, points string, cost int) {
		details[row] = label + strings.Repeat(" ", barStart-lipgloss.Width(label)) + bar
		points = strings.Repeat(" ", barStart) + points
		price := theme.Gold.Render(fmt.Sprintf("%s %d", credit, cost))
		if lipgloss.Width(points)+1+lipgloss.Width(price) <= detailW {
			details[row+1] = chartServiceValueRow(points, price, detailW)
		} else {
			details[row+1] = points
			details[row+2] = chartServiceValueRow("", price, detailW)
		}
	}
	gauge(0, fuelLabel, theme.FuelBar(fuelPct, barWidth),
		theme.FuelStyle(fuelPct).Render(fmt.Sprintf("%.0f/%.0f", fuel, tank)), refuelCost)
	gauge(3, "HULL", theme.HullBar(hullPct, barWidth),
		theme.HullStyle(hullPct).Render(fmt.Sprintf("%d/%d", sim.ActiveShip(st).Hull, sim.MaxHull(st, g.content))), repairCost)
	details[6] = fmt.Sprintf("CARGO %.0f/%.0f", st.CargoUnits, sim.CargoCapacityUnits(st, g.content))
	details[7] = chartServiceValueRow("SALE", theme.Gold.Render(fmt.Sprintf("%s %d", credit, st.CargoValue)), detailW)
	if st.BountyVouchers > 0 {
		details[8] = chartServiceValueRow("BOUNTY", theme.Amber.Render(fmt.Sprintf("%s %d", credit, st.BountyVouchers)), detailW)
	}
	if sim.InsuranceEligible(st, g.content) {
		details[10] = theme.Amber.Render("[I] INSURANCE")
		details[11] = theme.Amber.Render("ADVANCE")
		g.addHit(x, panelBodyY(10), detailW, 2, "svc:insurance", nil)
	}
	if !docked {
		details[9] = theme.Red.Render("IN BELT")
		details[10] = theme.DimStyle.Render("DOCK FOR SERVICES")
		details[12] = theme.Bright.Render("[ENTER]")
		details[13] = theme.Bright.Render("RETURN TO BELT")
		g.addHit(x, panelBodyY(12), detailW, 2, "btn:return-belt", nil)
	}
	services := []struct {
		key, label, id string
		enabled        bool
		hue            theme.Hue
	}{
		{"F", "REFUEL", "svc:refuel", docked && refuelCost > 0 && st.Credits >= g.content.Port.RefuelPerPoint, theme.HueGreen},
		{"R", "REPAIR", "svc:repair", docked && st.Credits >= repairCost && hullPct < 100, theme.HueGreen},
		{"C", "SELL CARGO", "svc:sell", docked && st.CargoValue+st.BountyVouchers > 0, theme.HueGold},
		{"S", "SHIPYARD", "btn:shipyard", docked, theme.HueViolet},
		{"L", "SHIP'S LOG", "btn:log", true, theme.HueCyan},
	}
	lines := make([]string, height)
	for i, svc := range services {
		row := i * chartServiceButtonH
		for j, buttonLine := range chartServiceButton(svc.key, svc.label, svc.enabled, ascii, svc.hue) {
			detail := ansi.Truncate(details[row+j], detailW, "…")
			lines[row+j] = detail + strings.Repeat(" ", detailW-lipgloss.Width(detail)+1) + buttonLine
		}
		g.addHit(x+buttonX, panelBodyY(row), chartServiceButtonW, chartServiceButtonH, svc.id, nil)
	}
	rating := "UNCERTIFIED"
	if st.JumpClass >= 0 {
		rating = "CLASS " + sim.GradeLetter(st.JumpClass)
	}
	prefix := "JUMP RATING: " + rating + " "
	certify := "[J] CERTIFY"
	linkStyle := theme.OptionHC(theme.HueViolet, false, true)
	if !docked {
		linkStyle = theme.DimStyle
	}
	footer := prefix + linkStyle.Render(certify)
	offset := width - lipgloss.Width(footer)
	lines[height-2] = strings.Repeat(" ", offset) + footer
	g.addHit(x+offset+lipgloss.Width(prefix), panelBodyY(height-2), lipgloss.Width(certify), 1, "svc:certify", nil)
	ferry := "[H] HAULER FERRY"
	separator := " · "
	if ascii {
		separator = " | "
	}
	footer = linkStyle.Render(ferry) + theme.DimStyle.Render(separator+"ENTER REMOTE: JUMP")
	offset = width - lipgloss.Width(footer)
	lines[height-1] = strings.Repeat(" ", offset) + footer
	g.addHit(x+offset, panelBodyY(height-1), lipgloss.Width(ferry), 1, "svc:ferry", nil)
	return strings.Join(lines, "\n")
}

// chartServiceValueRow keeps the amount flush with the gauge's right edge.
func chartServiceValueRow(label, amount string, width int) string {
	return label + strings.Repeat(" ", max(0, width-lipgloss.Width(label)-lipgloss.Width(amount))) + amount
}
