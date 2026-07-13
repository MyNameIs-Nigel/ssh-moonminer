package tui

import (
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"

	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/theme"
)

// chartFooterLines renders the explanatory text at the bottom of a chart
// panel. The default is a ticker-style marquee so no part of a long sentence
// silently disappears; the TWEAKS setting exchanges that motion for a normal
// word-wrapped paragraph.
func (g *Game) chartFooterLines(text string, width, maxRows int) []string {
	if width < 1 || maxRows < 1 || text == "" {
		return nil
	}
	if g.snap.State.Settings.WrapLongText {
		lines := wrapChartText(text, width)
		if len(lines) > maxRows {
			lines = lines[:maxRows]
		}
		for i := range lines {
			lines[i] = theme.DimStyle.Render(lines[i])
		}
		return lines
	}
	if g.snap.State.Settings.ReducedMotion {
		return []string{theme.DimStyle.Render(truncateChartText(text, width))}
	}
	return []string{theme.DimStyle.Render(marqueeChartText(text, width, g.tickCount))}
}

// bottomAlignPanelLines reserves the final physical rows of a panel for its
// description or guidance. Keeping those lines at the same baseline lets the
// two chart columns read as a deliberate pair even when the lists above them
// have different lengths.
func bottomAlignPanelLines(top, bottom []string, rows int) []string {
	if rows < 1 {
		return nil
	}
	if len(bottom) > rows {
		bottom = bottom[len(bottom)-rows:]
	}
	if len(top)+len(bottom) > rows {
		top = top[:max(0, rows-len(bottom))]
	}
	out := make([]string, 0, rows)
	out = append(out, top...)
	out = append(out, make([]string, max(0, rows-len(top)-len(bottom)))...)
	out = append(out, bottom...)
	return out
}

func marqueeChartText(text string, width, tick int) string {
	if width < 1 || text == "" {
		return ""
	}
	if lipgloss.Width(text) <= width {
		return text
	}
	// Chart descriptions are authored as terminal-width ASCII prose today;
	// rune indexing still keeps the component safe if content gains Unicode.
	loop := []rune(text + "   ·   ")
	if len(loop) == 0 {
		return ""
	}
	start := tick % len(loop)
	out := make([]rune, 0, width)
	for i := 0; len(out) < width; i++ {
		out = append(out, loop[(start+i)%len(loop)])
	}
	return string(out)
}

func truncateChartText(text string, width int) string {
	if width < 1 || text == "" {
		return ""
	}
	if lipgloss.Width(text) <= width {
		return text
	}
	if width == 1 {
		return "…"
	}
	runes := []rune(text)
	if len(runes) >= width {
		return string(runes[:width-1]) + "…"
	}
	return text
}

func wrapChartText(text string, width int) []string {
	if width < 1 || text == "" {
		return nil
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	var lines []string
	line := ""
	for _, word := range words {
		for lipgloss.Width(word) > width {
			part, rest := splitChartWord(word, width)
			if line != "" {
				lines = append(lines, line)
				line = ""
			}
			lines = append(lines, part)
			word = rest
		}
		if word == "" {
			continue
		}
		if line == "" {
			line = word
			continue
		}
		if lipgloss.Width(line)+1+lipgloss.Width(word) <= width {
			line += " " + word
		} else {
			lines = append(lines, line)
			line = word
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}

func splitChartWord(word string, width int) (part, rest string) {
	if width < 1 {
		return "", word
	}
	end := 0
	used := 0
	for end < len(word) {
		r, size := utf8.DecodeRuneInString(word[end:])
		rw := lipgloss.Width(string(r))
		if used+rw > width {
			break
		}
		used += rw
		end += size
	}
	if end == 0 {
		_, end = utf8.DecodeRuneInString(word)
	}
	return word[:end], word[end:]
}
