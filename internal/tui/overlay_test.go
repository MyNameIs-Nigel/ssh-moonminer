package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
	"github.com/mynameis-nigel/ssh-moonminer/internal/identity"
	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/hitbox"
)

func TestCompositeViewCentersOverlay(t *testing.T) {
	c, err := content.Load("")
	if err != nil {
		t.Fatal(err)
	}
	g := &Game{
		content: c,
		width:   80,
		height:  24,
		scr:     scrChart,
		overlay: ovHelp,
		snap:    sim.Snapshot{State: *sim.New(c, 1, 1000)},
		id:      identity.SessionIdentity{Slot: "test"},
		hits:    hitbox.New(),
		tickCount: 1,
	}
	base := g.renderChrome() + "\n" + g.renderScreen()
	out := g.compositeView(base)
	lines := strings.Split(out, "\n")
	if len(lines) != 24 {
		t.Fatalf("composite height: got %d want 24", len(lines))
	}
	foundHelp := false
	for _, line := range lines {
		if strings.Contains(line, "HELP") {
			foundHelp = true
			break
		}
	}
	if !foundHelp {
		t.Fatalf("help panel not visible in composite output")
	}
}

func TestCompositeViewPreservesBackgroundBesideOverlay(t *testing.T) {
	c, err := content.Load("")
	if err != nil {
		t.Fatal(err)
	}
	g := &Game{
		content:   c,
		width:     80,
		height:    24,
		scr:       scrChart,
		overlay:   ovHelp,
		snap:      sim.Snapshot{State: *sim.New(c, 1, 1000)},
		id:        identity.SessionIdentity{Slot: "test"},
		hits:      hitbox.New(),
		tickCount: 1,
	}
	base := g.renderChrome() + "\n" + g.renderScreen()
	baseLines := strings.Split(base, "\n")
	out := g.compositeView(base)
	outLines := strings.Split(out, "\n")

	ov := g.renderOverlay()
	ovLines := strings.Split(ov, "\n")
	ovW, ovH := overlayBounds(ovLines)
	y0 := (g.height - ovH) / 2
	x0 := (g.width - ovW) / 2

	// Pick a row inside the overlay's vertical span but check the columns to
	// its left, which should still show the original background text rather
	// than being blanked out.
	row := y0 + ovH/2
	if row < 0 || row >= len(baseLines) || row >= len(outLines) {
		t.Fatalf("row %d out of range", row)
	}
	wantLeft := strings.TrimRight(ansi.Cut(baseLines[row], 0, x0), " ")
	if wantLeft == "" {
		t.Skip("no non-blank background content to the left of the overlay on this row")
	}
	gotLeft := strings.TrimRight(ansi.Strip(ansi.Cut(outLines[row], 0, x0)), " ")
	wantLeftPlain := strings.TrimRight(ansi.Strip(wantLeft), " ")
	if gotLeft != wantLeftPlain {
		t.Fatalf("background left of overlay was blanked: got %q want %q", gotLeft, wantLeftPlain)
	}
}
