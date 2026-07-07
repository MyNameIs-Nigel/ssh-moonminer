package tui

import (
	"strings"
	"testing"

	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/theme"
)

func TestHighContrastSelectionPreservesNestedColor(t *testing.T) {
	fuel := theme.Amber.Render("⛽ 6")
	namePart := "▸ VESTA  MID BELT"
	line := theme.OptionHC(theme.HueCyan, true, true).Render(namePart) + "  " + fuel
	if hasBareANSICode(line) {
		t.Fatalf("selection style leaked raw ANSI when fuel is appended separately:\n%s", line)
	}
}

func hasBareANSICode(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] != '[' {
			continue
		}
		if i > 0 && s[i-1] == '\x1b' {
			continue
		}
		if strings.HasPrefix(s[i:], "[38;") || strings.HasPrefix(s[i:], "[0m") || strings.HasPrefix(s[i:], "[1m") {
			return true
		}
	}
	return false
}
