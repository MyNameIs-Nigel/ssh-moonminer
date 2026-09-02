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
	tweakWrapLongText
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
		return "PIRATE THREAT ASSIST"
	case tweakHighContrast:
		return "HIGH CONTRAST"
	case tweakASCII:
		return "ASCII SAFE MODE"
	case tweakReducedMotion:
		return "REDUCED MOTION"
	case tweakWrapLongText:
		return "LONG TEXT"
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
	case tweakWrapLongText:
		if st.WrapLongText {
			return "wrap"
		}
		return "scroll"
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
	case tweakWrapLongText:
		if delta != 0 {
			s.WrapLongText = !s.WrapLongText
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
	panelW, panelH := g.layoutOverlaySize(tweaksPanelW, tweaksPanelH)
	panelBodyH := panelH - 2
	innerW := panelW - 4
	lines := []string{
		theme.LabelStyle(hc).Render("Arrow keys change values · T, Q, or Esc close"),
		theme.DimStyle.Render("Pirate speed only. Rewards unchanged."),
	}
	tweakLineIdx := 2
	for i := 0; i < tweakCount; i++ {
		marker := "  "
		sel := i == g.tweaksSel
		if sel {
			marker = "▸ "
		}
		label := g.tweakLabel(i)
		val := g.tweakValue(st.Settings, i)
		left := marker + label
		pad := innerW - lipgloss.Width(left) - lipgloss.Width(val)
		if pad < 1 {
			pad = 1
		}
		line := theme.OptionHC(theme.HueViolet, sel, hc).Render(left+strings.Repeat(" ", pad)) + theme.Bright.Render(val)
		line = clipFrameLine(line, innerW)
		lines = append(lines, line)
	}
	scroll := 0
	if len(lines) > panelBodyH {
		scroll = centeredScroll(g.tweaksSel+tweakLineIdx, len(lines), panelBodyH)
	}
	visible := scrollWindowLines(lines, scroll, panelBodyH)
	for i, line := range visible {
		src := scroll + i
		if src >= tweakLineIdx && src < tweakLineIdx+tweakCount {
			g.tweaksHitLine(i, line, src-tweakLineIdx, panelW, panelH)
		}
	}
	body := strings.Join(visible, "\n")
	return theme.Panel("TWEAKS", panelW, panelH, body, theme.Accent(theme.HueViolet))
}

func (g *Game) tweaksHitLine(lineIdx int, label string, tweakIdx, panelW, panelH int) {
	g.overlayHitLine(panelW, panelH, lineIdx, label, "tweak", tweakIdx)
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
	case "esc", "q", "?":
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
	case ovSignalLost:
		// Dismissed by any input, mouse included — the game is mouse-everywhere.
		return g.dismissSignalLost()
	case ovPermit:
		if b, ok := g.hitAt(m.X, m.Y); ok && b.ID == "permit:buy" {
			return g.confirmPermitPurchase()
		}
		if b, ok := g.hitAt(m.X, m.Y); ok && b.ID == "permit:cancel" {
			g.overlay = ovNone
		}
	case ovTweaks:
		if b, ok := g.hitAt(m.X, m.Y); ok && strings.HasPrefix(b.ID, "tweak:") {
			if idx, ok := b.Data.(int); ok {
				g.tweaksSel = idx
				return g.cycleTweak(1)
			}
		}
	case ovDev:
		if b, ok := g.hitAt(m.X, m.Y); ok && strings.HasPrefix(b.ID, "dev:") {
			if idx, ok := b.Data.(int); ok {
				g.devSel = idx
				return g.devDelta(1)
			}
		}
	case ovSlotPicker:
		if b, ok := g.hitAt(m.X, m.Y); ok && strings.HasPrefix(b.ID, "picker:") {
			if idx, ok := b.Data.(int); ok {
				model, row, entries, ctxOK := g.pickerContext()
				if !ctxOK {
					break
				}
				if idx == g.pickerSel {
					return g.pickerConfirm(model, row, entries)
				}
				g.pickerSel = idx
				g.pickerClampScroll(len(entries))
			}
		}
	case ovSlotRemove:
		if b, ok := g.hitAt(m.X, m.Y); ok && strings.HasPrefix(b.ID, "removeopt:") {
			if sel, ok := b.Data.(int); ok {
				model, row, ctxOK := g.removeContext()
				if !ctxOK {
					break
				}
				if sel == g.removeConfirmSel {
					return g.removeConfirm(model, row)
				}
				g.removeConfirmSel = sel
			}
		}
	}
	return nil
}

func (g *Game) updateOverlayWheel(m tea.MouseWheelMsg) []tea.Cmd {
	delta := wheelDelta(m)
	if delta == 0 {
		return nil
	}
	switch g.overlay {
	case ovHelp:
		if delta < 0 {
			g.helpScrollBy(-1)
		} else {
			g.helpScrollBy(1)
		}
	case ovTweaks:
		if delta < 0 {
			g.tweaksSel = (g.tweaksSel - 1 + tweakCount) % tweakCount
		} else {
			g.tweaksSel = (g.tweaksSel + 1) % tweakCount
		}
	case ovDev:
		if delta < 0 {
			g.devSel = (g.devSel - 1 + devRowCount) % devRowCount
		} else {
			g.devSel = (g.devSel + 1) % devRowCount
		}
	case ovSlotPicker:
		_, _, entries, ok := g.pickerContext()
		if !ok || len(entries) == 0 {
			break
		}
		if delta < 0 {
			g.pickerSel = (g.pickerSel - 1 + len(entries)) % len(entries)
		} else {
			g.pickerSel = (g.pickerSel + 1) % len(entries)
		}
		g.pickerClampScroll(len(entries))
	}
	return nil
}
