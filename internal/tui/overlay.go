package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/theme"
)

func (g *Game) renderOverlay() string {
	switch g.overlay {
	case ovHelp:
		return g.renderHelpOverlay()
	case ovTweaks:
		return g.renderTweaksOverlay()
	case ovDev:
		return g.renderDevOverlay()
	case ovSlotPicker:
		return g.renderSlotPickerOverlay()
	case ovSlotRemove:
		return g.renderSlotRemoveOverlay()
	case ovPermit:
		return g.renderPermitOverlay()
	case ovKicked:
		panelW, panelH := g.layoutOverlaySize(50, 5)
		body := strings.Join(reflowOverlayLines([]string{
			theme.Red.Render(g.kickReason),
			theme.DimStyle.Render("Press any key."),
		}, panelW-4), "\n")
		return theme.Panel("NOTICE", panelW, panelH, body, theme.Accent(theme.HueRed))
	case ovSignalLost:
		return g.renderSignalLostOverlay()
	case ovOnboard:
		pg := g.onboardPg
		if pg >= len(onboardPages) {
			pg = len(onboardPages) - 1
		}
		panelW, panelH := g.layoutOverlaySize(70, 6)
		hint := "Press any key for more."
		if pg == len(onboardPages)-1 {
			hint = "Press any key to start."
		}
		body := strings.Join(reflowOverlayLines([]string{
			onboardPages[pg],
			"",
			theme.DimStyle.Render(hint),
		}, panelW-4), "\n")
		return theme.Panel("ONBOARDING", panelW, panelH, body, theme.Accent(theme.HueGold))
	default:
		return ""
	}
}

// renderSignalLostOverlay recaps the run the disconnect autopilot closed while
// the pilot was away. It is rebuilt from the persisted RunRecord rather than a
// RunOutcome, because the Session that produced the outcome died with the
// connection (docs/framework/05-reconnect-and-location-restore.md).
func (g *Game) renderSignalLostOverlay() string {
	rec := g.signalLost
	if rec == nil {
		return ""
	}
	st := g.snap.State
	credit := theme.Glyph("credit", st.Settings.ASCIISafe)
	kind := sim.OutcomeKind(rec.Outcome)
	lines := []string{
		theme.Red.Render("SIGNAL LOST — AUTOPILOT LOGGED"),
		theme.DimStyle.Render("The rig ran the escape without you."),
		"",
		theme.OutcomeStyle(rec.Outcome).Render(sim.OutcomeLabel(kind)),
		theme.Gold.Render("ASTEROID: ") + theme.TxtStyle.Render(rec.Asteroid),
	}
	if rec.CargoValueRecovered > 0 {
		lines = append(lines, theme.Green.Render(fmt.Sprintf("CARGO SEALED: +%s %d", credit, rec.CargoValueRecovered)))
	}
	if rec.CargoValueLost > 0 {
		lines = append(lines, theme.Red.Render(fmt.Sprintf("CARGO LOST: %s %d", credit, rec.CargoValueLost)))
	}
	if rec.CargoValueJettisoned > 0 {
		lines = append(lines, theme.Amber.Render(fmt.Sprintf("CARGO JETTISONED: %s %d", credit, rec.CargoValueJettisoned)))
	}
	lines = append(lines,
		theme.Amber.Render(fmt.Sprintf("HULL DELTA: %+d   FUEL DELTA: %+.0f", rec.HullDelta, rec.FuelDelta)),
		"",
		theme.DimStyle.Render("Press any key."))
	panelW, panelH := g.layoutOverlaySize(58, len(lines)+2)
	innerW := panelW - 4
	body := strings.Join(reflowOverlayLines(lines, innerW), "\n")
	return theme.Panel("RECONNECTED", panelW, panelH, body, theme.OutcomeAccent(rec.Outcome))
}

func (g *Game) compositeView(base string) string {
	if g.overlay == ovNone {
		return base
	}
	ov := g.renderOverlay()
	if ov == "" {
		return base
	}
	layout := g.frameLayout()
	faint := lipgloss.NewStyle().Faint(true)
	baseLines := normalizeFrameLines(strings.Split(base, "\n"), layout.Width, layout.Height)
	for i := range baseLines {
		line := baseLines[i]
		if lipgloss.Width(line) < layout.Width {
			line += strings.Repeat(" ", layout.Width-lipgloss.Width(line))
		}
		baseLines[i] = faint.Render(line)
	}
	ovLines := strings.Split(ov, "\n")
	ovW, ovH := overlayBounds(ovLines)
	y0 := chromeH + (layout.BodyH-ovH)/2
	x0 := (layout.Width - ovW) / 2
	if y0 < 0 {
		y0 = 0
	}
	if x0 < 0 {
		x0 = 0
	}
	for i, ovLine := range ovLines {
		row := y0 + i
		if row < 0 || row >= len(baseLines) {
			continue
		}
		baseLines[row] = overlayLine(baseLines[row], x0, ovLine, layout.Width)
	}
	return strings.Join(baseLines, "\n")
}

func overlayBounds(lines []string) (w, h int) {
	h = len(lines)
	for _, l := range lines {
		if lw := lipgloss.Width(l); lw > w {
			w = lw
		}
	}
	return w, h
}

func overlayLine(base string, x int, insert string, frameW int) string {
	if x < 0 {
		x = 0
	}
	insertW := lipgloss.Width(insert)
	right := x + insertW
	if right > frameW {
		right = frameW
	}
	// Keep the faint-rendered base content on either side of the overlay
	// instead of blanking it out.
	left := ansi.Cut(base, 0, x)
	if lipgloss.Width(left) < x {
		left += strings.Repeat(" ", x-lipgloss.Width(left))
	}
	trailStr := ansi.Cut(base, right, frameW)
	trailW := lipgloss.Width(trailStr)
	if want := frameW - right; trailW < want {
		trailStr += strings.Repeat(" ", want-trailW)
	}
	return left + insert + trailStr
}
