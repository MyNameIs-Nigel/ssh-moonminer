package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/hitbox"
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
		g.overlay = ovTweaks
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

func (g *Game) renderChart() string {
	st := g.snap.State
	leftW := g.width/2 - 1
	rightW := g.width - leftW - 1
	leftLines := []string{}
	for i, w := range g.content.Worlds {
		marker := "  "
		style := theme.TxtStyle
		if i == g.worldSel {
			marker = "▸ "
			style = theme.Selected
		}
		fuel := fmt.Sprintf("%s %d", theme.Glyph("fuel", st.Settings.ASCIISafe), w.TravelFuel)
		if !sim.CanDepart(&st, g.content, i) {
			fuel = theme.Red.Render(fuel)
		}
		line := style.Render(fmt.Sprintf("%s%s  %s  %s", marker, w.Name, w.Sub, fuel))
		leftLines = append(leftLines, line)
		g.hits.Add(hitbox.Box{Y: 3 + i, X: 1, W: leftW - 2, H: 1, ID: fmt.Sprintf("world:%d", i), Data: i})
	}
	if g.worldSel < len(g.content.Worlds) {
		leftLines = append(leftLines, "", theme.DimStyle.Render(g.content.Worlds[g.worldSel].Desc))
	}
	leftBody := strings.Join(leftLines, "\n")

	refuelCost := fmt.Sprintf("%s %d", theme.Glyph("credit", st.Settings.ASCIISafe), sim.RefuelCost(&st, g.content))
	repairCost := fmt.Sprintf("%s %d", theme.Glyph("credit", st.Settings.ASCIISafe), sim.RepairCost(&st, g.content))
	fuelPct := st.Fuel / sim.TankSize(&st, g.content) * 100
	rightLines := []string{
		"FUEL " + theme.RampBar(fuelPct, 20, false) + fmt.Sprintf(" %.0f%%", fuelPct),
		theme.Button("F", "REFUEL TO 100%", refuelCost, st.Credits > 0 && fuelPct < 100),
		"HULL " + theme.RampBar(float64(st.Hull), 20, false) + fmt.Sprintf(" %d%%", st.Hull),
		theme.Button("H", "REPAIR — FULL", repairCost, st.Credits >= sim.RepairCost(&st, g.content) && st.Hull < 100),
	}
	g.hits.Add(hitbox.Box{Y: 4, X: leftW + 2, W: 20, H: 1, ID: "svc:refuel"})
	g.hits.Add(hitbox.Box{Y: 6, X: leftW + 2, W: 20, H: 1, ID: "svc:repair"})
	if sim.InsuranceEligible(&st, g.content) {
		rightLines = append(rightLines, theme.Amber.Render("[I] INSURANCE ADVANCE"))
		g.hits.Add(hitbox.Box{Y: 7, X: leftW + 2, W: 22, H: 1, ID: "svc:insurance"})
	}
	rightLines = append(rightLines,
		theme.Button("U", "SHIPYARD", "", true),
		theme.Button("L", "SHIP'S LOG", "", true),
		"", theme.DimStyle.Render("Bigger rocks pay more but attract pirates."),
	)
	if g.scr == scrShipyard {
		rightLines = append(rightLines, "", theme.Bright.Render("◇ SHIPYARD"))
		for t := 0; t < 5; t++ {
			lvl := sim.UpgradeLevel(&st, sim.UpgradeTrack(t))
			pips := strings.Repeat("●", lvl) + strings.Repeat("○", g.content.Upgrades.MaxLevel-lvl)
			price := sim.UpgradePrice(g.content, sim.UpgradeTrack(t), lvl)
			line := fmt.Sprintf("%s %s %s %d cr", sim.TrackName(sim.UpgradeTrack(t)), pips,
				map[int]string{0: "", g.content.Upgrades.MaxLevel: "MAX"}[lvl], price)
			if t == g.upgradeSel {
				line = "▸ " + line
			}
			rightLines = append(rightLines, line)
			g.hits.Add(hitbox.Box{Y: 10 + t, X: leftW + 2, W: 30, H: 1, ID: fmt.Sprintf("upg:%d", t), Data: t})
		}
	}
	rightBody := strings.Join(rightLines, "\n")

	body := lipgloss.JoinHorizontal(lipgloss.Top,
		theme.Panel("STAR CHART", leftW, g.height-5, leftBody),
		theme.Panel("PORT SERVICES", rightW, g.height-5, rightBody),
	)
	hint := "↑↓ SELECT · ENTER DEPART · F REFUEL · H REPAIR · U SHIPYARD · L LOG · T TWEAKS · ? HELP · Q QUIT"
	return body + "\n" + g.renderKeybar(hint)
}

func (g *Game) updateClick(m tea.MouseClickMsg) []tea.Cmd {
	if b, ok := g.hits.At(m.X, m.Y); ok {
		now := g.now * 1000
		if b.ID == g.lastClickID && now-g.lastClickAt < 400 {
			switch {
			case strings.HasPrefix(b.ID, "world:"):
				return g.keyChart("enter")
			case strings.HasPrefix(b.ID, "belt:"):
				return g.keyBelt("enter")
			case b.ID == "svc:refuel":
				return g.keyChart("f")
			case b.ID == "svc:repair":
				return g.keyChart("h")
			case b.ID == "svc:insurance":
				return g.keyChart("i")
			case b.ID == "btn:bail":
				return g.keyMining(tea.KeyPressMsg{})
			case b.ID == "btn:overdrive":
				g.overdriveOn = true
				_, _ = g.sess.SetOverdrive(true)
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
