package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/theme"
)

func (g *Game) keyBelt(k string) []tea.Cmd {
	st := g.snap.State
	n := len(st.Belt)
	switch k {
	case "up", "k", "left":
		if n > 0 {
			g.rockSel = (g.rockSel - 1 + n) % n
		}
	case "down", "j", "right":
		if n > 0 {
			g.rockSel = (g.rockSel + 1) % n
		}
	case "enter", " ":
		if n == 0 {
			snap, _ := g.sess.Rescan(g.now)
			return g.refreshSnap(snap, nil)
		}
		rock := st.Belt[g.rockSel]
		snap, err := g.sess.Lock(g.now, rock.ID)
		if err == nil {
			g.scr = scrMining
		}
		return g.refreshSnap(snap, err)
	case "v":
		mode := int(st.Settings.BeltView)
		mode = (mode + 1) % 3
		st.Settings.BeltView = sim.BeltViewMode(mode)
		snap, _ := g.sess.UpdateSettings(g.now, func(s *sim.Settings) { s.BeltView = sim.BeltViewMode(mode) })
		return g.refreshSnap(snap, nil)
	case "r":
		snap, _ := g.sess.Rescan(g.now)
		g.rockSel = 0
		return g.refreshSnap(snap, nil)
	case "q":
		snap, _ := g.sess.Dock(g.now)
		g.scr = scrChart
		return g.refreshSnap(snap, nil)
	}
	return nil
}

func (g *Game) renderBelt() string {
	st := g.snap.State
	w := g.content.WorldByIndex(st.WorldIdx)
	worldName := "UNKNOWN"
	if w != nil {
		worldName = w.Name
	}
	lines := []string{theme.Gold.Render("◇ ASTEROID BELT — " + worldName)}
	if len(st.Belt) == 0 {
		lines = append(lines, theme.Amber.Render("Belt depleted — press R to rescan"))
	} else {
		for i, rock := range st.Belt {
			marker := "  "
			sel := i == g.rockSel
			if sel {
				marker = "▸ "
			}
			tierGlyph := g.content.Tiers.Glyphs[rock.Tier]
			line := fmt.Sprintf("%s%-8s %s %-8s VOL %4d  ⏱ %.1fs  %s %5d  -%2d%%  %s",
				marker, rock.Name, tierGlyph, g.content.Tiers.Labels[rock.Tier],
				rock.Volume, rock.DrillSec, theme.Glyph("credit", st.Settings.ASCIISafe),
				rock.Value, rock.FuelCost, theme.Dots(rock.Dots, 5))
			style := theme.TierStyleDim(rock.Tier)
			if sel {
				style = theme.TierStyle(rock.Tier, st.Settings.ASCIISafe).Bold(true).Background(lipgloss.Color(lipglossColorForTier(rock.Tier)))
			}
			rendered := style.Render(line)
			lines = append(lines, rendered)
			g.hitBodyLine(1+i, 1, rendered, fmt.Sprintf("belt:%d", i), i)
		}
	}
	if g.rockSel < len(st.Belt) {
		rock := st.Belt[g.rockSel]
		fuelStr := fmt.Sprintf("%d fuel", rock.FuelCost)
		if float64(rock.FuelCost) > st.Fuel {
			fuelStr = theme.Red.Render(fuelStr)
		} else {
			fuelStr = theme.White.Render(fuelStr)
		}
		lines = append(lines, "",
			fmt.Sprintf("TARGET LOCK: %s  FLIGHT %s  VALUE %s",
				theme.Gold.Render(rock.Name), fuelStr, theme.Gold.Render(fmt.Sprintf("%d", rock.Value))),
			theme.Button("ENTER", "LOCK & FLY", "", true, theme.HueGold),
			theme.DimStyle.Render("[V] VIEW  [R] RESCAN  [Q] DOCK"))
	}
	body := strings.Join(lines, "\n")
	hint := "↑/↓ SELECT · ENTER LOCK · V VIEW · R RESCAN · Q DOCK"
	return body + "\n" + g.renderKeybar(hint)
}

func lipglossColorForTier(tier int) string {
	switch tier {
	case 0:
		return "#1a2a3a"
	case 1:
		return "#0a2a3a"
	case 2:
		return "#2a1a4a"
	default:
		return "#3a2a10"
	}
}
