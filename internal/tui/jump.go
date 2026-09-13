package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/theme"
)

type jumpTickMsg struct{ generation uint64 }
type jumpFlashDoneMsg struct{ generation uint64 }

func jumpFlashCmd(generation uint64) tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg { return jumpFlashDoneMsg{generation} })
}

func jumpTickCmd(generation uint64) tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return jumpTickMsg{generation} })
}
func (g *Game) openJumpConfirm() {
	w := g.content.WorldByIndex(g.worldSel)
	if w == nil {
		return
	}
	g.jumpTarget = w.SystemID
	if _, err := sim.PreviewJump(&g.snap.State, g.content, g.jumpTarget); err != nil {
		g.setFlash(err.Error())
		return
	}
	g.overlay = ovJump
}
func (g *Game) updateJumpConfirm(k string) []tea.Cmd {
	if k == "esc" || k == "n" || k == "q" {
		g.overlay = ovNone
		return nil
	}
	if k != "enter" && k != "y" {
		return nil
	}
	snap, err := g.sess.Jump(g.now, g.jumpTarget)
	cmds := g.refreshSnap(snap, err)
	if err != nil {
		return cmds
	}
	return append(cmds, g.beginJump(snap.State.LastJump)...)
}
func (g *Game) beginJump(result *sim.JumpResult) []tea.Cmd {
	g.overlay = ovNone
	g.scr = scrJump
	g.jumpResult = result
	g.jumpBeat = g.content.Jump.CountdownSeconds
	g.jumpGeneration++
	g.jumpFlash = false
	if g.snap.State.Settings.ReducedMotion {
		g.jumpBeat = 0
	}
	if g.jumpBeat > 0 {
		return []tea.Cmd{jumpTickCmd(g.jumpGeneration)}
	}
	return nil
}
func (g *Game) keyJump(k string) []tea.Cmd {
	if k == "" {
		return nil
	}
	if g.jumpBeat > 0 || g.jumpFlash {
		g.jumpFlash = false
		g.jumpBeat = 0
		g.jumpGeneration++
	} else {
		g.scr = scrChart
	}
	return nil
}
func (g *Game) renderJumpConfirm() string {
	st := &g.snap.State
	p, err := sim.PreviewJump(st, g.content, g.jumpTarget)
	if err != nil {
		return err.Error()
	}
	target := g.content.SystemByID(g.jumpTarget)
	w := g.content.WorldByIndex(g.worldSel)
	travel := 0
	if w != nil && w.SystemID == target.ID {
		travel = w.TravelFuel
	}
	lines := []string{"JUMP — " + g.content.SystemByID(st.SystemID).Name + " → " + target.Name,
		fmt.Sprintf("RATING CLASS %s — CLEARED", sim.GradeLetter(st.JumpClass)), fmt.Sprintf("JUMP FUEL %.1f · LOCAL DEPARTURE %d (separate)", p.FuelCost, travel),
		fmt.Sprintf("TRANSIT STRESS %d HULL · DRIFT %.1f%%", p.Stress, p.DriftChance*100), fmt.Sprintf("PROJECTED FUEL %.1f · HULL %d (before drift)", p.FuelAfter, p.HullAfter),
		fmt.Sprintf("Drift: hull -%.0f–%.0f%%, remaining fuel -%.0f–%.0f%%, or hot arrival.", g.content.Jump.HardTranslationMin*100, g.content.Jump.HardTranslationMax*100, g.content.Jump.FuelBloomMin*100, g.content.Jump.FuelBloomMax*100)}
	gate := g.content.GateBetween(st.SystemID, target.ID)
	if gate.MisalignmentWeight > 0 {
		lines = append(lines, "Repeat crossings may misalign back to the origin.")
	}
	if p.LowHull {
		lines = append(lines, fmt.Sprintf("WARNING — BELOW %.0f%% HULL: RANDOM EVENT ZONE", g.content.Fleet.EventHullGatePct*100))
	}
	lines = append(lines, "[ENTER] JUMP", "[ESC] CANCEL")
	return g.progressionPanel("JUMP CONFIRM", lines, "jump:confirm")
}
func (g *Game) progressionPanel(title string, lines []string, id string) string {
	w, h := g.layoutOverlaySize(72, 17)
	body := reflowOverlayLines(lines, w-4)
	for i, line := range body {
		if strings.Contains(line, "[ESC]") {
			parts := strings.SplitN(id, ":", 2)
			g.overlayHitLine(w, h, i, line, parts[0], "cancel")
		}
		if strings.Contains(line, "[ENTER]") {
			parts := strings.SplitN(id, ":", 2)
			g.overlayHitLine(w, h, i, line, parts[0], parts[1])
		}
	}
	return theme.Panel(title, w, h, strings.Join(body, "\n"), theme.Accent(theme.HueCyan))
}
func (g *Game) renderJump() string {
	l := g.frameLayout()
	style := theme.Cyan
	if g.jumpFlash {
		color := "#176cb0"
		if g.jumpResult != nil && g.jumpResult.Drift != "" {
			color = "#9a1931"
		}
		style = lipgloss.NewStyle().Background(lipgloss.Color(color))
		return style.Width(l.Width).Height(l.Height).Render("")
	}
	lines := []string{}
	if g.jumpBeat > 0 {
		line := "1   THRESHOLD"
		if g.jumpBeat >= 3 {
			line = "3   ALIGNMENT LOCKED — " + g.content.SystemByID(g.jumpTarget).Name
		} else if g.jumpBeat == 2 {
			line = "2   CHARGING ——————————— 68%"
		}
		lines = append(lines, line, "", "ANY KEY — SKIP TO ARRIVAL")
	} else if r := g.jumpResult; r != nil {
		sys := g.content.SystemByID(r.SystemID)
		if sys == nil {
			return "INVALID ARRIVAL"
		}
		if sys.Hostile || r.Drift != "" {
			style = theme.Red
		}
		lines = append(lines, "ARRIVAL — "+sys.Name, sys.Signature)
		if r.FirstArrival {
			lines = append(lines, "FIRST ARRIVAL", sys.Pressure)
		}
		if r.Drift != "" {
			lines = append(lines, r.Drift)
		}
		lines = append(lines, fmt.Sprintf("FUEL %.1f · HULL %d", r.FuelAfter, r.HullAfter), "", "PRESS ANY KEY — DOCK")
	}
	body := strings.Join(reflowOverlayLines(lines, l.Width-4), "\n")
	return lipgloss.Place(l.Width, l.Height, lipgloss.Center, lipgloss.Center, style.Bold(true).Render(body))
}
