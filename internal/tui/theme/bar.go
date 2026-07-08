package theme

import (
	"strings"
	"unicode/utf8"

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

// SweepBar draws the mining skill-check "drill calibration" gauge: a
// highlighted target zone and a moving marker glyph. pos, zoneStart, and
// zoneWidth are all 0..1 along the gauge. Pass showMarker=false (reduced
// motion) to draw only the static zone, with no moving marker.
func SweepBar(pos, zoneStart, zoneWidth float64, width int, showMarker bool) string {
	if width < 1 {
		return ""
	}
	clamp01 := func(v float64) float64 {
		if v < 0 {
			return 0
		}
		if v > 1 {
			return 1
		}
		return v
	}
	zoneLo := int(clamp01(zoneStart) * float64(width))
	zoneHi := int(clamp01(zoneStart+zoneWidth) * float64(width))
	marker := -1
	if showMarker {
		marker = int(clamp01(pos) * float64(width))
		if marker >= width {
			marker = width - 1
		}
	}

	zoneStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(greenC))
	trackStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(emptyC))
	markerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(goldC)).Bold(true)

	var b strings.Builder
	for i := 0; i < width; i++ {
		switch {
		case i == marker:
			b.WriteString(markerStyle.Render("▲"))
		case i >= zoneLo && i < zoneHi:
			b.WriteString(zoneStyle.Render("▓"))
		default:
			b.WriteString(trackStyle.Render("░"))
		}
	}
	return b.String()
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
func Cursor(tick int, reducedMotion bool) string {
	if reducedMotion {
		return Bright.Render("█")
	}
	if tick%2 == 0 {
		return Bright.Render("█")
	}
	return " "
}

// KeybarHint renders a keybar hint with bright control keys and lighter labels.
func KeybarHint(hint string) string {
	segments := strings.Split(hint, "·")
	var out strings.Builder
	for i, seg := range segments {
		if i > 0 {
			out.WriteString(DimStyle.Render(" ·"))
		}
		seg = strings.TrimLeft(seg, " ")
		key, label := splitHintSegment(seg)
		if key != "" {
			out.WriteString(Bright.Render(key))
		}
		if label != "" {
			out.WriteString(TxtStyle.Render(label))
		}
	}
	return out.String()
}

func splitHintSegment(seg string) (key, label string) {
	if seg == "" {
		return "", ""
	}
	if seg[0] == '[' {
		if idx := strings.Index(seg, "]"); idx >= 0 {
			return seg[:idx+1], seg[idx+1:]
		}
	}
	if r, _ := utf8.DecodeRuneInString(seg); r == '↑' || r == '↓' || r == '←' || r == '→' {
		i := 0
		for i < len(seg) {
			r, size := utf8.DecodeRuneInString(seg[i:])
			if r == '↑' || r == '↓' || r == '←' || r == '→' || r == '/' {
				i += size
				continue
			}
			break
		}
		return seg[:i], seg[i:]
	}
	for _, mk := range []string{"ENTER", "SPACE", "ESC"} {
		if strings.HasPrefix(seg, mk) {
			rest := seg[len(mk):]
			if rest == "" || rest[0] == ' ' {
				return mk, rest
			}
		}
	}
	r, size := utf8.DecodeRuneInString(seg)
	if r == '?' || (r >= 'A' && r <= 'Z') {
		return seg[:size], seg[size:]
	}
	return "", seg
}
