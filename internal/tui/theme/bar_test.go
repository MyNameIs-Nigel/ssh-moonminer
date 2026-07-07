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

func TestKeybarHintStylesControlKeysNotLabelFirstLetters(t *testing.T) {
	hint := "↑/↓ SELECT · ENTER DEPART · F REFUEL · ? HELP"
	out := theme.KeybarHint(hint)
	for _, word := range []string{"SELECT", "DEPART", "REFUEL", "HELP"} {
		if !strings.Contains(out, word) {
			t.Fatalf("expected label %q in output, got %q", word, out)
		}
	}
	for _, key := range []string{"↑", "↓", "ENTER", "F", "?"} {
		if !strings.Contains(out, key) {
			t.Fatalf("expected control key %q in output, got %q", key, out)
		}
	}
}

func TestCursorReducedMotion(t *testing.T) {
	blink := theme.Cursor(0, false)
	steady := theme.Cursor(0, true)
	if steady == " " {
		t.Fatal("reduced motion cursor should be visible")
	}
	if blink == steady && theme.Cursor(1, false) == blink {
		t.Fatal("expected blink cursor to alternate when motion enabled")
	}
}
