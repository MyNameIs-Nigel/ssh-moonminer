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
	hangarW, loadoutW, statusW := shipyardPaneWidths(g.contentWidth())
	panelH := g.bodyHeight()
	panelBodyH := panelH - 2

	model := g.shipyardSelectedModel()
	acquireFootnote := ""
	if model != nil && st.Ships[model.ID] == nil &&
		statusW-2 < lipgloss.Width("Purchase does not switch ships.") {
		panelH--
		panelBodyH--
		acquireFootnote = theme.DimStyle.Render("Purchase does not switch ships.")
	}

	hangarBody, hangarAccent := g.renderShipyardHangar(&st, hangarW-2, panelBodyH)
	loadoutBody, statusBody := g.renderShipyardLoadout(&st, model, hangarW+1, loadoutW-2, statusW-2, panelBodyH)

	body := lipgloss.JoinHorizontal(lipgloss.Top,
		theme.Panel("HANGAR", hangarW, panelH, hangarBody, hangarAccent),
		theme.Panel(fmt.Sprintf("%s — %s · %s", model.Name, strings.ToUpper(model.Brand), strings.ToUpper(model.Class)), loadoutW, panelH, loadoutBody, theme.Accent(theme.HueViolet)),
		theme.Panel("STATUS", statusW, panelH, statusBody, theme.Accent(theme.HueViolet)),
	)
	if acquireFootnote != "" {
		body = lipgloss.JoinVertical(lipgloss.Left, body, acquireFootnote)
	}
	hint := "↑/↓ SELECT · TAB PANE · ENTER BUY/PICK · X REMOVE · ESC/Q CHART"
	return body + "\n" + g.renderKeybar(hint)
}

func shipyardViewportLines(lines []string, sel, bodyH int) []string {
	if bodyH < 1 {
		return nil
	}
	if len(lines) <= bodyH {
		return lines
	}
	scroll := shipyardViewportScroll(len(lines), sel, bodyH)
	return lines[scroll : scroll+bodyH]
}

func shipyardReflowPlain(lines []string, innerW int) []string {
	if innerW < 1 {
		return lines
	}
	var out []string
	for _, line := range lines {
		if line == "" {
			out = append(out, "")
			continue
		}
		for _, wrapped := range wrapChartText(line, innerW) {
			out = append(out, wrapped)
		}
	}
	return out
}

func shipyardViewportScroll(total, sel, bodyH int) int {
	return centeredScroll(sel, total, bodyH)
}

func (g *Game) renderShipyardHangar(st *sim.State, innerW, bodyH int) (string, string) {
	type hangarHit struct {
		row  int
		line string
		id   int
	}
	lines := make([]string, 0, len(g.content.Ships)+4)
	hits := make([]hangarHit, 0, len(g.content.Ships))
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
			suffix = theme.DimStyle.Render(fmt.Sprintf("%s %d", theme.Glyph("credit", st.Settings.ASCIISafe), model.Price))
			if sel {
				style = theme.Bright
			}
		}
		line := style.Render(marker + model.Name)
		if suffix != "" {
			line += " " + suffix
		}
		lines = append(lines, line)
		hits = append(hits, hangarHit{len(lines) - 1, line, i})
	}
	model := g.shipyardSelectedModel()
	acquireBtn := ""
	acquireRow := -1
	if model != nil && !sim.OwnsShip(st, model.ID) {
		price := sim.ShipAcquirePrice(st, g.content, model.ID)
		label := "BUY"
		if sim.IsBuyback(st, model.ID) {
			label = "BUY BACK"
		}
		acquireBtn = theme.Button("ENTER", label, fmt.Sprintf("%s %d", theme.Glyph("credit", st.Settings.ASCIISafe), price), st.Credits >= price, theme.HueViolet)
		lines = append(lines, "", theme.Violet.Render("◇ ACQUIRE"), acquireBtn)
		acquireRow = len(lines) - 1
	}
	scroll := shipyardViewportScroll(len(lines), g.shipyardHangarSel, bodyH)
	visible := shipyardViewportLines(lines, g.shipyardHangarSel, bodyH)
	for _, h := range hits {
		lineIdx := h.row - scroll
		if lineIdx >= 0 && lineIdx < len(visible) {
			g.hitPanelLine(lineIdx, 1, h.line, fmt.Sprintf("hangar:%d", h.id), h.id)
		}
	}
	if acquireRow >= 0 {
		if lineIdx := acquireRow - scroll; lineIdx >= 0 && lineIdx < len(visible) {
			g.hitPanelLine(lineIdx, 1, acquireBtn, "btn:acquire", model.ID)
		}
	}
	accent := theme.Accent(theme.HueViolet)
	return strings.Join(visible, "\n"), accent
}

// renderShipyardLoadout returns the LOADOUT and STATUS panel bodies for the
// currently hangar-selected ship model. loadoutX is the LOADOUT panel's
// absolute screen column (it sits to the right of HANGAR), needed so its
// row hitboxes don't collide with HANGAR's at the same line index.
func (g *Game) renderShipyardLoadout(st *sim.State, model *content.ShipModel, loadoutX, loadoutInnerW, statusInnerW, bodyH int) (string, string) {
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
		statusPlain := shipyardReflowPlain([]string{
			"ACQUIRE",
			"",
			fmt.Sprintf("%s — %s %d", label, theme.Glyph("credit", st.Settings.ASCIISafe), price),
			"New hull starts fully serviced.",
		}, statusInnerW)
		if statusInnerW >= lipgloss.Width("Purchase does not switch ships.") {
			statusPlain = append(statusPlain, "Purchase does not switch ships.")
		}
		statusPlain = append(statusPlain, "", fmt.Sprintf("SLOTS U%d W%d I1 J1", model.UtilitySlots, model.WeaponSlots))
		statusLines := make([]string, len(statusPlain))
		for i, line := range statusPlain {
			switch {
			case i == 0:
				statusLines[i] = theme.Violet.Render("◇ " + line)
			case line == "":
				statusLines[i] = ""
			case strings.HasPrefix(line, "SLOTS"):
				statusLines[i] = line
			default:
				statusLines[i] = theme.DimStyle.Render(line)
			}
		}
		return loadout, strings.Join(statusLines, "\n")
	}

	type loadoutHit struct {
		row  int
		line string
		id   int
	}
	rows := shipyardRows(model)
	lines := make([]string, 0, len(rows)+4)
	hits := make([]loadoutHit, 0, len(rows))
	for i, t := range shipyardTracks {
		grade := sim.TrackGrade(st, model.ID, t)
		cap := sim.TrackCap(model, t)
		sel := g.shipyardPane == shipyardPaneLoadout && g.shipyardRowSel == i
		priceStr := "CAPPED"
		if grade < cap {
			priceStr = fmt.Sprintf("%s %d", theme.Glyph("credit", st.Settings.ASCIISafe), sim.TrackPrice(g.content, model, t, grade))
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
		hits = append(hits, loadoutHit{len(lines) - 1, line, i})
	}
	lines = append(lines, "")
	rowIdx := len(shipyardTracks)
	if model.UtilitySlots > 0 {
		lines = append(lines, theme.DimStyle.Render("UTILITY"))
		for i := 0; i < model.UtilitySlots; i++ {
			line := g.renderSlotRow(st, inst.Utility, i, rowIdx, theme.Glyph("slot_utility", st.Settings.ASCIISafe))
			lines = append(lines, line)
			hits = append(hits, loadoutHit{len(lines) - 1, line, rowIdx})
			rowIdx++
		}
	}
	if model.WeaponSlots > 0 {
		lines = append(lines, theme.DimStyle.Render("WEAPON"))
		for i := 0; i < model.WeaponSlots; i++ {
			line := g.renderSlotRow(st, inst.Weapon, i, rowIdx, theme.Glyph("slot_weapon", st.Settings.ASCIISafe))
			lines = append(lines, line)
			hits = append(hits, loadoutHit{len(lines) - 1, line, rowIdx})
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
		line := g.renderSlotRow(st, single, 0, rowIdx, theme.Glyph("slot_internal", st.Settings.ASCIISafe))
		lines = append(lines, line)
		hits = append(hits, loadoutHit{len(lines) - 1, line, rowIdx})
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
		line := g.renderSlotRow(st, single, 0, rowIdx, theme.Glyph("slot_jumpdrive", st.Settings.ASCIISafe))
		lines = append(lines, line)
		hits = append(hits, loadoutHit{len(lines) - 1, line, rowIdx})
	}
	sel := g.shipyardRowSel
	if g.shipyardPane != shipyardPaneLoadout {
		sel = 0
	}
	scroll := shipyardViewportScroll(len(lines), sel, bodyH)
	visible := shipyardViewportLines(lines, sel, bodyH)
	for _, h := range hits {
		lineIdx := h.row - scroll
		if lineIdx >= 0 && lineIdx < len(visible) {
			g.hitPanelLine(lineIdx, loadoutX, h.line, fmt.Sprintf("loadout:%d", h.id), h.id)
		}
	}
	loadoutBody := strings.Join(visible, "\n")

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
	maxHull := sim.MaxHullFor(st, g.content, model.ID)
	hullPct := 0.0
	if maxHull > 0 {
		hullPct = float64(sim.ShipHull(st, model.ID)) / float64(maxHull) * 100
	}
	fuelCapacity := sim.ShipFuelCapacity(st, g.content, model.ID)
	fuelPct := 0.0
	if fuelCapacity > 0 {
		fuelPct = sim.ShipFuelAmount(st, g.content, model.ID) / fuelCapacity * 100
	}
	statusBarW := min(18, statusInnerW)
	statusPlain := shipyardReflowPlain([]string{
		fmt.Sprintf("HULL %d/%d", sim.ShipHull(st, model.ID), sim.MaxHullFor(st, g.content, model.ID)),
		fmt.Sprintf("%s %.0f/%.0f", theme.Glyph("fuel", st.Settings.ASCIISafe), sim.ShipFuelAmount(st, g.content, model.ID), sim.ShipFuelCapacity(st, g.content, model.ID)),
		"",
		fmt.Sprintf("PWR %d/%d", power, capacity),
		"",
		fmt.Sprintf("MASS %.0f", mass),
		fmt.Sprintf("×%.2f fx", massRatio),
		"",
		fmt.Sprintf("%s %d", theme.Glyph("credit", st.Settings.ASCIISafe), st.Credits),
	}, statusInnerW)
	if model.ID != st.ActiveShipID {
		statusPlain = append(statusPlain, "", "Not active — select in HANGAR to switch/fly.")
	}
	statusLines := make([]string, 0, len(statusPlain)+1)
	for _, line := range statusPlain {
		switch {
		case line == "":
			statusLines = append(statusLines, "")
		case strings.HasPrefix(line, "HULL "):
			statusLines = append(statusLines, theme.HullStyle(hullPct).Render(line))
		case strings.HasPrefix(line, theme.Glyph("fuel", st.Settings.ASCIISafe)):
			statusLines = append(statusLines, theme.FuelStyle(fuelPct).Render(line))
		case strings.HasPrefix(line, "PWR "):
			statusLines = append(statusLines, line, theme.RampBar(powerPct, statusBarW, true))
		case strings.HasPrefix(line, theme.Glyph("credit", st.Settings.ASCIISafe)):
			statusLines = append(statusLines, theme.Gold.Render(line))
		case strings.Contains(line, "Not active"):
			statusLines = append(statusLines, theme.DimStyle.Render(line))
		default:
			statusLines = append(statusLines, line)
		}
	}
	var descLines []string
	if sel := g.shipyardRowSel; g.shipyardPane == shipyardPaneLoadout && sel >= 0 && sel < len(rows) {
		if desc := shipyardRowDesc(inst, rows[sel]); desc != "" {
			for _, w := range wrapChartText(desc, statusInnerW) {
				descLines = append(descLines, theme.TxtStyle.Render(w))
			}
			descLines = append(descLines, "")
		}
	}
	statusBody := strings.Join(append(descLines, statusLines...), "\n")
	return loadoutBody, statusBody
}

// shipyardRowDesc names what a LOADOUT row does — a track's stat effect or a
// slot's installed item — shown in STATUS when that row is selected (the
// Shipyard menu otherwise names upgrades without saying what they improve).
func shipyardRowDesc(inst *sim.ShipInstance, row shipyardRow) string {
	if row.kind == rowTrack {
		return sim.TrackDesc(row.track)
	}
	if d := slotDeviceAt(inst, row); d != nil {
		return sim.SlotItemDesc(d.ItemID)
	}
	return "Empty slot — Enter to install."
}

func (g *Game) renderSlotRow(st *sim.State, devices []*sim.SlotDevice, index, rowIdx int, tag string) string {
	sel := g.shipyardPane == shipyardPaneLoadout && g.shipyardRowSel == rowIdx
	style := theme.OptionHC(theme.HueViolet, sel, st.Settings.HighContrast)
	prefix := "  "
	if sel {
		prefix = "▸ "
	}
	var d *sim.SlotDevice
	if index < len(devices) {
		d = devices[index]
	}
	if d == nil {
		return style.Render(prefix+tag+"  ") + theme.DimStyle.Render("empty — enter to install")
	}
	name := sim.SlotItemName(d.ItemID)
	powerGlyph := ""
	if p := sim.SlotItemPower(g.content, d.ItemID, d.Grade); p > 0 {
		powerGlyph = theme.Amber.Render(fmt.Sprintf(" ⚡%d", p))
	}
	line := fmt.Sprintf("%s%s %-14s %s %s%s", prefix, tag, name, gradeDots(d.Grade, sim.MaxGrade), sim.GradeLetter(d.Grade), powerGlyph)
	return style.Render(line)
}
