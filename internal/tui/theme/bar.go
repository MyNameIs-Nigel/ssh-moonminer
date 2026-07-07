package theme

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// RenderBar draws a block gauge.
func RenderBar(pct float64, width int, style lipgloss.Style) string {
	if width < 1 {
		return ""
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := int(float64(width) * pct / 100)
	if filled > width {
		filled = width
	}
	f := style.Render(strings.Repeat("█", filled))
	e := lipgloss.NewStyle().Foreground(lipgloss.Color(emptyC)).Render(strings.Repeat("█", width-filled))
	return f + e
}

// RampBar applies standard color ramp.
func RampBar(pct float64, width int, invert bool) string {
	return RenderBar(pct, width, GaugeStyle(pct, invert))
}

// FuelBar draws a fuel gauge — white when healthy, amber warning, red danger.
func FuelBar(pct float64, width int) string {
	return RenderBar(pct, width, FuelStyle(pct))
}

// HullBar draws a hull gauge — green when healthy, amber/red when damaged.
func HullBar(pct float64, width int) string {
	return RenderBar(pct, width, HullStyle(pct))
}

// DrillBar draws a drill progress gauge — cyan to green as it fills.
func DrillBar(pct float64, width int) string {
	return RenderBar(pct, width, DrillStyle(pct))
}

// Dots renders threat pips.
func Dots(n, of int) string {
	s := ""
	for i := 0; i < of; i++ {
		if i < n {
			c := brightC
			if n >= 4 {
				c = redC
			} else if n >= 3 {
				c = amberC
			}
			s += lipgloss.NewStyle().Foreground(lipgloss.Color(c)).Render("●")
		} else {
			s += DimStyle.Render("○")
		}
	}
	return s
}

// Button renders an interactive button label with hue-based styling.
func Button(key, label, price string, enabled bool, h Hue) string {
	if !enabled {
		return DimStyle.Render("[" + key + "] " + label + "  " + price)
	}
	dim, bright, _ := hueColors(h)
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(bright)).Bold(true)
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(dim))
	pricePart := ""
	if price != "" {
		pricePart = "  " + Gold.Render(price)
	}
	return keyStyle.Render("["+key+"]") + " " + labelStyle.Render(label) + pricePart
}

// Cursor renders blinking footer cursor.
func Cursor(tick int) string {
	if tick%2 == 0 {
		return Bright.Render("█")
	}
	return " "
}

// KeybarHint renders a keybar hint with bright key letters and dim separators.
func KeybarHint(hint string) string {
	runes := []rune(hint)
	var out strings.Builder
	for i, ch := range runes {
		var prev rune
		if i > 0 {
			prev = runes[i-1]
		}
		if isHintSep(ch) {
			out.WriteString(DimStyle.Render(string(ch)))
			continue
		}
		if ch == '↑' || ch == '↓' || ch == '←' || ch == '→' || ch == '▼' {
			out.WriteString(Bright.Render(string(ch)))
			continue
		}
		if ch == '?' && (i == 0 || isHintSep(prev)) {
			out.WriteString(Bright.Render("?"))
			continue
		}
		if ch >= 'A' && ch <= 'Z' && (i == 0 || isHintSep(prev) || prev == '[') {
			out.WriteString(Bright.Render(string(ch)))
			continue
		}
		out.WriteString(TxtStyle.Render(string(ch)))
	}
	return out.String()
}

func isHintSep(r rune) bool {
	return r == '·' || r == '|' || r == '/' || r == ' '
}
