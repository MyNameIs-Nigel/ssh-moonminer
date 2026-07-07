package theme_test

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/theme"
)

func TestHexColorsEmitANSI(t *testing.T) {
	// Lipgloss v2 requires a leading # on hex colors; bare hex silently becomes no-color.
	out := theme.Cyan.Render("test")
	if !strings.Contains(out, "\x1b[") {
		t.Fatalf("expected ANSI escape in rendered output, got %q", out)
	}
	if out == "test" {
		t.Fatal("color style produced no formatting")
	}
}

func TestBareHexIsNoColor(t *testing.T) {
	// Document the lipgloss v2 footgun we hit.
	bad := lipgloss.NewStyle().Foreground(lipgloss.Color("3fb6ff")).Render("x")
	if strings.Contains(bad, "\x1b[") {
		t.Fatalf("bare hex should not produce ANSI, got %q", bad)
	}
	good := lipgloss.NewStyle().Foreground(lipgloss.Color("#3fb6ff")).Render("x")
	if !strings.Contains(good, "\x1b[") {
		t.Fatalf("prefixed hex should produce ANSI, got %q", good)
	}
}
