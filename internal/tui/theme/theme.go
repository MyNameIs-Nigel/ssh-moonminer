package theme

import (
	"strings"

	"charm.land/lipgloss/v2"
)

var (
	lineC   = "16456e"
	txtC    = "bfe2ff"
	dimC    = "5e87aa"
	brightC = "3fb6ff"
	whiteC  = "dce9f5"
	cyanC   = "3fe0ff"
	violetC = "b48fff"
	goldC   = "ffcf5e"
	redC    = "ff5a6a"
	amberC  = "ffae3f"
	emptyC  = "0d3252"
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

// GaugeStyle returns bar color for percentage.
func GaugeStyle(pct float64, invert bool) lipgloss.Style {
	if invert {
		if pct >= 75 {
			return lipgloss.NewStyle().Foreground(lipgloss.Color(redC))
		}
		if pct >= 50 {
			return lipgloss.NewStyle().Foreground(lipgloss.Color(amberC))
		}
	}
	if pct < 20 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(redC))
	}
	if pct < 40 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(amberC))
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(brightC))
}

var (
	Selected = lipgloss.NewStyle().Foreground(lipgloss.Color(brightC)).Bold(true)
	DimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(dimC))
	TxtStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(txtC))
	Bright   = lipgloss.NewStyle().Foreground(lipgloss.Color(brightC))
	Amber    = lipgloss.NewStyle().Foreground(lipgloss.Color(amberC))
	Red      = lipgloss.NewStyle().Foreground(lipgloss.Color(redC))
)

// Panel renders a notched box panel.
func Panel(title string, w, h int, body string) string {
	if w < 4 || h < 3 {
		return body
	}
	border := lipgloss.NewStyle().Foreground(lipgloss.Color(lineC))
	titleStr := DimStyle.Render(Glyph("diamond", false) + " " + title)
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
