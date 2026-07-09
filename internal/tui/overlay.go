package tui

import (
	"strings"

	"charm.land/lipgloss/v2"

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
	case ovKicked:
		return theme.Panel("NOTICE", 50, 5, theme.Red.Render(g.kickReason)+"\n"+theme.DimStyle.Render("Press any key."), theme.Accent(theme.HueRed))
	case ovOnboard:
		pg := g.onboardPg
		if pg >= len(onboardPages) {
			pg = len(onboardPages) - 1
		}
		hint := "Press any key for more."
		if pg == len(onboardPages)-1 {
			hint = "Press any key to start."
		}
		return theme.Panel("ONBOARDING", 70, 6, onboardPages[pg]+"\n\n"+theme.DimStyle.Render(hint), theme.Accent(theme.HueGold))
	default:
		return ""
	}
}

func (g *Game) compositeView(base string) string {
	if g.overlay == ovNone {
		return base
	}
	ov := g.renderOverlay()
	if ov == "" {
		return base
	}
	faint := lipgloss.NewStyle().Faint(true)
	baseLines := strings.Split(base, "\n")
	for len(baseLines) < g.height {
		baseLines = append(baseLines, "")
	}
	for i := range baseLines {
		if i >= g.height {
			break
		}
		line := baseLines[i]
		if lipgloss.Width(line) < g.width {
			line += strings.Repeat(" ", g.width-lipgloss.Width(line))
		}
		baseLines[i] = faint.Render(line)
	}
	ovLines := strings.Split(ov, "\n")
	ovW, ovH := overlayBounds(ovLines)
	y0 := (g.height - ovH) / 2
	x0 := (g.width - ovW) / 2
	if y0 < 0 {
		y0 = 0
	}
	if x0 < 0 {
		x0 = 0
	}
	for i, ovLine := range ovLines {
		row := y0 + i
		if row < 0 || row >= len(baseLines) || row >= g.height {
			continue
		}
		baseLines[row] = overlayLine(baseLines[row], x0, ovLine, g.width)
	}
	if len(baseLines) > g.height {
		baseLines = baseLines[:g.height]
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

func overlayLine(base string, x int, insert string, termW int) string {
	if x < 0 {
		x = 0
	}
	pad := strings.Repeat(" ", x)
	// Base is faint-rendered; rebuild row as pad + overlay + trailing faint spaces.
	trail := termW - x - lipgloss.Width(insert)
	if trail < 0 {
		trail = 0
	}
	return pad + insert + strings.Repeat(" ", trail)
}
