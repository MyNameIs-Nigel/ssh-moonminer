package theme

import (
	"strings"

	"charm.land/lipgloss/v2"
)

var (
	bgC        = "#04101f"
	lineC      = "#16456e"
	txtC       = "#bfe2ff"
	dimC       = "#5e87aa"
	brightC    = "#3fb6ff"
	whiteC     = "#dce9f5"
	cyanC      = "#3fe0ff"
	cyanDimC   = "#1a7a99"
	violetC    = "#b48fff"
	violetDimC = "#6b4fa8"
	goldC      = "#ffcf5e"
	goldDimC   = "#a67c2e"
	greenC     = "#5eff8a"
	greenDimC  = "#2a8a4a"
	redC       = "#ff5a6a"
	amberC     = "#ffae3f"
	amberDimC  = "#a66a1a"
	emptyC     = "#0d3252"
)

// Hue identifies a semantic color family for Option() styling.
type Hue int

const (
	HueCyan Hue = iota
	HueViolet
	HueGold
	HueGreen
	HueAmber
	HueRed
)

var tierHex = []string{whiteC, cyanC, violetC, goldC}

// Glyph returns tier/symbol with ASCII-safe fallback.
func Glyph(name string, asciiSafe bool) string {
	if !asciiSafe {
		switch name {
		case "diamond":
			return "◇"
		case "credit":
			return "◈"
		case "fuel":
			return "⛽"
		case "skull":
			return "☠"
		case "drill":
			return "⛏"
		case "common":
			return "◇"
		case "uncommon":
			return "◆"
		case "rare":
			return "✦"
		case "legendary":
			return "★"
		}
		return "*"
	}
	switch name {
	case "diamond", "common":
		return "*"
	case "credit":
		return "$"
	case "fuel":
		return "F"
	case "skull":
		return "!"
	case "drill":
		return "D"
	case "uncommon":
		return "+"
	case "rare":
		return "#"
	case "legendary":
		return "@"
	}
	return "*"
}

// TierStyle returns color style for tier index.
func TierStyle(tier int, _ bool) lipgloss.Style {
	if tier < 0 || tier >= len(tierHex) {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(txtC))
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(tierHex[tier]))
}

// TierStyleDim returns a muted tier color for unselected list rows.
func TierStyleDim(tier int) lipgloss.Style {
	if tier < 0 || tier >= len(tierHex) {
		return DimStyle
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(tierHex[tier])).Faint(true)
}

// OutcomeStyle returns style for outcome kind.
func OutcomeStyle(kind string) lipgloss.Style {
	switch kind {
	case "clean":
		return lipgloss.NewStyle().Foreground(lipgloss.Color(goldC)).Bold(true)
	case "bail":
		return lipgloss.NewStyle().Foreground(lipgloss.Color(cyanC)).Bold(true)
	case "raided":
		return lipgloss.NewStyle().Foreground(lipgloss.Color(redC)).Bold(true)
	case "stranded":
		return lipgloss.NewStyle().Foreground(lipgloss.Color(amberC)).Bold(true)
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(txtC))
	}
}

// OutcomeAccent returns the panel border accent for an outcome kind.
func OutcomeAccent(kind string) string {
	switch kind {
	case "clean":
		return goldC
	case "bail":
		return cyanC
	case "raided":
		return redC
	case "stranded":
		return amberC
	default:
		return brightC
	}
}

// GaugeStyle returns bar color for percentage.
func GaugeStyle(pct float64, invert bool) lipgloss.Style {
	if invert {
		if pct >= 75 {
			return lipgloss.NewStyle().Foreground(lipgloss.Color(redC))
		}
		if pct >= 50 {
			return lipgloss.NewStyle().Foreground(lipgloss.Color(amberC))
		}
		return lipgloss.NewStyle().Foreground(lipgloss.Color(greenC))
	}
	if pct < 20 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(redC))
	}
	if pct < 40 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(amberC))
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(brightC))
}

// DrillStyle returns color for drill progress (filling = good).
func DrillStyle(pct float64) lipgloss.Style {
	if pct >= 75 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(greenC))
	}
	if pct >= 40 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(cyanC))
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(brightC))
}

// FuelStyle returns color for fuel level — white when healthy, amber warning, red danger.
func FuelStyle(pct float64) lipgloss.Style {
	if pct < 20 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(redC)).Bold(true)
	}
	if pct < 35 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(amberC))
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(whiteC))
}

// HullStyle returns color for hull integrity.
func HullStyle(pct float64) lipgloss.Style {
	if pct < 25 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(redC)).Bold(true)
	}
	if pct < 50 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(amberC))
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(greenC))
}

func hueColors(h Hue) (dim, bright, bg string) {
	switch h {
	case HueViolet:
		return violetDimC, violetC, "#2a1a4a"
	case HueGold:
		return goldDimC, goldC, "#3a2a10"
	case HueGreen:
		return greenDimC, greenC, "#0a2a18"
	case HueAmber:
		return amberDimC, amberC, "#3a2208"
	case HueRed:
		return "#992a3a", redC, "#3a0a14"
	default:
		return cyanDimC, cyanC, "#0a2a3a"
	}
}

// Option returns a style for selectable items — dim when unselected, bright + bg when selected.
func Option(h Hue, selected bool) lipgloss.Style {
	dim, bright, bg := hueColors(h)
	if selected {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(bright)).Bold(true).Background(lipgloss.Color(bg))
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(dim))
}

// Accent returns the bright hex for a hue (panel borders, HUD rules).
func Accent(h Hue) string {
	_, bright, _ := hueColors(h)
	return bright
}

var (
	Selected = lipgloss.NewStyle().Foreground(lipgloss.Color(brightC)).Bold(true)
	DimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(dimC))
	TxtStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(txtC))
	Bright   = lipgloss.NewStyle().Foreground(lipgloss.Color(brightC))
	Amber    = lipgloss.NewStyle().Foreground(lipgloss.Color(amberC))
	Red      = lipgloss.NewStyle().Foreground(lipgloss.Color(redC))
	Gold     = lipgloss.NewStyle().Foreground(lipgloss.Color(goldC))
	Green    = lipgloss.NewStyle().Foreground(lipgloss.Color(greenC))
	Violet   = lipgloss.NewStyle().Foreground(lipgloss.Color(violetC))
	Cyan     = lipgloss.NewStyle().Foreground(lipgloss.Color(cyanC))
	White    = lipgloss.NewStyle().Foreground(lipgloss.Color(whiteC))
)

// Panel renders a notched box panel with an accent-colored border.
func Panel(title string, w, h int, body string, accent string) string {
	if w < 4 || h < 3 {
		return body
	}
	if accent == "" {
		accent = lineC
	}
	border := lipgloss.NewStyle().Foreground(lipgloss.Color(accent))
	titleStr := lipgloss.NewStyle().Foreground(lipgloss.Color(accent)).Bold(true).Render(Glyph("diamond", false) + " " + title)
	top := "┌" + titleStr + strings.Repeat("─", max(0, w-lipgloss.Width(titleStr)-2)) + "┐"
	lines := strings.Split(body, "\n")
	var out []string
	out = append(out, border.Render(top))
	for i := 0; i < h-2; i++ {
		lineBody := ""
		if i < len(lines) {
			lineBody = lines[i]
		}
		pad := w - 2 - lipgloss.Width(lineBody)
		if pad < 0 {
			lineBody = lipgloss.NewStyle().Width(w - 2).Render(lineBody)
			pad = 0
		}
		out = append(out, border.Render("│")+lineBody+strings.Repeat(" ", pad)+border.Render("│"))
	}
	out = append(out, border.Render("└"+strings.Repeat("─", w-2)+"┘"))
	return strings.Join(out, "\n")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
