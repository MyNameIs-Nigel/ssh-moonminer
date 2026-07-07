package theme_test

import (
	"strings"
	"testing"

	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/theme"
)

func TestKeybarHintPreservesUTF8Arrows(t *testing.T) {
	out := theme.KeybarHint("↑/↓ SELECT · ENTER")
	if strings.Contains(out, "â") {
		t.Fatalf("UTF-8 arrows were corrupted: %q", out)
	}
	if !strings.Contains(out, "↑") || !strings.Contains(out, "↓") {
		t.Fatalf("expected arrow runes in output, got %q", out)
	}
}
