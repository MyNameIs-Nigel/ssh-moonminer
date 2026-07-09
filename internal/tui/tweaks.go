package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/theme"
)

const (
	tweakBeltView = iota
	tweakPirateAgg
	tweakHighContrast
	tweakASCII
	tweakReducedMotion
	tweakCount
)

var pirateAggLevels = []float64{0.5, 1.0, 1.5, 2.0}

var beltViewLabels = []string{"tiles", "orescan", "radar"}

const tweaksPanelW = 52
const tweaksPanelH = 12

func (g *Game) tweakLabel(i int) string {
	switch i {
	case tweakBeltView:
		return "BELT VIEW"
	case tweakPirateAgg:
		return "PIRATE AGGRESSION"
	case tweakHighContrast:
		return "HIGH CONTRAST"
	case tweakASCII:
		return "ASCII SAFE MODE"
	case tweakReducedMotion:
		return "REDUCED MOTION"
	default:
		return ""
	}
}

func (g *Game) tweakValue(st sim.Settings, i int) string {
	switch i {
	case tweakBeltView:
		mode := int(st.BeltView)
		if mode < 0 || mode >= len(beltViewLabels) {
			mode = 0
		}
		return beltViewLabels[mode]
	case tweakPirateAgg:
		return fmt.Sprintf("%.1f", st.PirateAggression)
	case tweakHighContrast:
		return boolLabel(st.HighContrast)
	case tweakASCII:
		return boolLabel(st.ASCIISafe)
	case tweakReducedMotion:
		return boolLabel(st.ReducedMotion)
	default:
		return ""
	}
}

func boolLabel(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

func applyTweakDelta(s *sim.Settings, row, delta int) {
	switch row {
	case tweakBeltView:
		mode := (int(s.BeltView) + delta) % 3
		if mode < 0 {
			mode += 3
		}
		s.BeltView = sim.BeltViewMode(mode)
	case tweakPirateAgg:
		idx := pirateAggIndex(s.PirateAggression)
		idx = (idx + delta) % len(pirateAggLevels)
		if idx < 0 {
			idx += len(pirateAggLevels)
		}
		s.PirateAggression = pirateAggLevels[idx]
	case tweakHighContrast:
		if delta != 0 {
			s.HighContrast = !s.HighContrast
		}
	case tweakASCII:
		if delta != 0 {
			s.ASCIISafe = !s.ASCIISafe
		}
	case tweakReducedMotion:
		if delta != 0 {
			s.ReducedMotion = !s.ReducedMotion
		}
	}
}

func (g *Game) cycleTweak(delta int) []tea.Cmd {
	i := g.tweaksSel
	snap, err := g.sess.UpdateSettings(g.now, func(s *sim.Settings) {
		applyTweakDelta(s, i, delta)
	})
	return g.refreshSnap(snap, err)
}

func pirateAggIndex(v float64) int {
	for i, level := range pirateAggLevels {
		if level == v {
			return i
		}
	}
	return 1
}

func (g *Game) renderTweaksOverlay() string {
	st := g.snap.State
	hc := st.Settings.HighContrast
	lines := []string{theme.LabelStyle(hc).Render("Arrow keys change values · T, Q, or Esc close"), ""}
	for i := 0; i < tweakCount; i++ {
		marker := "  "
		sel := i == g.tweaksSel
		if sel {
			marker = "▸ "
		}
		label := g.tweakLabel(i)
		val := g.tweakValue(st.Settings, i)
		left := marker + label
		pad := tweaksPanelW - 4 - lipgloss.Width(left) - lipgloss.Width(val)
		if pad < 1 {
			pad = 1
		}
		line := left + strings.Repeat(" ", pad) + theme.Bright.Render(val)
		line = theme.OptionHC(theme.HueViolet, sel, hc).Render(left+strings.Repeat(" ", pad)) + theme.Bright.Render(val)
		lines = append(lines, line)
		// Hitboxes registered relative to overlay panel; map to terminal in overlay click handler.
		g.tweaksHitLine(len(lines) - 1, line, i)
	}
	body := strings.Join(lines, "\n")
	return theme.Panel("TWEAKS", tweaksPanelW, tweaksPanelH, body, theme.Accent(theme.HueViolet))
}

func (g *Game) tweaksHitLine(lineIdx int, label string, tweakIdx int) {
	y0 := (g.height - tweaksPanelH) / 2
	x0 := (g.width - tweaksPanelW) / 2
	if y0 < 0 {
		y0 = 0
	}
	if x0 < 0 {
		x0 = 0
	}
	// Panel body starts one line below top border.
	y := y0 + 1 + lineIdx
	x := x0 + 1
	w := lipgloss.Width(label)
	if w < 1 {
		w = 1
	}
	g.addHit(x, y, w, 1, fmt.Sprintf("tweak:%d", tweakIdx), tweakIdx)
}

func (g *Game) updateTweaksOverlay(k string) []tea.Cmd {
	switch k {
	case "esc", "t", "q", "?":
		g.overlay = ovNone
	case "up", "k":
		g.tweaksSel = (g.tweaksSel - 1 + tweakCount) % tweakCount
	case "down", "j":
		g.tweaksSel = (g.tweaksSel + 1) % tweakCount
	case "left", "h":
		return g.cycleTweak(-1)
	case "right", "l", "enter", " ":
		return g.cycleTweak(1)
	}
	return nil
}

func (g *Game) updateHelpOverlay(k string) []tea.Cmd {
	switch k {
	case "esc", "?":
		g.overlay = ovNone
	case "up", "k":
		g.helpScrollBy(-1)
	case "down", "j":
		g.helpScrollBy(1)
	}
	return nil
}

func (g *Game) updateOverlayClick(m tea.MouseClickMsg) []tea.Cmd {
	switch g.overlay {
	case ovTweaks:
		if b, ok := g.hits.At(m.X, m.Y); ok && strings.HasPrefix(b.ID, "tweak:") {
			if idx, ok := b.Data.(int); ok {
				g.tweaksSel = idx
				return g.cycleTweak(1)
			}
		}
	}
	return nil
}

func (g *Game) updateOverlayWheel(m tea.MouseWheelMsg) []tea.Cmd {
	switch g.overlay {
	case ovHelp:
		if m.Y < 0 {
			g.helpScrollBy(-1)
		} else if m.Y > 0 {
			g.helpScrollBy(1)
		}
	case ovTweaks:
		if m.Y < 0 {
			g.tweaksSel = (g.tweaksSel - 1 + tweakCount) % tweakCount
		} else if m.Y > 0 {
			g.tweaksSel = (g.tweaksSel + 1) % tweakCount
		}
	case ovDev:
		if m.Y < 0 {
			g.devSel = (g.devSel - 1 + devRowCount) % devRowCount
		} else if m.Y > 0 {
			g.devSel = (g.devSel + 1) % devRowCount
		}
	}
	return nil
}
