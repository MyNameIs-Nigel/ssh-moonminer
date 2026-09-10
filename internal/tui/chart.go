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
	// Belt-and-suspenders for a pilot who reaches the chart while still out at
	// a belt: every port service is IsDocked()-gated, so the chart would be a
	// dead end. Enter becomes RETURN TO BELT — deliberately a pure screen
	// change, never sim.Dock, because docking rearms jammer/EMP/missiles and
	// restores shields, and restoring someone's position must not hand them a
	// service they did not fly back to buy.
	// See docs/framework/05-reconnect-and-location-restore.md.
	if !st.IsDocked() {
		switch k {
		case "enter", " ":
			g.scr = scrBelt
			g.rockSel = 0
			return nil
		case "f", "r", "c", "i", "s", "j", "h":
			g.setFlash("STILL IN BELT — RETURN TO BELT, THEN [Q] DOCK")
			return nil
		}
	}
	switch k {
	case "up", "k":
		g.worldSel = (g.worldSel - 1 + n) % n
	case "down":
		g.worldSel = (g.worldSel + 1) % n
	case "j":
		g.overlay = ovCertify
	case "h":
		g.ferrySel = 0
		g.overlay = ovFerry
	case "enter", " ":
		if w := g.content.WorldByIndex(g.worldSel); w != nil && w.SystemID != st.SystemID {
			g.openJumpConfirm()
			return nil
		}
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
	leftW, rightW := chartPaneWidths(g.contentWidth())
	panelH := g.bodyHeight()
	panelBodyH := panelH - 2
	accent := theme.Accent(theme.HueCyan)
	var leftLines []string
	type chartWorldHit struct {
		row  int
		line string
		idx  int
	}
	var worldHits []chartWorldHit
	for _, system := range g.content.Systems {
		currentSystem := system.ID == st.SystemID
		systemGlyph := theme.Glyph("diamond", st.Settings.ASCIISafe)
		systemLabel := systemGlyph + " " + system.Name
		if currentSystem {
			systemGlyph = theme.Glyph("diamond_filled", st.Settings.ASCIISafe)
			systemLabel = systemGlyph + " " + system.Name
		} else if sim.JumpLockReason(&st, g.content, system.ID) != "" {
			systemLabel += " — " + theme.Red.Render("LOCKED")
		} else {
			systemLabel += " — " + theme.Red.Render("NOT IN SYSTEM")
		}
		systemHeader := theme.Violet.Render(systemLabel)
		if currentSystem {
			systemHeader = theme.Bright.Bold(true).Render(systemLabel)
		}
		leftLines = append(leftLines, systemHeader)
		for i, w := range g.content.Worlds {
			if w.SystemID != system.ID {
				continue
			}
			marker := "  "
			sel := i == g.worldSel
			if sel {
				marker = "▸ "
			}
			status := ""
			if currentSystem {
				lock := sim.RouteLockReason(&st, g.content, i)
				status = fmt.Sprintf("%s %d", theme.Glyph("fuel", st.Settings.ASCIISafe), w.TravelFuel)
				if sim.RemainingCargoCapacity(&st, g.content) <= 0 {
					status = theme.Red.Render("HOLD FULL — SELL CARGO")
				} else if lock != "" {
					status = theme.Red.Render(lock)
				} else if !sim.CanDepart(&st, g.content, i) {
					status = theme.Red.Render(status)
				} else {
					status = theme.Amber.Render(status)
				}
			}
			namePart := fmt.Sprintf("%s%s  %s", marker, w.Name, w.Sub)
			line := theme.OptionHC(theme.HueCyan, sel, st.Settings.HighContrast || currentSystem).Render(namePart)
			if status != "" {
				line += "  " + status
			}
			line = ansi.Truncate(line, leftW-2, "…")
			leftLines = append(leftLines, line)
			worldHits = append(worldHits, chartWorldHit{len(leftLines) - 1, line, i})
		}
	}
	leftFooter := []string{}
	if g.worldSel >= 0 && g.worldSel < len(g.content.Worlds) {
		leftFooter = g.chartFooterLines(g.content.Worlds[g.worldSel].Desc, leftW-2, panelBodyH-4)
	}
	selRow := -1
	for _, h := range worldHits {
		if h.idx == g.worldSel {
			selRow = h.row
			break
		}
	}
	leftBodyLines, leftScroll := panelViewportWithFooter(leftLines, leftFooter, panelBodyH, selRow)
	for _, h := range worldHits {
		visRow := h.row - leftScroll
		if visRow >= 0 && visRow < len(leftBodyLines)-len(leftFooter) {
			g.hitPanelLine(visRow, 1, h.line, fmt.Sprintf("world:%d", h.idx), h.idx)
		}
	}
	leftBody := strings.Join(leftBodyLines, "\n")

	docked := st.IsDocked()
	rightBody := g.renderChartServices(leftW+1, rightW-2, panelBodyH)

	chartTitle := "STAR CHART"
	if system := g.content.SystemByID(st.SystemID); system != nil {
		chartTitle += " — " + system.Name
	}
	servicesTitle := "PORT SERVICES"
	if !docked {
		servicesTitle = "PORT SERVICES — OFFLINE"
	}
	body := lipgloss.JoinHorizontal(lipgloss.Top,
		theme.Panel(chartTitle, leftW, panelH, leftBody, accent),
		theme.Panel(servicesTitle, rightW, panelH, rightBody, accent),
	)
	hint := "↑/↓ SELECT · ENTER FLY · C SELL · S YARD · L LOG · T TWEAKS · ? HELP · Q QUIT"
	if !docked {
		hint = "ENTER RETURN TO BELT · L LOG · T TWEAKS · ? HELP · Q QUIT"
	}
	return body + "\n" + g.renderKeybar(hint)
}

const doubleClickMS = 400

func (g *Game) updateClick(m tea.MouseClickMsg) []tea.Cmd {
	if b, ok := g.hitAt(m.X, m.Y); ok {
		// Buttons fire on single click.
		switch b.ID {
		case "svc:certify":
			return g.keyChart("j")
		case "svc:ferry":
			return g.keyChart("h")
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
		case "btn:acquire":
			return g.keyShipyard("enter")
		case "btn:shipyard:remove":
			return g.keyShipyard("x")
		case "btn:log":
			return g.keyChart("l")
		case "btn:return-belt":
			return g.keyChart("enter")
		case "btn:scan":
			return g.keyBelt("s")
		case "btn:lock":
			return g.keyBelt("enter")
		case "btn:bail":
			return g.keyMining(tea.KeyPressMsg{Code: 'b', Text: "b"})
		case "btn:skillcheck":
			return g.keyMining(tea.KeyPressMsg{Code: ' ', Text: " "})
		case "btn:tribute:accept":
			return g.keyMining(tea.KeyPressMsg{Code: 'd', Text: "d"})
		case "btn:tribute:refuse":
			return g.keyMining(tea.KeyPressMsg{Code: 'r', Text: "r"})
		case "btn:tribute:fight":
			return g.keyMining(tea.KeyPressMsg{Code: 'f', Text: "f"})
		case "btn:combat:fire":
			return g.keyMining(tea.KeyPressMsg{Code: 'f', Text: "f"})
		case "btn:combat:missile":
			return g.keyMining(tea.KeyPressMsg{Code: 'g', Text: "g"})
		case "btn:combat:escape":
			return g.keyMining(tea.KeyPressMsg{Code: 'b', Text: "b"})
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
	delta := wheelDelta(m)
	if delta == 0 {
		return nil
	}
	switch g.scr {
	case scrChart:
		if delta < 0 {
			return g.keyChart("up")
		}
		return g.keyChart("down")
	case scrShipyard:
		if delta < 0 {
			return g.keyShipyard("up")
		}
		return g.keyShipyard("down")
	case scrBelt:
		if delta < 0 {
			return g.keyBelt("up")
		}
		return g.keyBelt("down")
	case scrLog:
		if delta < 0 {
			return g.keyLog("up")
		}
		return g.keyLog("down")
	}
	return nil
}

// wheelDelta reads the button that the terminal reports for a wheel event.
// MouseWheelMsg.Y is the pointer's row on screen, not the wheel direction.
func wheelDelta(m tea.MouseWheelMsg) int {
	switch m.Button {
	case tea.MouseWheelUp:
		return -1
	case tea.MouseWheelDown:
		return 1
	default:
		return 0
	}
}
