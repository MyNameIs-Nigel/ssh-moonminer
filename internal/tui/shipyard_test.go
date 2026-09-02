package tui

import (
	"strings"
	"testing"

	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
	"github.com/mynameis-nigel/ssh-moonminer/internal/identity"
	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/hitbox"
)

func shipyardTestContent(t *testing.T) *content.Content {
	t.Helper()
	c, err := content.Load("")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func newShipyardGame(t *testing.T, st *sim.State) *Game {
	t.Helper()
	return &Game{
		content:   shipyardTestContent(t),
		width:     80,
		height:    24,
		scr:       scrShipyard,
		hits:      hitbox.New(),
		snap:      sim.Snapshot{State: *st},
		id:        identity.SessionIdentity{Slot: "test"},
		tickCount: 1,
	}
}

func TestShipyardTabCyclesHangarAndLoadoutOnly(t *testing.T) {
	c := shipyardTestContent(t)
	st := sim.New(c, 1, 1000)
	g := newShipyardGame(t, st)

	if g.shipyardPane != shipyardPaneHangar {
		t.Fatalf("initial pane = %d, want hangar", g.shipyardPane)
	}
	g.keyShipyard("tab")
	if g.shipyardPane != shipyardPaneLoadout {
		t.Fatal("tab on owned active ship should focus LOADOUT")
	}
	g.keyShipyard("tab")
	if g.shipyardPane != shipyardPaneHangar {
		t.Fatal("second tab should return to HANGAR, never STATUS")
	}
	g.keyShipyard("tab")
	g.keyShipyard("shift+tab")
	if g.shipyardPane != shipyardPaneHangar {
		t.Fatal("shift+tab should always land on HANGAR")
	}
	for i := 0; i < 8; i++ {
		g.keyShipyard("tab")
		if g.shipyardPane != shipyardPaneHangar && g.shipyardPane != shipyardPaneLoadout {
			t.Fatalf("tab cycle %d entered impossible pane %d", i, g.shipyardPane)
		}
	}
}

func TestShipyardTabStaysInHangarForUnownedShip(t *testing.T) {
	c := shipyardTestContent(t)
	st := sim.New(c, 1, 1000)
	g := newShipyardGame(t, st)
	g.shipyardHangarSel = 1 // cicada, not owned on fresh save

	g.keyShipyard("tab")
	if g.shipyardPane != shipyardPaneHangar {
		t.Fatal("tab on unowned ship must not switch to LOADOUT or STATUS")
	}
}

func TestShipyardRendersAllFourShipsInHangar(t *testing.T) {
	c := shipyardTestContent(t)
	st := sim.New(c, 1, 1000)
	g := newShipyardGame(t, st)
	out := g.renderShipyard()

	for _, model := range c.Ships {
		if !strings.Contains(out, model.Name) {
			t.Errorf("expected hangar to list %s, got:\n%s", model.Name, out)
		}
	}
	if !strings.Contains(out, "ACTIVE") {
		t.Error("expected the starter skiff to render as ACTIVE")
	}
}

func TestShipyardRowsMatchShipClassSlotCounts(t *testing.T) {
	c := shipyardTestContent(t)
	for _, id := range []string{"skiff", "cicada", "warden", "mule"} {
		model := c.ShipByID(id)
		if model == nil {
			t.Fatalf("missing ship model %q", id)
		}
		rows := shipyardRows(model)
		wantWeapon := 0
		wantUtility := model.UtilitySlots
		for _, r := range rows {
			if r.kind == rowWeapon {
				wantWeapon++
			}
		}
		if wantWeapon != model.WeaponSlots {
			t.Errorf("%s: expected %d weapon rows, counted %d", id, model.WeaponSlots, wantWeapon)
		}
		gotUtility := 0
		for _, r := range rows {
			if r.kind == rowUtility {
				gotUtility++
			}
		}
		if gotUtility != wantUtility {
			t.Errorf("%s: expected %d utility rows, counted %d", id, wantUtility, gotUtility)
		}
		gotInternal := 0
		for _, r := range rows {
			if r.kind == rowInternal {
				gotInternal++
			}
		}
		if gotInternal != 1 {
			t.Errorf("%s: expected exactly 1 internal row, counted %d", id, gotInternal)
		}
		gotJumpDrive := 0
		for _, r := range rows {
			if r.kind == rowJumpDrive {
				gotJumpDrive++
			}
		}
		if gotJumpDrive != 1 {
			t.Errorf("%s: expected exactly 1 Jump Drive row, counted %d", id, gotJumpDrive)
		}
	}
	if c.ShipByID("skiff").WeaponSlots != 0 || c.ShipByID("cicada").WeaponSlots != 0 {
		t.Error("both Miner-class starter ships should have zero weapon slots")
	}
	if c.ShipByID("warden").WeaponSlots == 0 {
		t.Error("the Fighter-class ship should have at least one weapon slot")
	}
}

func TestShipyardHangarHitboxAlignment(t *testing.T) {
	c := shipyardTestContent(t)
	st := sim.New(c, 1, 1000)
	g := newShipyardGame(t, st)
	out := g.View().Content
	assertHitboxAtText(t, g, out, "▸ "+c.Ships[0].Name, "hangar:0")
}

func TestShipyardNotOwnedShipShowsBuyPrompt(t *testing.T) {
	c := shipyardTestContent(t)
	st := sim.New(c, 1, 1000)
	g := newShipyardGame(t, st)
	// Select the second ship (cicada), which isn't owned yet.
	g.shipyardHangarSel = 1
	out := g.renderShipyard()
	if !strings.Contains(out, "NOT OWNED") {
		t.Fatalf("expected a NOT OWNED prompt for an unpurchased ship, got:\n%s", out)
	}
}

func TestShipyardLoadoutShowsSelectedRowDescription(t *testing.T) {
	c := shipyardTestContent(t)
	st := sim.New(c, 1, 1000)
	g := newShipyardGame(t, st)
	g.shipyardPane = shipyardPaneLoadout
	g.shipyardRowSel = 0 // first track row: thrusters

	out := g.renderShipyard()
	// A narrow STATUS column can word-wrap the description across lines, so
	// check a fragment short enough to survive that rather than the full
	// sentence.
	if !strings.Contains(out, "Faster escapes") {
		t.Fatalf("expected STATUS to show the selected track's description, got:\n%s", out)
	}
}

func TestShipyardLoadoutShowsEmptySlotDescription(t *testing.T) {
	c := shipyardTestContent(t)
	st := sim.New(c, 1, 1000)
	g := newShipyardGame(t, st)
	g.shipyardPane = shipyardPaneLoadout
	rows := shipyardRows(c.ShipByID("skiff"))
	for i, r := range rows {
		if r.kind == rowUtility {
			g.shipyardRowSel = i
			break
		}
	}

	out := g.renderShipyard()
	if !strings.Contains(out, "Empty slot") {
		t.Fatalf("expected STATUS to prompt for an empty selected slot, got:\n%s", out)
	}
}

func TestShipyardKeyNavigationDoesNotPanic(t *testing.T) {
	c := shipyardTestContent(t)
	st := sim.New(c, 1, 1000)
	g := newShipyardGame(t, st)
	for _, k := range []string{"down", "down", "up", "tab", "shift+tab", "down", "up", "esc"} {
		g.keyShipyard(k)
		g.renderShipyard()
	}
	if g.scr != scrChart {
		t.Fatalf("expected esc to return to the star chart, got screen %v", g.scr)
	}
}

func TestShipyardSlotTargetCyclesThroughCatalog(t *testing.T) {
	c := shipyardTestContent(t)
	st := sim.New(c, 1, 1000)
	g := newShipyardGame(t, st)
	inst := st.Ships["skiff"]

	row := shipyardRow{kind: rowUtility, index: 0}
	kind, item, grade := g.shipyardSlotTarget(inst, row)
	if kind != sim.SlotUtility || grade != 0 {
		t.Fatalf("expected grade 0 on an empty slot, got item=%s grade=%d", item, grade)
	}
	if sim.SlotItemLocked(item) {
		t.Fatalf("expected the default pick to be unlocked, got %s", item)
	}

	inst.Utility[0] = &sim.SlotDevice{ItemID: item, Grade: sim.MaxGrade}
	_, item2, grade2 := g.shipyardSlotTarget(inst, row)
	if item2 == item {
		t.Fatalf("expected a maxed device to cycle to a different item, still got %s", item2)
	}
	if grade2 != 0 {
		t.Fatalf("expected the cycled-to item to start at grade 0, got %d", grade2)
	}
}

func TestShipyardInternalRowHasNoWeaponOnMiner(t *testing.T) {
	c := shipyardTestContent(t)
	model := c.ShipByID("skiff")
	rows := shipyardRows(model)
	for _, r := range rows {
		if r.kind == rowWeapon {
			t.Fatal("the starter skiff (Miner class) must not have any weapon rows")
		}
	}
}
