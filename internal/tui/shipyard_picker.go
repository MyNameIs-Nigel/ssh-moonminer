package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/theme"
)

// Two small overlays that complete the LOADOUT panel's slot actions
// (docs/tui/05-shipyard-screen.md "Navigation"): ovSlotPicker for Enter on
// a slot row (browse every catalog item/grade plus any stored inventory
// device, instead of the old cycle-on-Enter shortcut) and ovSlotRemove for
// X on an occupied slot (store for free later reinstall, or sell for 95%
// value). Both re-derive which slot they're acting on from
// shipyardPane/shipyardHangarSel/shipyardRowSel every render/keypress
// rather than caching it, since those fields are frozen (keyShipyard isn't
// called) while any overlay is open.

const (
	pickerPanelW  = 56
	pickerVisible = 8
	pickerPanelH  = pickerVisible + 6
)

// pickerEntry is one selectable row in the slot picker: either a catalog
// item at a caller-adjustable grade (buyable) or a specific stored device
// from inventory (free to re-equip).
type pickerEntry struct {
	itemID   string
	grade    int
	locked   bool
	fromInv  bool
	invIndex int
	price    int
	afford   bool
	powerFit bool
}

func (e pickerEntry) enabled() bool {
	return !e.locked && e.powerFit && (e.fromInv || e.afford)
}

// pickerRowLabel names the LOADOUT row the picker/remove overlay is acting
// on, e.g. "UTILITY 1", "WEAPON 2", "INTERNAL".
func pickerRowLabel(row shipyardRow) string {
	switch row.kind {
	case rowUtility:
		return fmt.Sprintf("UTILITY %d", row.index+1)
	case rowWeapon:
		return fmt.Sprintf("WEAPON %d", row.index+1)
	default:
		return "INTERNAL"
	}
}

// openSlotPicker opens the item picker for row, seeding the catalog grade
// cursors and initial selection from shipyardSlotTarget's existing
// pick-something-sane logic (empty slot: first unlocked item at grade 0;
// occupied slot: that item's current grade).
func (g *Game) openSlotPicker(inst *sim.ShipInstance, row shipyardRow) {
	kind, seedItem, seedGrade := g.shipyardSlotTarget(inst, row)
	items := sim.SlotItemsFor(kind)
	if len(items) == 0 && inventoryCountForKind(g.snap.State.Inventory, kind) == 0 {
		return // nothing installable in this slot kind at all — nothing to pick
	}
	g.pickerCatalogGrade = make([]int, len(items))
	g.pickerSel = 0
	for i, id := range items {
		if id == seedItem {
			g.pickerCatalogGrade[i] = seedGrade
			g.pickerSel = i
		}
	}
	g.pickerScroll = 0
	g.pickerClampScroll(len(items) + inventoryCountForKind(g.snap.State.Inventory, kind))
	g.overlay = ovSlotPicker
}

func inventoryCountForKind(inv []*sim.SlotDevice, kind sim.SlotKind) int {
	n := 0
	for _, d := range inv {
		if d != nil && sim.SlotItemKind(d.ItemID) == kind {
			n++
		}
	}
	return n
}

// pickerBuildEntries lists every catalog item (at its current grade cursor)
// followed by every stored inventory device matching kind, each annotated
// with whether picking it right now is affordable and power-fits — the
// picker renders disabled rows instead of hiding them (tui/05 spec).
func (g *Game) pickerBuildEntries(st *sim.State, shipID string, kind sim.SlotKind, current *sim.SlotDevice) []pickerEntry {
	items := sim.SlotItemsFor(kind)
	entries := make([]pickerEntry, 0, len(items))

	capacity := sim.PowerCapacityFor(st, g.content, shipID)
	powerWithout := g.pickerPowerWithout(st, shipID, current)

	for i, id := range items {
		grade := 0
		if i < len(g.pickerCatalogGrade) {
			grade = g.pickerCatalogGrade[i]
		}
		price := sim.SlotItemPrice(g.content, id, grade)
		power := sim.SlotItemPower(g.content, id, grade)
		entries = append(entries, pickerEntry{
			itemID:   id,
			grade:    grade,
			locked:   sim.SlotItemLocked(id),
			price:    price,
			afford:   st.Credits >= price,
			powerFit: powerWithout+power <= capacity,
		})
	}
	for idx, d := range st.Inventory {
		if d == nil || sim.SlotItemKind(d.ItemID) != kind {
			continue
		}
		power := sim.SlotItemPower(g.content, d.ItemID, d.Grade)
		entries = append(entries, pickerEntry{
			itemID:   d.ItemID,
			grade:    d.Grade,
			fromInv:  true,
			invIndex: idx,
			powerFit: powerWithout+power <= capacity,
		})
	}
	return entries
}

// pickerPowerWithout returns shipID's installed power with current's draw
// backed out — the headroom available for whatever replaces it. Shared by
// pickerBuildEntries (per-entry power-fit checks) and pickerConfirm (the
// "insufficient power" flash), so both always agree on the same number.
func (g *Game) pickerPowerWithout(st *sim.State, shipID string, current *sim.SlotDevice) int {
	var curPower int
	if current != nil {
		curPower = sim.SlotItemPower(g.content, current.ItemID, current.Grade)
	}
	return sim.InstalledPower(st, g.content, shipID) - curPower
}

// pickerRow resolves just the model/row an overlay is acting on from frozen
// shipyard navigation state (shipyardHangarSel/shipyardRowSel, unchanged
// while any overlay is open), without the cost of building the full entries
// list — shared by pickerContext, removeContext, and handlers (like the
// grade-adjust left/right keys) that only need to know the slot kind.
func (g *Game) pickerRow() (model *content.ShipModel, row shipyardRow, ok bool) {
	st := g.snap.State
	model = g.shipyardSelectedModel()
	if model == nil || !sim.OwnsShip(&st, model.ID) {
		return nil, shipyardRow{}, false
	}
	rows := shipyardRows(model)
	if g.shipyardRowSel < 0 || g.shipyardRowSel >= len(rows) {
		return nil, shipyardRow{}, false
	}
	row = rows[g.shipyardRowSel]
	if row.kind == rowTrack {
		return nil, shipyardRow{}, false
	}
	return model, row, true
}

// pickerContext re-derives the model/row/entries the picker overlay is
// currently acting on. ok is false if that state is somehow no longer valid
// (e.g. the ship was lost mid-overlay some other way), in which case
// callers should close the overlay.
func (g *Game) pickerContext() (model *content.ShipModel, row shipyardRow, entries []pickerEntry, ok bool) {
	model, row, ok = g.pickerRow()
	if !ok {
		return nil, shipyardRow{}, nil, false
	}
	st := g.snap.State
	current := slotDeviceAt(st.Ships[model.ID], row)
	entries = g.pickerBuildEntries(&st, model.ID, row.slotKind(), current)
	return model, row, entries, true
}

func (g *Game) pickerClampScroll(total int) {
	visible := pickerVisible
	if total < visible {
		visible = total
	}
	if visible <= 0 {
		g.pickerScroll = 0
		return
	}
	if g.pickerSel < g.pickerScroll {
		g.pickerScroll = g.pickerSel
	}
	if g.pickerSel >= g.pickerScroll+visible {
		g.pickerScroll = g.pickerSel - visible + 1
	}
}

func (g *Game) updateSlotPickerOverlay(k string) []tea.Cmd {
	switch k {
	case "esc", "q":
		g.overlay = ovNone
		return nil
	case "left", "h", "right", "l":
		// Grade-adjust only needs the row's slot kind (to know the catalog
		// item count g.pickerSel indexes into), not the full entries build
		// pickerContext does — skip that cost on what's likely the
		// highest-frequency key in this overlay.
		_, row, ok := g.pickerRow()
		if !ok {
			g.overlay = ovNone
			return nil
		}
		items := sim.SlotItemsFor(row.slotKind())
		if g.pickerSel >= len(items) {
			return nil
		}
		delta := -1
		if k == "right" || k == "l" {
			delta = 1
		}
		g.pickerCatalogGrade[g.pickerSel] = clampInt(g.pickerCatalogGrade[g.pickerSel]+delta, 0, sim.MaxGrade)
		return nil
	}

	model, row, entries, ok := g.pickerContext()
	if !ok {
		g.overlay = ovNone
		return nil
	}
	total := len(entries)

	switch k {
	case "up", "k":
		if total > 0 {
			g.pickerSel = (g.pickerSel - 1 + total) % total
			g.pickerClampScroll(total)
		}
	case "down", "j":
		if total > 0 {
			g.pickerSel = (g.pickerSel + 1) % total
			g.pickerClampScroll(total)
		}
	case "enter", " ":
		return g.pickerConfirm(model, row, entries)
	}
	return nil
}

func (g *Game) pickerConfirm(model *content.ShipModel, row shipyardRow, entries []pickerEntry) []tea.Cmd {
	if g.pickerSel < 0 || g.pickerSel >= len(entries) {
		return nil
	}
	e := entries[g.pickerSel]
	switch {
	case e.locked:
		g.setFlash("ITEM LOCKED")
		return nil
	case !e.powerFit:
		st := g.snap.State
		current := slotDeviceAt(st.Ships[model.ID], row)
		powerWithout := g.pickerPowerWithout(&st, model.ID, current)
		wouldUse := powerWithout + sim.SlotItemPower(g.content, e.itemID, e.grade)
		capacity := sim.PowerCapacityFor(&st, g.content, model.ID)
		g.setFlash(fmt.Sprintf("INSUFFICIENT POWER — %d/%d USED", wouldUse, capacity))
		return nil
	case !e.fromInv && !e.afford:
		g.setFlash("INSUFFICIENT CREDITS")
		return nil
	}

	var snap sim.Snapshot
	var err error
	if e.fromInv {
		snap, err = g.sess.InstallSlotDeviceFromInventory(g.now, model.ID, row.slotKind(), row.index, e.invIndex)
	} else {
		snap, err = g.sess.InstallSlotDevice(g.now, model.ID, row.slotKind(), row.index, e.itemID, e.grade)
	}
	g.overlay = ovNone
	return g.refreshSnap(snap, err)
}

func (g *Game) renderSlotPickerOverlay() string {
	_, row, entries, ok := g.pickerContext()
	if !ok {
		return ""
	}

	lines := []string{
		theme.DimStyle.Render("←/→ grade · ↑/↓ item · ENTER install · ESC cancel"),
		"",
	}
	visible := pickerVisible
	if len(entries) < visible {
		visible = len(entries)
	}
	for i := g.pickerScroll; i < g.pickerScroll+visible && i < len(entries); i++ {
		e := entries[i]
		sel := i == g.pickerSel
		prefix := "  "
		if sel {
			prefix = "▸ "
		}
		name := sim.SlotItemName(e.itemID)
		if e.fromInv {
			name += " (stored)"
		}
		var right string
		switch {
		case e.locked:
			right = "LOCKED"
		case e.fromInv:
			right = "FREE"
		default:
			right = fmt.Sprintf("%d cr", e.price)
		}
		powerGlyph := ""
		if power := sim.SlotItemPower(g.content, e.itemID, e.grade); power > 0 {
			powerGlyph = fmt.Sprintf(" ⚡%d", power)
		}
		label := fmt.Sprintf("%s%-19s %s %s %8s%s", prefix, name, gradeDots(e.grade, sim.MaxGrade), sim.GradeLetter(e.grade), right, powerGlyph)

		var style lipgloss.Style
		switch {
		case e.enabled():
			style = theme.OptionHC(theme.HueViolet, sel, g.snap.State.Settings.HighContrast)
		case sel:
			style = theme.Red
		default:
			style = theme.DimStyle
		}
		line := style.Render(label)
		lines = append(lines, line)
		g.pickerHitLine(len(lines)-1, line, i)
	}
	for len(lines) < pickerVisible+2 {
		lines = append(lines, "")
	}
	scrollHint := ""
	if len(entries) > visible {
		scrollHint = fmt.Sprintf("%d/%d — wheel/↑↓ to scroll", g.pickerSel+1, len(entries))
	}
	lines = append(lines, theme.DimStyle.Render(scrollHint))
	body := strings.Join(lines, "\n")
	title := fmt.Sprintf("INSTALL — %s", pickerRowLabel(row))
	return theme.Panel(title, pickerPanelW, pickerPanelH, body, theme.Accent(theme.HueViolet))
}

func (g *Game) pickerHitLine(lineIdx int, label string, entryIdx int) {
	g.overlayHitLine(pickerPanelW, pickerPanelH, lineIdx, label, "picker", entryIdx)
}

// --- Remove confirm: store vs. sell for sim.SlotItemSellValue --------------

const (
	removeConfirmStore = iota
	removeConfirmSell
)

const (
	removePanelW = 46
	removePanelH = 9
)

func (g *Game) updateSlotRemoveOverlay(k string) []tea.Cmd {
	model, row, ok := g.removeContext()
	if !ok {
		g.overlay = ovNone
		return nil
	}
	switch k {
	case "esc", "q":
		g.overlay = ovNone
		return nil
	case "up", "down", "left", "right", "h", "j", "k", "l", "tab":
		g.removeConfirmSel = 1 - g.removeConfirmSel
	case "s":
		g.removeConfirmSel = removeConfirmStore
		return g.removeConfirm(model, row)
	case "v":
		g.removeConfirmSel = removeConfirmSell
		return g.removeConfirm(model, row)
	case "enter", " ":
		return g.removeConfirm(model, row)
	}
	return nil
}

func (g *Game) removeConfirm(model *content.ShipModel, row shipyardRow) []tea.Cmd {
	var snap sim.Snapshot
	var err error
	if g.removeConfirmSel == removeConfirmSell {
		snap, err = g.sess.SellSlotDevice(g.now, model.ID, row.slotKind(), row.index)
	} else {
		snap, err = g.sess.StoreSlotDevice(g.now, model.ID, row.slotKind(), row.index)
	}
	g.overlay = ovNone
	return g.refreshSnap(snap, err)
}

// removeContext re-derives the model/row the remove-confirm overlay is
// acting on, sharing pickerRow's preamble plus its own extra requirement:
// the row must actually have a device installed.
func (g *Game) removeContext() (model *content.ShipModel, row shipyardRow, ok bool) {
	model, row, ok = g.pickerRow()
	if !ok {
		return nil, shipyardRow{}, false
	}
	if slotDeviceAt(g.snap.State.Ships[model.ID], row) == nil {
		return nil, shipyardRow{}, false
	}
	return model, row, true
}

func (g *Game) renderSlotRemoveOverlay() string {
	st := g.snap.State
	model, row, ok := g.removeContext()
	if !ok {
		return ""
	}
	inst := st.Ships[model.ID]
	d := slotDeviceAt(inst, row)
	if d == nil {
		return ""
	}
	sellValue := sim.SlotItemSellValue(g.content, d.ItemID, d.Grade)

	name := sim.SlotItemName(d.ItemID) + " " + gradeDots(d.Grade, sim.MaxGrade) + " " + sim.GradeLetter(d.Grade)
	lines := []string{
		theme.DimStyle.Render("REMOVE — " + pickerRowLabel(row)),
		name,
		"",
	}
	storeStyle := theme.OptionHC(theme.HueViolet, g.removeConfirmSel == removeConfirmStore, st.Settings.HighContrast)
	sellStyle := theme.OptionHC(theme.HueViolet, g.removeConfirmSel == removeConfirmSell, st.Settings.HighContrast)
	storeMarker, sellMarker := "  ", "  "
	if g.removeConfirmSel == removeConfirmStore {
		storeMarker = "▸ "
	} else {
		sellMarker = "▸ "
	}
	sellPct := g.content.Slots.SellValuePct * 100
	storeLine := storeStyle.Render(storeMarker + "[S] STORE — keep it, install free later")
	sellLine := sellStyle.Render(sellMarker + fmt.Sprintf("[V] SELL — %d cr (%.0f%% value)", sellValue, sellPct))
	lines = append(lines, storeLine, sellLine, "", theme.DimStyle.Render("↑/↓ choose · Enter confirm · Esc cancel"))
	g.removeHitLine(3, storeLine, removeConfirmStore)
	g.removeHitLine(4, sellLine, removeConfirmSell)

	body := strings.Join(lines, "\n")
	return theme.Panel("REMOVE MODULE", removePanelW, removePanelH, body, theme.Accent(theme.HueViolet))
}

func (g *Game) removeHitLine(lineIdx int, label string, sel int) {
	g.overlayHitLine(removePanelW, removePanelH, lineIdx, label, "removeopt", sel)
}
