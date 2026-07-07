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

// Button renders an interactive button label.
func Button(key, label, price string, enabled bool) string {
	if !enabled {
		return DimStyle.Render("[" + key + "] " + label + "  " + price)
	}
	return Bright.Render("["+key+"]") + " " + TxtStyle.Render(label) + "  " + Amber.Render(price)
}

// Cursor renders blinking footer cursor.
func Cursor(tick int) string {
	if tick%2 == 0 {
		return Bright.Render("█")
	}
	return " "
}
