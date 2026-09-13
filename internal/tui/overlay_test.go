package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
	"github.com/mynameis-nigel/ssh-moonminer/internal/identity"
	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/hitbox"
	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/theme"
)

func TestCompositeViewCentersOverlay(t *testing.T) {
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
	y0 := chromeH + (g.bodyHeight()-ovH)/2
	x0 := (g.contentWidth() - ovW) / 2

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

func TestOnboardingOverlayShowsEveryReflowedLineAtMinimumViewport(t *testing.T) {
	g := newResponsiveTestGame(t, minWidth, minHeight)
	g.overlay = ovOnboard
	panelW, _ := g.layoutOverlaySize(70, 6)
	for pg, page := range onboardPages {
		g.onboardPg = pg
		out := ansi.Strip(g.View().Content)
		for _, line := range reflowOverlayLines([]string{page}, panelW-4) {
			if text := ansi.Strip(line); text != "" && !strings.Contains(out, text) {
				t.Fatalf("page %d clipped onboarding text %q at %dx%d", pg, text, minWidth, minHeight)
			}
		}
	}
}

func TestReflowOverlayLinesPreservesStyle(t *testing.T) {
	long := strings.Repeat("warning ", 8)
	styled := theme.Red.Render(long)
	lines := reflowOverlayLines([]string{styled}, 20)
	if len(lines) < 2 {
		t.Fatalf("expected wrapped lines, got %d: %#v", len(lines), lines)
	}
	for i, line := range lines {
		if ansi.Strip(line) == "" {
			t.Fatalf("line %d lost visible text: %q", i, line)
		}
		if !strings.HasPrefix(line, "\x1b[") {
			t.Fatalf("line %d lost style prefix: %q", i, line)
		}
	}
}
