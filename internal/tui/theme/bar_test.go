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
	hint := "↑/↓ SELECT · ENTER DEPART · TAB PANE · ESC/Q CHART · F REFUEL · ? HELP"
	out := theme.KeybarHint(hint)
	for _, word := range []string{"SELECT", "DEPART", "PANE", "CHART", "REFUEL", "HELP"} {
		if !strings.Contains(out, word) {
			t.Fatalf("expected label %q in output, got %q", word, out)
		}
	}
	for _, key := range []string{"↑", "↓", "ENTER", "TAB", "ESC/Q", "F", "?"} {
		if !strings.Contains(out, key) {
			t.Fatalf("expected control key %q in output, got %q", key, out)
		}
	}
	for _, key := range []string{"TAB", "ESC/Q"} {
		if !strings.Contains(out, theme.Bright.Render(key)) {
			t.Fatalf("expected the whole %q token to use the control-key style: %q", key, out)
		}
	}
}

func TestShipMarkerMatchesShipClass(t *testing.T) {
	for class, want := range map[string]string{
		"miner": "■", "fighter": "▲", "freighter": "█",
	} {
		if got := theme.ShipMarker(class); !strings.Contains(got, want) {
			t.Fatalf("%s ship marker = %q, want %q", class, got, want)
		}
	}
}
