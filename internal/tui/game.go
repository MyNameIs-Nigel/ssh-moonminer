package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
	"github.com/mynameis-nigel/ssh-moonminer/internal/game"
	"github.com/mynameis-nigel/ssh-moonminer/internal/identity"
	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/hitbox"
	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/theme"
)

// Model is the Bubble Tea model type.
type Model = tea.Model

// ProgramOption is a Bubble Tea program option.
type ProgramOption = tea.ProgramOption

const minWidth, minHeight = 80, 24

type screen int

const (
	scrChart screen = iota
	scrBelt
	scrMining
	scrSummary
	scrLog
	scrShipyard
	scrDeath
	scrJump
)

type overlay int

const (
	ovNone overlay = iota
	ovHelp
	ovTweaks
	ovKicked
	ovOnboard
	ovDev
	ovSlotPicker
	ovSlotRemove
	ovPermit
	ovSignalLost
	ovJump
	ovCertify
	ovFerry
)

type (
	tickMsg      time.Time
	snapMsg      sim.Snapshot
	kickedMsg    string
	deathTickMsg time.Time
)

type flash struct {
	text    string
	expires int64
}

// Game is the root TUI model.
type Game struct {
	sess    *game.Session
	content *content.Content
	id      identity.SessionIdentity
	attach  game.AttachResult
	devMode bool

	snap       sim.Snapshot
	width      int
	height     int
	scr        screen
	overlay    overlay
	created    bool
	onboardPg  int
	helpScroll int
	tweaksSel  int
	devSel     int

	jumpFlash      bool
	jumpTarget     string
	jumpBeat       int
	jumpGeneration uint64
	jumpResult     *sim.JumpResult
	ferrySel       int
	worldSel       int
	rockSel        int
	logScroll      int

	// Shipyard screen navigation state (internal/tui/shipyard.go).
	shipyardPane      int
	shipyardHangarSel int
	shipyardRowSel    int

	// Slot item picker / remove-confirm overlay state
	// (internal/tui/shipyard_picker.go). Which slot they act on is always
	// re-derived from shipyardPane/shipyardHangarSel/shipyardRowSel above
	// (frozen while an overlay is open), not stored separately.
	pickerSel          int
	pickerScroll       int
	pickerCatalogGrade []int
	removeConfirmSel   int
	pendingSlotInstall *pickerEntry
	pendingPermitWorld int

	tickCount int
	now       int64
	idleSecs  int64
	lastInput int64

	lastClickID string
	lastClickAt int64

	lastOutcome *sim.RunOutcome
	flash       flash
	hits        *hitbox.Registry
	kickReason  string

	// signalLost is the run record the disconnect autopilot closed while the
	// pilot was away, shown once as ovSignalLost on reconnect and then
	// acknowledged (framework/05).
	signalLost *sim.RunRecord

	// Death-sequence flicker: deathFrame counts fast ticks since the ship
	// was lost; deathFlickerFrames is how many of those play the CRT-dying
	// glitch before it settles on the plain CONNECTION LOST screen (zero
	// under reduced motion — cut straight to the final frame).
	deathFrame         int
	deathFlickerFrames int
	deathFrameText     string

	// Combat-mode interstitial (internal/tui/combat.go): combatEntered
	// tracks whether the last processed snapshot was already in
	// PhaseCombat, so the amber "COMBAT MODE ENGAGED" card only fires once
	// per engagement; combatIntroActive is whether that card is currently
	// showing (any key, or a ~2s timer, dismisses it).
	combatEntered     bool
	combatIntroActive bool
}

// NewGame constructs the session UI.
func NewGame(id identity.SessionIdentity, attach game.AttachResult, c *content.Content, w, h int, now, idleSecs int64, devMode bool) Model {
	g := &Game{
		sess: attach.Session, content: c, id: id, attach: attach, devMode: devMode,
		width: w, height: h, now: now, idleSecs: idleSecs, lastInput: now,
		scr: scrChart, created: attach.Created, hits: hitbox.New(),
	}
	if attach.Created {
		g.overlay = ovOnboard
	}
	snap, _ := g.sess.SnapshotNow()
	g.snap = snap
	// Open the session where the pilot actually is. WorldIdx, SystemID and
	// Belt survive a disconnect by design (framework/02), so hardcoding the
	// chart stranded belt-side pilots on a dock screen whose every service is
	// IsDocked()-gated. Run is never persisted, so no restore can land on
	// scrMining; an empty Belt still routes here, because the belt screen
	// handles it and the chart is the softlock.
	// See docs/framework/05-reconnect-and-location-restore.md.
	if g.scr = initialScreen(&snap.State); g.scr == scrBelt {
		g.rockSel = 0
	}
	// Tell a returning pilot what their autopilot did. Never shown over
	// onboarding — a freshly created pilot has no run history to report.
	if snap.State.DisconnectNotice != nil && g.overlay == ovNone {
		g.signalLost = snap.State.DisconnectNotice
		g.overlay = ovSignalLost
	}
	for i := range c.Worlds {
		if c.Worlds[i].SystemID == snap.State.SystemID && sim.RouteLockReason(&snap.State, c, i) == "" {
			g.worldSel = i
			break
		}
	}
	return g
}

// initialScreen picks the screen a session opens on from the restored save.
// A pilot whose save says they are at a belt opens at that belt, including
// when the belt is empty — they mined it out, the belt screen handles that,
// and routing them to the chart instead is the framework/05 softlock.
func initialScreen(st *sim.State) screen {
	if st.IsDocked() {
		return scrChart
	}
	return scrBelt
}

func (g *Game) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		listenKick(g.sess),
		listenSnaps(g.sess),
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

const deathFlickerInterval = 80 * time.Millisecond

func deathTickCmd() tea.Cmd {
	return tea.Tick(deathFlickerInterval, func(t time.Time) tea.Msg { return deathTickMsg(t) })
}

func listenKick(s *game.Session) tea.Cmd {
	return func() tea.Msg {
		reason, ok := <-s.Kicked()
		if !ok {
			return nil
		}
		return kickedMsg(reason)
	}
}

func listenSnaps(s *game.Session) tea.Cmd {
	return func() tea.Msg {
		snap, ok := <-s.Snapshots()
		if !ok {
			return nil
		}
		return snapMsg(snap)
	}
}

func (g *Game) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		g.width, g.height = m.Width, m.Height
	case tickMsg:
		g.tickCount++
		g.now = time.Now().Unix()
		if g.now-g.lastInput >= g.idleSecs {
			return g, tea.Quit
		}
		if g.flash.expires > 0 && g.now >= g.flash.expires {
			g.flash = flash{}
		}
		cmds = append(cmds, g.consumeRunOutcome()...)
		cmds = append(cmds, tickCmd())
	case jumpTickMsg:
		if g.scr == scrJump && m.generation == g.jumpGeneration && g.jumpBeat > 0 {
			g.jumpBeat--
			if g.jumpBeat > 0 {
				cmds = append(cmds, jumpTickCmd(g.jumpGeneration))
			} else {
				g.jumpFlash = true
				cmds = append(cmds, jumpFlashCmd(g.jumpGeneration))
			}
		}
	case jumpFlashDoneMsg:
		if g.scr == scrJump && m.generation == g.jumpGeneration {
			g.jumpFlash = false
		}
	case deathTickMsg:
		if g.scr == scrDeath && g.deathFrame < g.deathFlickerFrames {
			g.deathFrame++
			if g.deathFrame < g.deathFlickerFrames {
				cmds = append(cmds, deathTickCmd())
			}
		}
	case snapMsg:
		if m.Revision < g.snap.Revision {
			cmds = append(cmds, listenSnaps(g.sess))
			break
		}
		g.snap = sim.Snapshot(m)
		if g.snap.State.Run != nil && g.scr != scrMining {
			g.scr = scrMining
		}
		cmds = append(cmds, g.syncCombatEntry()...)
		cmds = append(cmds, g.consumeRunOutcome()...)
		cmds = append(cmds, listenSnaps(g.sess))
	case combatIntroDoneMsg:
		g.combatIntroActive = false
	case kickedMsg:
		if m != "" {
			g.kickReason = string(m)
			g.overlay = ovKicked
		}
	case tea.KeyPressMsg:
		if m.String() == "ctrl+c" {
			return g, tea.Quit
		}
		g.lastInput = g.now
		if g.combatIntroActive {
			g.combatIntroActive = false
			break
		}
		if g.overlay != ovNone {
			cmds = append(cmds, g.updateOverlay(m)...)
			break
		}
		if g.width < minWidth || g.height < minHeight {
			break
		}
		cmds = append(cmds, g.updateKey(m)...)
	case tea.MouseClickMsg:
		g.lastInput = g.now
		if g.width < minWidth || g.height < minHeight {
			break
		}
		if g.overlay != ovNone {
			cmds = append(cmds, g.updateOverlayClick(m)...)
			break
		}
		cmds = append(cmds, g.updateClick(m)...)
	case tea.MouseWheelMsg:
		g.lastInput = g.now
		if g.overlay == ovHelp || g.overlay == ovTweaks || g.overlay == ovDev || g.overlay == ovSlotPicker {
			cmds = append(cmds, g.updateOverlayWheel(m)...)
			break
		}
		if g.overlay != ovNone {
			break
		}
		cmds = append(cmds, g.updateWheel(m)...)
	}

	return g, tea.Batch(cmds...)
}

func (g *Game) updateOverlay(m tea.KeyPressMsg) []tea.Cmd {
	switch g.overlay {
	case ovKicked:
		if m.String() != "" {
			return []tea.Cmd{tea.Quit}
		}
	case ovOnboard:
		if m.String() == "" {
			break
		}
		if g.onboardPg >= len(onboardPages)-1 {
			g.overlay = ovNone
			g.onboardPg = 0
		} else {
			g.onboardPg++
		}
	case ovSignalLost:
		if m.String() == "" {
			break
		}
		return g.dismissSignalLost()
	case ovHelp:
		return g.updateHelpOverlay(m.String())
	case ovTweaks:
		return g.updateTweaksOverlay(m.String())
	case ovDev:
		return g.updateDevOverlay(m.String())
	case ovSlotPicker:
		return g.updateSlotPickerOverlay(m.String())
	case ovSlotRemove:
		return g.updateSlotRemoveOverlay(m.String())
	case ovJump:
		return g.updateJumpConfirm(m.String())
	case ovCertify:
		return g.updateCertify(m.String())
	case ovFerry:
		return g.updateFerry(m.String())
	case ovPermit:
		return g.updatePermitOverlay(m.String())
	}
	return nil
}

func (g *Game) updateKey(m tea.KeyPressMsg) []tea.Cmd {
	k := m.String()
	if g.scr == scrJump {
		return g.keyJump(k)
	}
	// The death screen bypasses overlay compositing entirely (see View()),
	// so an overlay opened here would be invisible and unclosable — never
	// let these global bindings open one while g.scr == scrDeath.
	if k == "?" && g.scr != scrDeath {
		if g.overlay == ovHelp {
			g.overlay = ovNone
		} else {
			g.helpScroll = 0
			g.overlay = ovHelp
		}
		return nil
	}
	if k == "ctrl+c" {
		return []tea.Cmd{tea.Quit}
	}
	if g.devMode && k == "ctrl+d" && g.scr != scrDeath {
		if g.overlay == ovDev {
			g.overlay = ovNone
		} else {
			g.devSel = 0
			g.overlay = ovDev
		}
		return nil
	}
	switch g.scr {
	case scrChart:
		return g.keyChart(k)
	case scrShipyard:
		return g.keyShipyard(k)
	case scrBelt:
		return g.keyBelt(k)
	case scrMining:
		return g.keyMining(m)
	case scrDeath:
		return g.keyDeath(k)
	case scrSummary:
		return g.keySummary(k)
	case scrLog:
		return g.keyLog(k)
	}
	return nil
}

// dismissSignalLost closes the reconnect notice and acknowledges it in the
// save, so it is shown exactly once (framework/05).
func (g *Game) dismissSignalLost() []tea.Cmd {
	g.overlay = ovNone
	g.signalLost = nil
	snap, err := g.sess.AckDisconnectNotice(g.now)
	if err != nil {
		return nil
	}
	return g.refreshSnap(snap, nil)
}

func (g *Game) setFlash(text string) {
	g.flash = flash{text: text, expires: g.now + 2}
}

// consumeRunOutcome moves a finished run directly to its summary. It is
// called after both tick snapshots and player-triggered snapshots so a pirate
// destroyed by an autocannon or a manual volley never briefly returns to the
// mining view.
func (g *Game) consumeRunOutcome() []tea.Cmd {
	out := g.sess.LastOutcome()
	if out == nil || g.scr != scrMining || g.snap.State.Run != nil {
		return nil
	}
	g.lastOutcome = out
	g.sess.ClearOutcome()
	// A fast autocannon kill can happen while the combat-entry card is still
	// held. It must not cover the completed run summary.
	g.combatIntroActive = false
	if out.Kind != sim.OutcomeShipLost {
		g.scr = scrSummary
		return nil
	}
	g.deathFrameText = ansi.Strip(g.assembleFrameContent(g.renderChrome() + "\n" + g.renderScreen()))
	g.deathFrame = 0
	g.deathFlickerFrames = 12
	if g.snap.State.Settings.ReducedMotion {
		g.deathFlickerFrames = 0
	}
	g.scr = scrDeath
	// The death screen bypasses overlay compositing entirely (see View()), but
	// Update()'s key routing checks overlay state first — leaving one open here
	// would swallow every keypress into a stale, invisible overlay handler.
	g.overlay = ovNone
	if g.deathFlickerFrames > 0 {
		return []tea.Cmd{deathTickCmd()}
	}
	return nil
}

func (g *Game) refreshSnap(snap sim.Snapshot, err error) []tea.Cmd {
	if err != nil {
		g.setFlash(err.Error())
		return nil
	}
	if snap.Revision < g.snap.Revision {
		return nil
	}
	g.snap = snap
	if g.snap.State.Run != nil {
		g.scr = scrMining
	}
	cmds := g.syncCombatEntry()
	return append(cmds, g.consumeRunOutcome()...)
}

func (g *Game) View() tea.View {
	g.hits.Clear()
	var body string
	if g.width < minWidth || g.height < minHeight {
		body = lipgloss.Place(g.width, g.height, lipgloss.Center, lipgloss.Center,
			fmt.Sprintf("RESIZE TERMINAL — need %dx%d, have %dx%d", minWidth, minHeight, g.width, g.height))
	} else {
		var frame string
		switch {
		case g.scr == scrJump:
			frame = g.renderJump()
		case g.scr == scrDeath:
			frame = g.renderDeath()
		case g.combatIntroActive:
			frame = g.renderCombatIntro()
		default:
			frame = g.compositeView(g.renderChrome() + "\n" + g.renderScreen())
		}
		body = g.placeFrame(g.assembleFrameContent(frame))
	}
	v := tea.NewView(body)
	v.AltScreen = true
	v.WindowTitle = fmt.Sprintf("MOON MINER — %s", strings.ToUpper(g.id.Slot))
	return v
}

func (g *Game) screenAccent() string {
	switch g.scr {
	case scrShipyard:
		return theme.Accent(theme.HueViolet)
	case scrBelt:
		return theme.Accent(theme.HueGold)
	case scrMining:
		return theme.Accent(theme.HueAmber)
	case scrDeath:
		return theme.OutcomeAccent(string(sim.OutcomeShipLost))
	case scrSummary:
		if g.lastOutcome != nil {
			return theme.OutcomeAccent(string(g.lastOutcome.Kind))
		}
		return theme.Accent(theme.HueGold)
	case scrLog:
		return theme.Accent(theme.HueCyan)
	default:
		return theme.Accent(theme.HueCyan)
	}
}

func (g *Game) renderChrome() string {
	st := g.snap.State
	fuelAmount := sim.FuelAmount(&st, g.content)
	fuelPct := fuelAmount / sim.TankSize(&st, g.content) * 100
	accent := g.screenAccent()
	accentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(accent))

	title := theme.Cyan.Render("MOON MINER")
	pilot := theme.Violet.Render("PILOT: " + strings.ToUpper(g.id.Slot))
	credits := theme.Gold.Render(fmt.Sprintf("%s %d", theme.Glyph("credit", st.Settings.ASCIISafe), st.Credits))
	fuel := theme.FuelStyle(fuelPct).Render(fmt.Sprintf("%s %.0f%%", theme.Glyph("fuel", st.Settings.ASCIISafe), fuelPct))
	hullPct := sim.HullPct(&st, g.content)
	hull := theme.HullStyle(hullPct).Render(fmt.Sprintf("HULL %.0f%%", hullPct))
	shield := g.renderShieldStatus(&st, 0)
	cargoUnits := st.CargoUnits + sim.RunHeldUnits(st.Run)
	cargo := theme.Gold.Render(fmt.Sprintf("CARGO %.0f/%.0f", cargoUnits, sim.CargoCapacityUnits(&st, g.content)))

	// Keep cargo in the shared nav chrome, including live run cargo. The
	// compact fields guarantee it remains visible even at the 80-column
	// minimum terminal width rather than falling off the right edge.
	hud := fmt.Sprintf(" %s · %s · %s · %s · %s · %s · %s ", title, pilot, credits, fuel, hull, shield, cargo)
	if lipgloss.Width(hud) > g.contentWidth() {
		// Preserve both defensive charge and cargo in compact mode. The pilot
		// label and verbose fuel/credit spacing are the expendable chrome.
		compactPilot := theme.Violet.Render(strings.ToUpper(g.id.Slot))
		compactCredits := theme.Gold.Render(fmt.Sprintf("%s%d", theme.Glyph("credit", st.Settings.ASCIISafe), st.Credits))
		compactFuel := theme.FuelStyle(fuelPct).Render(fmt.Sprintf("%s%.0f%%", theme.Glyph("fuel", st.Settings.ASCIISafe), fuelPct))
		compactHull := theme.HullStyle(hullPct).Render(fmt.Sprintf("HULL%.0f%%", hullPct))
		hud = fmt.Sprintf(" %s · %s · %s · %s · %s · %s · %s ", title, compactPilot, compactCredits, compactFuel, compactHull, shield, cargo)
	}
	if lipgloss.Width(hud) > g.contentWidth() {
		hud = fmt.Sprintf(" %s · %s · %s · %s · %s ", title, pilot, hull, shield, cargo)
	}
	hudLine := accentStyle.Render(strings.Repeat("─", g.contentWidth()))
	return hudLine + "\n" + hud + "\n" + hudLine
}

// renderShieldStatus renders a compact shield readout for the shared HUD and
// a bar-backed detailed readout for dock and belt screens. Every normal
// screen uses renderChrome, so the compact form keeps the shield visible
// throughout play while the detailed form exposes recovery at the belt.
func (g *Game) renderShieldStatus(st *sim.State, width int) string {
	hp, maxHP, damaged := sim.ShieldStatus(st, g.content)
	if maxHP <= 0 {
		return theme.DimStyle.Render("SHIELD —")
	}
	pct := hp / maxHP * 100
	status := ""
	switch {
	case damaged:
		status = " " + theme.Red.Render("DAMAGED")
	case st.WorldIdx >= 0 && sim.ShieldRechargeETA(st, g.content) > 0:
		status = " " + theme.Cyan.Render(fmt.Sprintf("RECHARGING %.0fs", sim.ShieldRechargeETA(st, g.content)))
	case st.IsDocked():
		status = " " + theme.Green.Render("CHARGED")
	}
	if width <= 0 {
		return theme.Cyan.Render(fmt.Sprintf("SHD %.0f/%.0f", hp, maxHP)) + status
	}
	if width <= 10 {
		return fmt.Sprintf("SHIELD %s %s", theme.HullShieldBar(100, pct, width), theme.Cyan.Render(fmt.Sprintf("%.0f/%.0f", hp, maxHP)))
	}
	return fmt.Sprintf("SHIELD %s %3.0f%% %s%s",
		theme.HullShieldBar(100, pct, width), pct,
		theme.Cyan.Render(fmt.Sprintf("%.0f/%.0f", hp, maxHP)), status)
}

func (g *Game) renderDockShieldService(st *sim.State) string {
	_, maxHP, damaged := sim.ShieldStatus(st, g.content)
	if maxHP <= 0 {
		return theme.DimStyle.Render("NO SHIELD INSTALLED")
	}
	if damaged {
		return theme.Red.Render("SHIELD SERVICE REQUIRED")
	}
	return theme.Green.Render("SHIELD SERVICE — CHARGED")
}

func (g *Game) renderKeybar(hint string) string {
	shipClass := "miner"
	if model := g.content.ShipByID(g.snap.State.ActiveShipID); model != nil {
		shipClass = model.Class
	}
	marker := theme.ShipMarker(shipClass)
	bar := hint
	if g.flash.text == "" {
		bar = theme.KeybarHint(hint)
	} else {
		bar = theme.Red.Render(g.flash.text)
	}
	bar = ansi.Truncate(bar, g.contentWidth()-lipgloss.Width(marker), "…")
	pad := g.contentWidth() - lipgloss.Width(bar) - lipgloss.Width(marker)
	if pad < 0 {
		pad = 0
	}
	bar = bar + strings.Repeat(" ", pad) + marker
	accent := g.screenAccent()
	ruleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(accent)).Faint(true)
	return ruleStyle.Render(strings.Repeat("─", g.contentWidth())) + "\n" + bar
}
