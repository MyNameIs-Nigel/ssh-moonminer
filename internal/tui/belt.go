package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/theme"
)

const radarHeight = 6

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
			return nil
		}
		rock := st.Belt[g.rockSel]
		snap, err := g.sess.Lock(g.now, rock.ID)
		if err == nil {
			g.scr = scrMining
		}
		return g.refreshSnap(snap, err)
	case "s":
		if n == 0 {
			return nil
		}
		rock := st.Belt[g.rockSel]
		snap, err := g.sess.Scan(g.now, rock.ID)
		return g.refreshSnap(snap, err)
	case "v":
		mode := int(st.Settings.BeltView)
		mode = (mode + 1) % 3
		st.Settings.BeltView = sim.BeltViewMode(mode)
		snap, _ := g.sess.UpdateSettings(g.now, func(s *sim.Settings) { s.BeltView = sim.BeltViewMode(mode) })
		return g.refreshSnap(snap, nil)
	case "q":
		snap, _ := g.sess.Dock(g.now)
		g.scr = scrChart
		return g.refreshSnap(snap, nil)
	}
	return nil
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// renderRadar draws a scatter of belt contacts as a small HUD above the
// asteroid list. Contacts are hollow until selected; the selected contact
// fills in and shows its name — in its true tier color once scanned, or a
// neutral "target lock" color if its details are still unknown.
func (g *Game) renderRadar(st *sim.State) string {
	w := g.width
	if w < 24 {
		w = 24
	}
	type mark struct {
		col   int
		text  string
		style lipgloss.Style
	}
	rows := make([][]mark, radarHeight)
	for i, rock := range st.Belt {
		col := clampInt(rock.X*(w-1)/100, 0, w-1)
		row := clampInt(rock.Y*(radarHeight-1)/100, 0, radarHeight-1)
		sel := i == g.rockSel
		glyph := "○"
		style := theme.DimStyle
		text := glyph
		if sel {
			glyph = "●"
			style = theme.White
			if rock.Scanned {
				style = theme.TierStyle(rock.Tier, st.Settings.ASCIISafe)
			}
			text = glyph + " " + rock.Name
		}
		if col+lipgloss.Width(text) > w {
			col = clampInt(w-lipgloss.Width(text), 0, w-1)
		}
		rows[row] = append(rows[row], mark{col: col, text: text, style: style})
	}
	lines := make([]string, radarHeight)
	for r := 0; r < radarHeight; r++ {
		ms := rows[r]
		sort.Slice(ms, func(i, j int) bool { return ms[i].col < ms[j].col })
		var b strings.Builder
		cur := 0
		for _, m := range ms {
			if m.col < cur {
				continue
			}
			b.WriteString(strings.Repeat(" ", m.col-cur))
			b.WriteString(m.style.Render(m.text))
			cur = m.col + lipgloss.Width(m.text)
		}
		if cur < w {
			b.WriteString(strings.Repeat(" ", w-cur))
		}
		lines[r] = b.String()
	}
	return strings.Join(lines, "\n")
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
		lines = append(lines, theme.Amber.Render("Belt depleted — press Q to dock and chart a new course"))
	} else {
		lines = append(lines, g.renderRadar(&st), "")
		for i, rock := range st.Belt {
			marker := "  "
			sel := i == g.rockSel
			if sel {
				marker = "▸ "
			}
			var line string
			if rock.Scanned {
				tierGlyph := g.content.Tiers.Glyphs[rock.Tier]
				line = fmt.Sprintf("%s%-8s %5.1fkm  %s %-9s VOL %4d  ⏱ %4.1fs  %s %5d  %s",
					marker, rock.Name, rock.Distance, tierGlyph, g.content.Tiers.Labels[rock.Tier],
					rock.Volume, rock.DrillSec, theme.Glyph("credit", st.Settings.ASCIISafe), rock.Value, theme.Dots(rock.Dots, 5))
			} else {
				line = fmt.Sprintf("%s%-8s %5.1fkm  %s",
					marker, rock.Name, rock.Distance, theme.DimStyle.Render("UNKNOWN"))
			}
			var style lipgloss.Style
			switch {
			case sel && rock.Scanned:
				style = theme.TierStyle(rock.Tier, st.Settings.ASCIISafe).Bold(true).Background(lipgloss.Color(lipglossColorForTier(rock.Tier)))
			case sel:
				style = theme.White.Bold(true).Background(lipgloss.Color(lipglossColorForUnknown))
			case rock.Scanned:
				style = theme.TierStyleDim(rock.Tier)
			default:
				style = theme.DimStyle
			}
			rendered := style.Render(line)
			lines = append(lines, rendered)
			g.hitBodyLine(len(lines)-1, 1, rendered, fmt.Sprintf("belt:%d", i), i)
		}
	}
	if g.rockSel < len(st.Belt) {
		rock := st.Belt[g.rockSel]
		switch {
		case st.Scan != nil && st.Scan.AsteroidID == rock.ID:
			pct := 0.0
			if st.Scan.Duration > 0 {
				pct = st.Scan.Elapsed / st.Scan.Duration * 100
			}
			remaining := st.Scan.Duration - st.Scan.Elapsed
			if remaining < 0 {
				remaining = 0
			}
			lines = append(lines, "",
				fmt.Sprintf("SCANNING %s  %s  %.1fs remaining", theme.Gold.Render(rock.Name), theme.RampBar(pct, 24, false), remaining),
				theme.DimStyle.Render("[Q] DOCK"))
		case rock.Scanned:
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
				theme.DimStyle.Render("[V] VIEW  [Q] DOCK"))
		default:
			scanFuelStr := fmt.Sprintf("%.0f fuel", g.content.Belt.ScanFuelCost)
			if g.content.Belt.ScanFuelCost > st.Fuel {
				scanFuelStr = theme.Red.Render(scanFuelStr)
			} else {
				scanFuelStr = theme.White.Render(scanFuelStr)
			}
			scanSecs := rock.Distance * g.content.Belt.ScanSecPerKm
			scanLine := fmt.Sprintf("CONTACT: %s  DISTANCE %.1fkm  SCAN COST %s  TIME %.1fs",
				theme.White.Render(rock.Name), rock.Distance, scanFuelStr, scanSecs)
			lines = append(lines, "", scanLine,
				theme.Button("S", "SCAN ASTEROID", "", true, theme.HueCyan),
				theme.DimStyle.Render("[V] VIEW  [Q] DOCK"))
			g.hitBodyLine(len(lines)-2, 1, lines[len(lines)-2], "btn:scan", nil)
		}
	}
	body := strings.Join(lines, "\n")
	hint := "↑/↓ SELECT · S SCAN · ENTER LOCK · V VIEW · Q DOCK"
	return body + "\n" + g.renderKeybar(hint)
}

const lipglossColorForUnknown = "#12324a"

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
