package tui

import (
	"strings"
	"testing"

	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
)

// TestShipyardEnterOnSlotOpensPickerNotDirectInstall covers the bug report:
// Enter on a slot row must open the item picker overlay, not silently
// install the first catalog item (Extra Cargo) the way the old
// cycle-on-Enter shortcut did.
func TestShipyardEnterOnSlotOpensPickerNotDirectInstall(t *testing.T) {
	c := shipyardTestContent(t)
	st := sim.New(c, 1, 1000)
	g := newShipyardGame(t, st)
	g.shipyardPane = shipyardPaneLoadout
	rows := shipyardRows(c.ShipByID("skiff"))
	utilityRow := -1
	for i, r := range rows {
		if r.kind == rowUtility {
			utilityRow = i
			break
		}
	}
	if utilityRow < 0 {
		t.Fatal("expected the skiff to have a utility row")
	}
	g.shipyardRowSel = utilityRow

	g.keyShipyard("enter")

	if g.overlay != ovSlotPicker {
		t.Fatalf("expected Enter on an empty slot to open ovSlotPicker, got overlay %v", g.overlay)
	}
	if g.snap.State.Ships["skiff"].Utility[0] != nil {
		t.Fatal("Enter must not install anything directly — only opening the picker should happen")
	}
}

// TestShipyardXOnOccupiedSlotOpensRemoveConfirmNotDirectRemove covers the
// bug report: X must open a store-vs-sell confirm, not silently discard the
// device with no refund.
func TestShipyardXOnOccupiedSlotOpensRemoveConfirmNotDirectRemove(t *testing.T) {
	c := shipyardTestContent(t)
	st := sim.New(c, 1, 1000)
	if err := sim.InstallSlotDevice(st, c, "skiff", sim.SlotUtility, 0, sim.ItemShield, 0); err != nil {
		t.Fatal(err)
	}
	g := newShipyardGame(t, st)
	g.shipyardPane = shipyardPaneLoadout
	rows := shipyardRows(c.ShipByID("skiff"))
	utilityRow := -1
	for i, r := range rows {
		if r.kind == rowUtility {
			utilityRow = i
			break
		}
	}
	g.shipyardRowSel = utilityRow

	g.keyShipyard("x")

	if g.overlay != ovSlotRemove {
		t.Fatalf("expected X on an occupied slot to open ovSlotRemove, got overlay %v", g.overlay)
	}
	if g.snap.State.Ships["skiff"].Utility[0] == nil {
		t.Fatal("X must not remove anything directly — only opening the confirm should happen")
	}
}

// TestShipyardXOnEmptySlotDoesNothing guards against opening a remove
// confirm for a slot with nothing installed.
func TestShipyardXOnEmptySlotDoesNothing(t *testing.T) {
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

	g.keyShipyard("x")

	if g.overlay != ovNone {
		t.Fatalf("expected X on an empty slot to do nothing, got overlay %v", g.overlay)
	}
}

// TestPickerBuildEntriesListsCatalogAndMatchingInventoryOnly covers that
// pickerBuildEntries lists every catalog item for the slot kind plus only
// the inventory devices matching that kind — not devices from other kinds.
func TestPickerBuildEntriesListsCatalogAndMatchingInventoryOnly(t *testing.T) {
	c := shipyardTestContent(t)
	st := sim.New(c, 1, 1000)
	st.Credits = 100000
	if err := sim.InstallSlotDevice(st, c, "skiff", sim.SlotUtility, 0, sim.ItemChaff, 1); err != nil {
		t.Fatal(err)
	}
	if err := sim.StoreSlotDevice(st, c, "skiff", sim.SlotUtility, 0); err != nil {
		t.Fatal(err)
	}
	if err := sim.InstallSlotDevice(st, c, "skiff", sim.SlotInternal, 0, sim.ItemSeismic, 0); err != nil {
		t.Fatal(err)
	}
	if err := sim.StoreSlotDevice(st, c, "skiff", sim.SlotInternal, 0); err != nil {
		t.Fatal(err)
	}
	g := newShipyardGame(t, st)
	g.pickerCatalogGrade = make([]int, len(sim.SlotItemsFor(sim.SlotUtility)))

	entries := g.pickerBuildEntries(st, "skiff", sim.SlotUtility, nil)

	items := sim.SlotItemsFor(sim.SlotUtility)
	if len(entries) != len(items)+1 {
		t.Fatalf("expected %d catalog rows + 1 matching inventory row, got %d entries", len(items), len(entries))
	}
	var invRows int
	for _, e := range entries {
		if e.fromInv {
			invRows++
			if e.itemID != sim.ItemChaff {
				t.Fatalf("expected only the stored chaff launcher to appear, got %s", e.itemID)
			}
		}
	}
	if invRows != 1 {
		t.Fatalf("expected exactly 1 inventory row (the stored internal seismic sensor must not leak into a utility picker), got %d", invRows)
	}
}

// TestPickerRendersStorageSection makes stored modules discoverable and
// selectable from a distinct in-storage section of the item picker.
func TestPickerRendersStorageSection(t *testing.T) {
	c := shipyardTestContent(t)
	st := sim.New(c, 1, 1000)
	st.Inventory = []*sim.SlotDevice{{ItemID: sim.ItemChaff, Grade: 1}}
	g := newShipyardGame(t, st)
	g.shipyardPane = shipyardPaneLoadout
	rows := shipyardRows(c.ShipByID("skiff"))
	for i, r := range rows {
		if r.kind == rowUtility {
			g.shipyardRowSel = i
			break
		}
	}
	g.openSlotPicker(st.Ships["skiff"], rows[g.shipyardRowSel])

	out := g.renderSlotPickerOverlay()
	if !strings.Contains(out, "IN STORAGE") || !strings.Contains(out, "CHAFF LAUNCHER") {
		t.Fatalf("expected a visible storage section with the stored module, got:\n%s", out)
	}
}

// TestPickerReplacementUsesRemoveConfirmation ensures selecting a different
// module never overwrites the installed one directly.
func TestPickerReplacementUsesRemoveConfirmation(t *testing.T) {
	c := shipyardTestContent(t)
	st := sim.New(c, 1, 1000)
	st.Credits = 100000
	if err := sim.InstallSlotDevice(st, c, "skiff", sim.SlotUtility, 0, sim.ItemShield, 0); err != nil {
		t.Fatal(err)
	}
	g := newShipyardGame(t, st)
	g.shipyardPane = shipyardPaneLoadout
	rows := shipyardRows(c.ShipByID("skiff"))
	for i, r := range rows {
		if r.kind == rowUtility {
			g.shipyardRowSel = i
			break
		}
	}
	g.openSlotPicker(st.Ships["skiff"], rows[g.shipyardRowSel])
	model, row, entries, ok := g.pickerContext()
	if !ok {
		t.Fatal("expected a valid picker context")
	}
	for i, e := range entries {
		if e.itemID == sim.ItemCargo {
			g.pickerSel = i
			break
		}
	}
	g.pickerConfirm(model, row, entries)

	if g.overlay != ovSlotRemove || g.pendingSlotInstall == nil {
		t.Fatalf("expected replacement to open the store/sell dialog, got overlay=%v pending=%+v", g.overlay, g.pendingSlotInstall)
	}
	if d := g.snap.State.Ships["skiff"].Utility[0]; d == nil || d.ItemID != sim.ItemShield {
		t.Fatalf("opening removal confirmation changed installed module: %+v", d)
	}
}

// TestPickerLeftRightAdjustsCatalogGradeOnly covers that arrow keys only
// move the grade cursor on catalog rows, never on the inventory section.
func TestPickerLeftRightAdjustsCatalogGradeOnly(t *testing.T) {
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
	g.openSlotPicker(st.Ships["skiff"], rows[g.shipyardRowSel])
	if g.overlay != ovSlotPicker {
		t.Fatal("expected openSlotPicker to set ovSlotPicker")
	}
	g.pickerSel = 0
	before := g.pickerCatalogGrade[0]

	g.updateSlotPickerOverlay("right")
	if g.pickerCatalogGrade[0] != before+1 {
		t.Fatalf("expected right to raise the grade cursor by 1: before=%d after=%d", before, g.pickerCatalogGrade[0])
	}
	g.updateSlotPickerOverlay("left")
	if g.pickerCatalogGrade[0] != before {
		t.Fatalf("expected left to undo the raise: got %d, want %d", g.pickerCatalogGrade[0], before)
	}
}

// TestPickerEscClosesWithoutInstalling covers Esc canceling the picker.
func TestPickerEscClosesWithoutInstalling(t *testing.T) {
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
	g.openSlotPicker(st.Ships["skiff"], rows[g.shipyardRowSel])

	g.updateSlotPickerOverlay("esc")

	if g.overlay != ovNone {
		t.Fatalf("expected esc to close the picker, got overlay %v", g.overlay)
	}
	if g.snap.State.Ships["skiff"].Utility[0] != nil {
		t.Fatal("esc must not install anything")
	}
}

// TestGradeDotsRendersExactlyMaxWidth covers the pip-rendering bug: dots
// must render exactly `max` circles, not always a fixed 5, so a track
// capped below S doesn't show trailing circles it can never fill.
func TestGradeDotsRendersExactlyMaxWidth(t *testing.T) {
	got := gradeDots(2, 3)
	want := "●●○"
	// Style codes wrap each glyph, so just count rune occurrences instead of
	// an exact string match.
	filled := 0
	empty := 0
	for _, r := range got {
		switch r {
		case '●':
			filled++
		case '○':
			empty++
		}
	}
	if filled != 2 || empty != 1 {
		t.Fatalf("gradeDots(2, 3) has %d filled + %d empty circles, want 2 filled + 1 empty (like %q)", filled, empty, want)
	}
}

func TestRenderShipyardTrackPipsMatchTrackCapNotFixedFive(t *testing.T) {
	c := shipyardTestContent(t)
	st := sim.New(c, 1, 1000)
	g := newShipyardGame(t, st)
	model := c.ShipByID("skiff")
	cap := sim.TrackCap(model, sim.TrackThrusters)
	if cap >= sim.MaxGrade {
		t.Skip("skiff's thrusters cap must be below MaxGrade for this test to be meaningful")
	}

	out := g.renderShipyard()
	var thrustersLine string
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "THRUSTERS") {
			thrustersLine = line
			break
		}
	}
	if thrustersLine == "" {
		t.Fatal("expected a THRUSTERS row in the shipyard render")
	}
	circles := 0
	for _, r := range thrustersLine {
		if r == '●' || r == '○' {
			circles++
		}
	}
	if circles != cap {
		t.Fatalf("THRUSTERS row rendered %d pips, want exactly its cap (%d) — a fixed-5 rendering would show %d", circles, cap, sim.MaxGrade)
	}
}
