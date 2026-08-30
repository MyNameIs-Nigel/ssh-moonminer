package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/hitbox"
)

func TestMiningPinnedActionsStayVisibleAtMinimumHeight(t *testing.T) {
	g := newResponsiveTestGame(t, 80, 24)
	_, run, _ := startResponsiveTestRun(t, g)
	run.SkillCheck = &sim.SkillCheck{Window: 3}
	run.ActiveEvent = &sim.RunEvent{Kind: sim.EventCargoShift, Remaining: 12}
	run.EMPActive = true
	run.FuelAsteroid = true

	out := g.View().Content
	assertSemanticAnchors(t, out, "BAIL", "PRESSURE POINT ACTIVE")
	assertHitboxAtText(t, g, out, "BAIL", "btn:bail")
}

func TestTributeActionsStayVisibleWhenClamped(t *testing.T) {
	g := newResponsiveTestGame(t, 80, 24)
	_, run, _ := startResponsiveTestRun(t, g)
	run.Phase = sim.PhaseTribute
	run.PirateID = g.content.Pirates[0].ID
	g.snap.State.CargoValue = 500
	run.CargoValue = 500

	out := g.View().Content
	assertSemanticAnchors(t, out, "DROP CARGO", "REFUSE / RUN")
	assertHitboxAtText(t, g, out, "DROP CARGO", "btn:tribute:accept")
	assertHitboxAtText(t, g, out, "REFUSE / RUN", "btn:tribute:refuse")
}

func TestCombatEscapeStaysVisibleWithLongLog(t *testing.T) {
	g := newResponsiveTestGame(t, 80, 24)
	_, run, _ := startResponsiveTestRun(t, g)
	run.Phase = sim.PhaseCombat
	run.Combat = &sim.CombatState{
		PirateName: "TEST RAIDER", PirateHull: 40, PirateMaxHull: 50,
		Bounty: 75, OddsPct: 45, Bearing: .4, Range: .6,
		Log: []string{
			"first exchange", "second exchange", "third exchange",
			"fourth exchange", "fifth exchange",
		},
	}

	out := g.View().Content
	assertSemanticAnchors(t, out, "BAIL", "TEST RAIDER")
	assertHitboxAtText(t, g, out, "BAIL", "btn:combat:escape")
}

func TestChartKeepsSelectedRouteVisibleAfterShrink(t *testing.T) {
	g := newResponsiveTestGame(t, 144, 48)
	g.worldSel = len(g.content.Worlds) - 1
	world := g.content.Worlds[g.worldSel].Name

	shrink := func(w, h int) {
		g.width = w
		g.height = h
		g.hits = hitbox.New()
	}
	for _, step := range []struct{ w, h int }{
		{144, 48}, {120, 30}, {80, 24}, {81, 24}, {144, 48},
	} {
		shrink(step.w, step.h)
		out := g.View().Content
		if !strings.Contains(ansi.Strip(out), world) {
			t.Fatalf("selected route %q missing at %dx%d", world, step.w, step.h)
		}
		assertHitboxAtText(t, g, out, world, "world:"+itoa(g.worldSel))
	}
}

func TestBeltKeepsSelectedRockRowAtMinimumViewport(t *testing.T) {
	g := newResponsiveTestGame(t, 80, 24)
	departResponsiveTestGame(t, g)
	g.scr = scrBelt
	g.rockSel = len(g.snap.State.Belt) - 1
	g.snap.State.Settings.BeltView = sim.BeltViewRadar

	out := g.View().Content
	rock := g.snap.State.Belt[g.rockSel]
	marker := "▸ " + rock.Name
	if !strings.Contains(ansi.Strip(out), strings.TrimSpace(marker)) {
		t.Fatalf("selected rock row missing at 80x24:\n%s", out)
	}
	assertHitboxAtText(t, g, out, marker, "belt:"+itoa(g.rockSel))
}

func TestBeltLockAndFlyHitbox(t *testing.T) {
	g := newResponsiveTestGame(t, 80, 24)
	departResponsiveTestGame(t, g)
	g.scr = scrBelt
	g.rockSel = 0

	out := g.View().Content
	assertHitboxAtText(t, g, out, "LOCK & FLY", "btn:lock")
}

func TestBeltScanAsteroidHitbox(t *testing.T) {
	g := newResponsiveTestGame(t, 80, 24)
	departResponsiveTestGame(t, g)
	g.scr = scrBelt
	g.rockSel = 0
	g.snap.State.Belt[0].Scanned = false

	out := g.View().Content
	assertHitboxAtText(t, g, out, "SCAN ASTEROID", "btn:scan")
}

func TestShipyardSelectionStaysVisibleAfterResize(t *testing.T) {
	g := newResponsiveTestGame(t, 144, 48)
	g.scr = scrShipyard
	g.shipyardPane = shipyardPaneLoadout
	g.shipyardHangarSel = len(g.content.Ships) - 1
	rows := shipyardRows(g.shipyardSelectedModel())
	g.shipyardRowSel = len(rows) - 1

	resize := func(h int) {
		g.height = h
		g.hits = hitbox.New()
	}
	for _, h := range []int{48, 28, 24, 32, 48} {
		resize(h)
		out := g.View().Content
		ship := g.content.Ships[g.shipyardHangarSel].Name
		if !strings.Contains(ansi.Strip(out), ship) {
			t.Fatalf("hangar selection %q missing at height %d", ship, h)
		}
	}
}

func TestShipyardAcquireHitbox(t *testing.T) {
	g := newResponsiveTestGame(t, 80, 24)
	g.scr = scrShipyard
	g.shipyardHangarSel = 1
	g.hits = hitbox.New()
	hangarW, _, _ := shipyardPaneWidths(g.contentWidth())
	_, _ = g.renderShipyardHangar(&g.snap.State, hangarW-2, g.bodyHeight()-2)
	found := false
	for y := panelBodyY(0); y < panelBodyY(g.bodyHeight()); y++ {
		if b, ok := g.hits.At(1, y); ok && b.ID == "btn:acquire" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("btn:acquire hitbox missing from hangar panel")
	}
}

func TestTweaksOverlayKeepsSelectionWhenClamped(t *testing.T) {
	g := newResponsiveTestGame(t, 80, 24)
	g.overlay = ovTweaks
	g.tweaksSel = tweakCount - 1

	out := g.View().Content
	if !strings.Contains(ansi.Strip(out), "LONG TEXT") {
		t.Fatalf("clamped tweaks overlay dropped selected row:\n%s", out)
	}
	assertHitboxAtText(t, g, out, "LONG TEXT", "tweak:"+itoa(tweakCount-1))
}

func TestChartServiceHitboxesAlignWithFinalViewport(t *testing.T) {
	for _, h := range []int{24, 30, 48} {
		g := newResponsiveTestGame(t, 80, h)
		out := g.View().Content
		assertHitboxAtText(t, g, out, "REFUEL", "svc:refuel")
		assertHitboxAtText(t, g, out, "SHIPYARD", "btn:shipyard")
	}
}

func TestGutterClicksDoNothing(t *testing.T) {
	g := newResponsiveTestGame(t, 120, 40)
	before := g.worldSel
	left, right, top, bottom := gutterClickPositions(g)
	for _, m := range []tea.MouseClickMsg{left, right, top, bottom} {
		if cmds := g.updateClick(m); cmds != nil {
			t.Fatalf("gutter click at (%d,%d) returned commands: %#v", m.X, m.Y, cmds)
		}
	}
	if g.worldSel != before {
		t.Fatalf("gutter click changed selection from %d to %d", before, g.worldSel)
	}
}

func TestResizeGuardRejectsPointerInput(t *testing.T) {
	g := newResponsiveTestGame(t, 79, 23)
	before := g.worldSel
	if cmds := g.updateClick(tea.MouseClickMsg{X: 10, Y: 10}); cmds != nil {
		t.Fatalf("resize guard click returned commands: %#v", cmds)
	}
	if g.worldSel != before {
		t.Fatalf("resize guard click changed selection from %d to %d", before, g.worldSel)
	}
	if _, ok := g.hitAt(10, 10); ok {
		t.Fatal("resize guard should not register hitboxes")
	}
}

func TestRepeatedRenderIsDeterministic(t *testing.T) {
	g := newResponsiveTestGame(t, 100, 32)
	_, run, _ := startResponsiveTestRun(t, g)
	run.Phase = sim.PhaseCombat
	run.Combat = &sim.CombatState{PirateName: "TEST", PirateMaxHull: 50, PirateHull: 25, Bounty: 10, OddsPct: 50}

	first := g.View().Content
	second := g.View().Content
	if first != second {
		t.Fatal("identical state and dimensions produced different render bytes")
	}
}

func TestCenteredScrollKeepsSelectionVisible(t *testing.T) {
	if got := centeredScroll(5, 10, 4); got != 3 {
		t.Fatalf("centeredScroll(5,10,4) = %d, want 3", got)
	}
	if got := centeredScroll(1, 10, 4); got != 0 {
		t.Fatalf("centeredScroll(1,10,4) = %d, want 0", got)
	}
	if got := centeredScroll(9, 10, 4); got != 6 {
		t.Fatalf("centeredScroll(9,10,4) = %d, want 6", got)
	}
}
