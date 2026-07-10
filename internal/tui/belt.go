package tui

import (
	"fmt"
	"math"
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

type radarMark struct {
	col   int
	text  string
	style lipgloss.Style
}

// renderMarkGrid lays out marks on a row/col grid, resolving overlaps by
// nudging later marks past earlier ones on the same row. Shared by the
// ore-scan and radar-scope belt views so both stay collision-free.
func renderMarkGrid(w, h int, rows [][]radarMark) string {
	lines := make([]string, h)
	for r := 0; r < h; r++ {
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

func sizeGlyph(size string) string {
	switch size {
	case "lg":
		return "▓"
	case "md":
		return "▒"
	default:
		return "░"
	}
}

// renderOreScan draws each contact as a Size-shaped blob at its belt X,Y
// coordinates. Blobs are hollow/dim until selected; the selected blob fills
// in and gains its designation label — in its true tier color once scanned,
// or a neutral "target lock" color if its details are still unknown.
func (g *Game) renderOreScan(st *sim.State) string {
	w := g.width
	if w < 24 {
		w = 24
	}
	rows := make([][]radarMark, radarHeight)
	for i, rock := range st.Belt {
		col := clampInt(rock.X*(w-1)/100, 0, w-1)
		row := clampInt(rock.Y*(radarHeight-1)/100, 0, radarHeight-1)
		sel := i == g.rockSel
		glyph := sizeGlyph(rock.Size)
		style := theme.DimStyle
		text := glyph
		if sel {
			style = theme.White
			if rock.Scanned {
				style = theme.TierStyle(rock.Tier, st.Settings.ASCIISafe)
			}
			text = glyph + " " + rock.Name
		} else if rock.Scanned {
			style = theme.TierStyleDim(rock.Tier)
		}
		if col+lipgloss.Width(text) > w {
			col = clampInt(w-lipgloss.Width(text), 0, w-1)
		}
		rows[row] = append(rows[row], radarMark{col: col, text: text, style: style})
	}
	return renderMarkGrid(w, radarHeight, rows)
}

// renderRadarScope draws contacts as bearing/range blips on a circular
// scope centered on the ship, with a sweep line that rotates one step per
// UI tick and a crosshair + callout on the selected contact. Distinct from
// renderOreScan: bearing/range are derived from each rock's X,Y instead of
// placing blobs directly at their belt coordinates.
func (g *Game) renderRadarScope(st *sim.State) string {
	w := g.width
	if w < 24 {
		w = 24
	}
	h := radarHeight
	cx, cy := w/2, (h-1)/2
	rx, ry := float64(cx-1), float64(cy)
	if rx < 1 {
		rx = 1
	}
	if ry < 1 {
		ry = 1
	}

	rows := make([][]radarMark, h)
	// Ambient gridmarks: a ring plus the sweep line, both under the blips.
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			nx, ny := (float64(c)-float64(cx))/rx, (float64(r)-float64(cy))/ry
			d := math.Hypot(nx, ny)
			if d > 0.85 && d <= 1.0 {
				rows[r] = append(rows[r], radarMark{col: c, text: "·", style: theme.DimStyle})
			}
		}
	}
	if !st.Settings.ReducedMotion {
		sweep := float64(g.tickCount%36) * (2 * math.Pi / 36)
		for t := 0.0; t < 1.0; t += 0.08 {
			c := cx + int(math.Round(t*rx*math.Cos(sweep)))
			r := cy + int(math.Round(t*ry*math.Sin(sweep)))
			if c >= 0 && c < w && r >= 0 && r < h {
				rows[r] = append(rows[r], radarMark{col: c, text: "·", style: theme.DimStyle})
			}
		}
	}

	maxDist := 0.0
	for _, rock := range st.Belt {
		if rock.Distance > maxDist {
			maxDist = rock.Distance
		}
	}
	if maxDist <= 0 {
		maxDist = 1
	}
	for i, rock := range st.Belt {
		bearing := math.Atan2(float64(rock.Y-50), float64(rock.X-50))
		radius := clampF(rock.Distance/maxDist, 0.15, 1.0)
		col := cx + int(math.Round(radius*rx*math.Cos(bearing)))
		row := cy + int(math.Round(radius*ry*math.Sin(bearing)))
		col = clampInt(col, 0, w-1)
		row = clampInt(row, 0, h-1)

		sel := i == g.rockSel
		glyph := "◉"
		style := theme.DimStyle
		text := glyph
		if rock.Scanned {
			style = theme.TierStyleDim(rock.Tier)
		}
		if sel {
			glyph = "┼"
			style = theme.White
			if rock.Scanned {
				style = theme.TierStyle(rock.Tier, st.Settings.ASCIISafe)
			}
			text = glyph + " " + rock.Name
		}
		if col+lipgloss.Width(text) > w {
			col = clampInt(w-lipgloss.Width(text), 0, w-1)
		}
		rows[row] = append(rows[row], radarMark{col: col, text: text, style: style})
	}
	// Ship anchor at scope center, drawn last so it always shows through.
	rows[cy] = append(rows[cy], radarMark{col: cx, text: "▲", style: theme.Bright})
	return renderMarkGrid(w, h, rows)
}

func (g *Game) renderBelt() string {
	st := g.snap.State
	w := g.content.WorldByIndex(st.WorldIdx)
	worldName := "UNKNOWN"
	if w != nil {
		worldName = w.Name
	}
	lines := []string{theme.Gold.Render("◇ ASTEROID BELT — " + worldName)}
	lines = append(lines, g.renderShieldStatus(&st, 20))
	if len(st.Belt) == 0 {
		lines = append(lines, theme.Amber.Render("Belt depleted — press Q to dock and chart a new course"))
	} else {
		switch st.Settings.BeltView {
		case sim.BeltViewOreScan:
			lines = append(lines, theme.DimStyle.Render("ORE SCAN"), g.renderOreScan(&st), "")
		case sim.BeltViewRadar:
			lines = append(lines, theme.DimStyle.Render("RADAR SCOPE"), g.renderRadarScope(&st), "")
		default:
			lines = append(lines, theme.DimStyle.Render("DATA TILES"))
		}
		for i, rock := range st.Belt {
			marker := "  "
			sel := i == g.rockSel
			if sel {
				marker = "▸ "
			}
			outOfRange := !rock.Scanned && sim.IsOutOfRange(&st, g.content, &rock)
			var line string
			switch {
			case rock.Scanned:
				tierGlyph := g.content.Tiers.Glyphs[rock.Tier]
				line = fmt.Sprintf("%s%-8s %5.1fkm  %s %-9s VOL %4d  ⏱ %4.1fs  %s %5d  %s",
					marker, rock.Name, rock.Distance, tierGlyph, g.content.Tiers.Labels[rock.Tier],
					rock.Volume, rock.DrillSec, theme.Glyph("credit", st.Settings.ASCIISafe), rock.Value, theme.Dots(rock.Dots, 5))
			case outOfRange:
				line = fmt.Sprintf("%s%-8s %5.1fkm  %s",
					marker, rock.Name, rock.Distance, theme.Red.Render("OUT OF RANGE"))
			default:
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
			case outOfRange:
				style = theme.DimStyle
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
		case sim.IsOutOfRange(&st, g.content, &rock):
			lockKm := sim.ScannerLockKm(&st, g.content)
			lines = append(lines, "",
				fmt.Sprintf("CONTACT: %s  DISTANCE %.1fkm", theme.White.Render(rock.Name), rock.Distance),
				theme.Red.Render(fmt.Sprintf("OUT OF RANGE — SCANNER REACHES %.0fkm", lockKm)),
				theme.DimStyle.Render("Upgrade Scanner grade at the shipyard to reach farther contacts."),
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
