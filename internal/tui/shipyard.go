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

// The Shipyard is a standalone screen (not a Star Chart overlay) — see
// docs/tui/05-shipyard-screen.md. Three panels: HANGAR (owned/buyable ship
// models), LOADOUT (the selected ship's track grades + slots), and STATUS
// (power/mass meters, credits, buy/buyback prompt).

const (
	shipyardPaneHangar = iota
	shipyardPaneLoadout
)

type shipyardRowKind int

const (
	rowTrack shipyardRowKind = iota
	rowUtility
	rowWeapon
	rowInternal
	rowJumpDrive
)

type shipyardRow struct {
	kind  shipyardRowKind
	track sim.Track
	index int // slot index, for rowUtility/rowWeapon
}

// slotKind maps a UI row kind to the sim's SlotKind — the one place that
// association is expressed, shared by every row-kind switch below.
func (row shipyardRow) slotKind() sim.SlotKind {
	switch row.kind {
	case rowUtility:
		return sim.SlotUtility
	case rowWeapon:
		return sim.SlotWeapon
	case rowJumpDrive:
		return sim.SlotJumpDrive
	default:
		return sim.SlotInternal
	}
}

var shipyardTracks = []sim.Track{
	sim.TrackThrusters, sim.TrackHull, sim.TrackFuelEff, sim.TrackPowerGen, sim.TrackScanner,
}

func shipyardRows(model *content.ShipModel) []shipyardRow {
	rows := make([]shipyardRow, 0, 5+model.UtilitySlots+model.WeaponSlots+2)
	for _, t := range shipyardTracks {
		rows = append(rows, shipyardRow{kind: rowTrack, track: t})
	}
	for i := 0; i < model.UtilitySlots; i++ {
		rows = append(rows, shipyardRow{kind: rowUtility, index: i})
	}
	for i := 0; i < model.WeaponSlots; i++ {
		rows = append(rows, shipyardRow{kind: rowWeapon, index: i})
	}
	rows = append(rows, shipyardRow{kind: rowInternal})
	rows = append(rows, shipyardRow{kind: rowJumpDrive})
	return rows
}

func (g *Game) shipyardSelectedModel() *content.ShipModel {
	if len(g.content.Ships) == 0 {
		return nil
	}
	if g.shipyardHangarSel < 0 || g.shipyardHangarSel >= len(g.content.Ships) {
		g.shipyardHangarSel = 0
	}
	return &g.content.Ships[g.shipyardHangarSel]
}

func (g *Game) keyShipyard(k string) []tea.Cmd {
	st := g.snap.State
	n := len(g.content.Ships)
	if n == 0 {
		return nil
	}
	switch k {
	case "esc", "q":
		g.scr = scrChart
		return nil
	case "tab":
		model := g.shipyardSelectedModel()
		if g.shipyardPane == shipyardPaneHangar && sim.OwnsShip(&st, model.ID) {
			g.shipyardPane = shipyardPaneLoadout
			g.shipyardRowSel = 0
		} else {
			g.shipyardPane = shipyardPaneHangar
		}
		return nil
	case "shift+tab":
		g.shipyardPane = shipyardPaneHangar
		return nil
	case "up", "k":
		if g.shipyardPane == shipyardPaneHangar {
			g.shipyardHangarSel = (g.shipyardHangarSel - 1 + n) % n
			g.shipyardRowSel = 0
		} else {
			rows := shipyardRows(g.shipyardSelectedModel())
			g.shipyardRowSel = (g.shipyardRowSel - 1 + len(rows)) % len(rows)
		}
	case "down", "j":
		if g.shipyardPane == shipyardPaneHangar {
			g.shipyardHangarSel = (g.shipyardHangarSel + 1) % n
			g.shipyardRowSel = 0
		} else {
			rows := shipyardRows(g.shipyardSelectedModel())
			g.shipyardRowSel = (g.shipyardRowSel + 1) % len(rows)
		}
	case "enter", " ":
		return g.shipyardActivate()
	case "x", "backspace", "delete":
		return g.shipyardRemove()
	}
	return nil
}

// shipyardSlotTarget picks a sane starting cursor for the item picker
// overlay (internal/tui/shipyard_picker.go, openSlotPicker): the first
// unlocked catalog item at grade E on an empty slot, the next grade of the
// currently-installed item while below S (so the default selection reads
// as "the upgrade you'd get"), or the next catalog item at grade E once
// maxed.
func (g *Game) shipyardSlotTarget(inst *sim.ShipInstance, row shipyardRow) (sim.SlotKind, string, int) {
	kind := row.slotKind()
	var current *sim.SlotDevice
	switch row.kind {
	case rowUtility:
		if inst != nil && row.index < len(inst.Utility) {
			current = inst.Utility[row.index]
		}
	case rowWeapon:
		if inst != nil && row.index < len(inst.Weapon) {
			current = inst.Weapon[row.index]
		}
	case rowInternal:
		if inst != nil {
			current = inst.Internal
		}
	case rowJumpDrive:
		if inst != nil {
			current = inst.JumpDrive
		}
	}
	items := sim.SlotItemsFor(kind)
	if len(items) == 0 {
		return kind, "", 0
	}
	if current == nil {
		for _, id := range items {
			if !sim.SlotItemLocked(id) {
				return kind, id, 0
			}
		}
		return kind, items[0], 0
	}
	if current.Grade < sim.MaxGrade {
		return kind, current.ItemID, current.Grade + 1
	}
	idx := 0
	for i, id := range items {
		if id == current.ItemID {
			idx = i
			break
		}
	}
	for i := 1; i <= len(items); i++ {
		next := items[(idx+i)%len(items)]
		if !sim.SlotItemLocked(next) {
			return kind, next, 0
		}
	}
	return kind, current.ItemID, current.Grade
}

func (g *Game) shipyardActivate() []tea.Cmd {
	st := g.snap.State
	model := g.shipyardSelectedModel()
	if model == nil {
		return nil
	}
	if g.shipyardPane == shipyardPaneHangar {
		if !sim.OwnsShip(&st, model.ID) {
			snap, err := g.sess.AcquireShip(g.now, model.ID)
			if err == nil {
				g.setFlash("BOUGHT FULLY SERVICED — ENTER TO ACTIVATE")
			}
			return g.refreshSnap(snap, err)
		}
		if model.ID != st.ActiveShipID {
			snap, err := g.sess.SwitchActiveShip(g.now, model.ID)
			return g.refreshSnap(snap, err)
		}
		g.shipyardPane = shipyardPaneLoadout
		g.shipyardRowSel = 0
		return nil
	}
	if !sim.OwnsShip(&st, model.ID) {
		return nil
	}
	rows := shipyardRows(model)
	if g.shipyardRowSel < 0 || g.shipyardRowSel >= len(rows) {
		return nil
	}
	row := rows[g.shipyardRowSel]
	if row.kind == rowTrack {
		snap, err := g.sess.BuyShipTrack(g.now, model.ID, row.track)
		return g.refreshSnap(snap, err)
	}
	g.openSlotPicker(st.Ships[model.ID], row)
	return nil
}

func (g *Game) shipyardRemove() []tea.Cmd {
	st := g.snap.State
	if g.shipyardPane != shipyardPaneLoadout {
		return nil
	}
	model := g.shipyardSelectedModel()
	if model == nil || !sim.OwnsShip(&st, model.ID) {
		return nil
	}
	rows := shipyardRows(model)
	if g.shipyardRowSel < 0 || g.shipyardRowSel >= len(rows) {
		return nil
	}
	row := rows[g.shipyardRowSel]
	if row.kind == rowTrack {
		return nil // track rows aren't removable
	}
	if slotDeviceAt(st.Ships[model.ID], row) == nil {
		return nil // nothing installed to remove
	}
	g.removeConfirmSel = 0
	g.overlay = ovSlotRemove
	return nil
}

// slotDeviceAt returns the device currently installed at row on inst, or
// nil for an empty slot / a track row.
func slotDeviceAt(inst *sim.ShipInstance, row shipyardRow) *sim.SlotDevice {
	if inst == nil {
		return nil
	}
	switch row.kind {
	case rowUtility:
		if row.index < len(inst.Utility) {
			return inst.Utility[row.index]
		}
	case rowWeapon:
		if row.index < len(inst.Weapon) {
			return inst.Weapon[row.index]
		}
	case rowInternal:
		return inst.Internal
	case rowJumpDrive:
		return inst.JumpDrive
	}
	return nil
}

// gradeDots renders a grade (0..max, E..S) as the existing violet
// filled/empty circle indicator, matching the pips already shipped in
// internal/tui/chart.go before this screen split out on its own. max is the
// row's own cap (a track's model-specific TrackCap, or sim.MaxGrade for
// slot devices, which have no per-item cap) — dots always render at exactly
// that width rather than a fixed 5, so a ship whose track caps below S
// doesn't show trailing pips it can never fill.
func gradeDots(grade, max int) string {
	if max < 0 {
		max = 0
	}
	g := clampInt(grade, 0, max)
	filled := strings.Repeat(theme.Violet.Render("●"), g)
	empty := strings.Repeat(theme.DimStyle.Render("○"), max-g)
	return filled + empty
}

func (g *Game) renderShipyard() string {
	st := g.snap.State
	hangarW := 20
	statusW := 22
	loadoutW := g.contentWidth() - hangarW - statusW
	if loadoutW < 30 {
		loadoutW = 30
	}
	panelH := g.height - 5

	hangarBody, hangarAccent := g.renderShipyardHangar(&st)
	model := g.shipyardSelectedModel()
	loadoutBody, statusBody := g.renderShipyardLoadout(&st, model, hangarW+1)

	body := lipgloss.JoinHorizontal(lipgloss.Top,
		theme.Panel("HANGAR", hangarW, panelH, hangarBody, hangarAccent),
		theme.Panel(fmt.Sprintf("%s — %s · %s", model.Name, strings.ToUpper(model.Brand), strings.ToUpper(model.Class)), loadoutW, panelH, loadoutBody, theme.Accent(theme.HueViolet)),
		theme.Panel("STATUS", statusW, panelH, statusBody, theme.Accent(theme.HueViolet)),
	)
	hint := "↑/↓ SELECT · TAB PANE · ENTER BUY/PICK · X REMOVE · ESC/Q CHART"
	return body + "\n" + g.renderKeybar(hint)
}

func (g *Game) renderShipyardHangar(st *sim.State) (string, string) {
	lines := make([]string, 0, len(g.content.Ships))
	for i, model := range g.content.Ships {
		sel := i == g.shipyardHangarSel
		marker := "  "
		if sel {
			marker = "▸ "
		}
		var suffix string
		style := theme.DimStyle
		switch {
		case model.ID == st.ActiveShipID:
			suffix = theme.Green.Render("ACTIVE")
			style = theme.OptionHC(theme.HueViolet, sel && g.shipyardPane == shipyardPaneHangar, st.Settings.HighContrast)
		case sim.OwnsShip(st, model.ID):
			style = theme.OptionHC(theme.HueViolet, sel && g.shipyardPane == shipyardPaneHangar, st.Settings.HighContrast)
		case sim.IsBuyback(st, model.ID):
			suffix = theme.Amber.Render(fmt.Sprintf("LOST·%d", sim.ShipAcquirePrice(st, g.content, model.ID)))
			if sel {
				style = theme.Bright
			} else {
				style = theme.Amber
			}
		default:
			suffix = theme.DimStyle.Render(fmt.Sprintf("%d cr", model.Price))
			if sel {
				style = theme.Bright
			}
		}
		line := style.Render(marker + model.Name)
		if suffix != "" {
			line += " " + suffix
		}
		lines = append(lines, line)
		g.hitPanelLine(len(lines)-1, 1, line, fmt.Sprintf("hangar:%d", i), i)
	}
	accent := theme.Accent(theme.HueViolet)
	return strings.Join(lines, "\n"), accent
}

// renderShipyardLoadout returns the LOADOUT and STATUS panel bodies for the
// currently hangar-selected ship model. loadoutX is the LOADOUT panel's
// absolute screen column (it sits to the right of HANGAR), needed so its
// row hitboxes don't collide with HANGAR's at the same line index.
func (g *Game) renderShipyardLoadout(st *sim.State, model *content.ShipModel, loadoutX int) (string, string) {
	inst := st.Ships[model.ID]
	if inst == nil {
		loadout := strings.Join([]string{
			"",
			theme.DimStyle.Render("NOT OWNED"),
			theme.DimStyle.Render("Select in HANGAR and press Enter to acquire."),
		}, "\n")
		price := sim.ShipAcquirePrice(st, g.content, model.ID)
		label := "BUY"
		if sim.IsBuyback(st, model.ID) {
			label = "BUY BACK"
		}
		status := strings.Join([]string{
			theme.Violet.Render("◇ ACQUIRE"),
			"",
			theme.Button("ENTER", label, fmt.Sprintf("%d cr", price), st.Credits >= price, theme.HueViolet),
			theme.DimStyle.Render("New hull starts fully serviced."),
			theme.DimStyle.Render("Purchase does not switch ships."),
			"",
			fmt.Sprintf("SLOTS  U%d  W%d  I1  J1", model.UtilitySlots, model.WeaponSlots),
		}, "\n")
		return loadout, status
	}

	rows := shipyardRows(model)
	lines := make([]string, 0, len(rows)+4)
	for i, t := range shipyardTracks {
		grade := sim.TrackGrade(st, model.ID, t)
		cap := sim.TrackCap(model, t)
		sel := g.shipyardPane == shipyardPaneLoadout && g.shipyardRowSel == i
		priceStr := "CAPPED"
		if grade < cap {
			priceStr = fmt.Sprintf("%d cr", sim.TrackPrice(g.content, model, t, grade))
		}
		prefix := "  "
		if sel {
			prefix = "▸ "
		}
		name := fmt.Sprintf("%-16s", sim.TrackName(t))
		line := theme.OptionHC(theme.HueViolet, sel, st.Settings.HighContrast).Render(prefix+name) +
			" " + gradeDots(grade, cap) + " " + sim.GradeLetter(grade) +
			" " + theme.Gold.Render(priceStr)
		lines = append(lines, line)
		g.hitPanelLine(len(lines)-1, loadoutX, line, fmt.Sprintf("loadout:%d", i), i)
	}
	lines = append(lines, "")
	rowIdx := len(shipyardTracks)
	if model.UtilitySlots > 0 {
		lines = append(lines, theme.DimStyle.Render("UTILITY"))
		for i := 0; i < model.UtilitySlots; i++ {
			lines = append(lines, g.renderSlotRow(st, inst.Utility, i, rowIdx, "U"))
			g.hitPanelLine(len(lines)-1, loadoutX, lines[len(lines)-1], fmt.Sprintf("loadout:%d", rowIdx), rowIdx)
			rowIdx++
		}
	}
	if model.WeaponSlots > 0 {
		lines = append(lines, theme.DimStyle.Render("WEAPON"))
		for i := 0; i < model.WeaponSlots; i++ {
			lines = append(lines, g.renderSlotRow(st, inst.Weapon, i, rowIdx, "W"))
			g.hitPanelLine(len(lines)-1, loadoutX, lines[len(lines)-1], fmt.Sprintf("loadout:%d", rowIdx), rowIdx)
			rowIdx++
		}
	}
	lines = append(lines, theme.DimStyle.Render("INTERNAL"))
	{
		var single []*sim.SlotDevice
		if inst.Internal != nil {
			single = []*sim.SlotDevice{inst.Internal}
		} else {
			single = []*sim.SlotDevice{nil}
		}
		lines = append(lines, g.renderSlotRow(st, single, 0, rowIdx, "I"))
		g.hitPanelLine(len(lines)-1, loadoutX, lines[len(lines)-1], fmt.Sprintf("loadout:%d", rowIdx), rowIdx)
		rowIdx++
	}
	lines = append(lines, theme.DimStyle.Render("JUMP DRIVE"))
	{
		var single []*sim.SlotDevice
		if inst.JumpDrive != nil {
			single = []*sim.SlotDevice{inst.JumpDrive}
		} else {
			single = []*sim.SlotDevice{nil}
		}
		lines = append(lines, g.renderSlotRow(st, single, 0, rowIdx, "J"))
		g.hitPanelLine(len(lines)-1, loadoutX, lines[len(lines)-1], fmt.Sprintf("loadout:%d", rowIdx), rowIdx)
	}
	loadoutBody := strings.Join(lines, "\n")

	power := sim.InstalledPower(st, g.content, model.ID)
	capacity := sim.PowerCapacityFor(st, g.content, model.ID)
	powerPct := 0.0
	if capacity > 0 {
		powerPct = float64(power) / float64(capacity) * 100
	}
	mass := sim.InstalledMass(st, g.content, model.ID)
	massRatio := 1.0
	if model.BaseMass > 0 {
		massRatio = mass / model.BaseMass
	}
	const statusBarW = 18
	statusLines := []string{
		fmt.Sprintf("HULL %d/%d", sim.ShipHull(st, model.ID), sim.MaxHullFor(st, g.content, model.ID)),
		fmt.Sprintf("FUEL %.0f/%.0f", sim.ShipFuelAmount(st, g.content, model.ID), sim.ShipFuelCapacity(st, g.content, model.ID)),
		"",
		fmt.Sprintf("PWR %d/%d", power, capacity),
		theme.RampBar(powerPct, statusBarW, true),
		"",
		fmt.Sprintf("MASS %.0f", mass),
		fmt.Sprintf("×%.2f fx", massRatio),
		"",
		theme.Gold.Render(fmt.Sprintf("%s %d cr", theme.Glyph("credit", st.Settings.ASCIISafe), st.Credits)),
	}
	if model.ID != st.ActiveShipID {
		statusLines = append(statusLines, "", theme.DimStyle.Render("Not active — select in"), theme.DimStyle.Render("HANGAR to switch/fly."))
	}
	statusBody := strings.Join(statusLines, "\n")
	return loadoutBody, statusBody
}

func (g *Game) renderSlotRow(st *sim.State, devices []*sim.SlotDevice, index, rowIdx int, tag string) string {
	sel := g.shipyardPane == shipyardPaneLoadout && g.shipyardRowSel == rowIdx
	prefix := "  "
	if sel {
		prefix = "▸ "
	}
	var d *sim.SlotDevice
	if index < len(devices) {
		d = devices[index]
	}
	if d == nil {
		label := fmt.Sprintf("%s%s  %s", prefix, tag, theme.DimStyle.Render("empty — enter to install"))
		if sel {
			return theme.Bright.Render(prefix+tag+"  ") + theme.DimStyle.Render("empty — enter to install")
		}
		return label
	}
	name := sim.SlotItemName(d.ItemID)
	powerGlyph := ""
	if p := sim.SlotItemPower(g.content, d.ItemID, d.Grade); p > 0 {
		powerGlyph = theme.Amber.Render(fmt.Sprintf(" ⚡%d", p))
	}
	line := fmt.Sprintf("%s%s %-14s %s %s%s", prefix, tag, name, gradeDots(d.Grade, sim.MaxGrade), sim.GradeLetter(d.Grade), powerGlyph)
	if sel {
		return theme.Bright.Render(line)
	}
	return theme.TxtStyle.Render(line)
}
