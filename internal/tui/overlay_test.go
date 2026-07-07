package tui

import (
	"strings"
	"testing"

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
