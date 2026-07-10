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

	worldSel  int
	rockSel   int
	logScroll int

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

	// Death-sequence flicker: deathFrame counts fast ticks since the ship
	// was lost; deathFlickerFrames is how many of those play the CRT-dying
	// glitch before it settles on the plain CONNECTION LOST screen (zero
	// under reduced motion — cut straight to the final frame).
	deathFrame         int
	deathFlickerFrames int
	deathFrameText     string
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
	return g
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
		if out := g.sess.LastOutcome(); out != nil && g.scr == scrMining {
			g.lastOutcome = out
			if out.Kind == sim.OutcomeShipLost {
				g.deathFrameText = ansi.Strip(g.renderChrome() + "\n" + g.renderScreen())
				g.deathFrame = 0
				g.deathFlickerFrames = 12
				if g.snap.State.Settings.ReducedMotion {
					g.deathFlickerFrames = 0
				}
				g.scr = scrDeath
				// The death screen bypasses overlay compositing entirely
				// (see View()), but Update()'s key routing checks overlay
				// state first — leaving one open here would swallow every
				// keypress into a stale overlay handler and strand the
				// player on "CONNECTION LOST" with no way to continue.
				g.overlay = ovNone
				if g.deathFlickerFrames > 0 {
					cmds = append(cmds, deathTickCmd())
				}
			} else {
				g.scr = scrSummary
			}
			g.sess.ClearOutcome()
		}
		cmds = append(cmds, tickCmd())
	case deathTickMsg:
		if g.scr == scrDeath && g.deathFrame < g.deathFlickerFrames {
			g.deathFrame++
			if g.deathFrame < g.deathFlickerFrames {
				cmds = append(cmds, deathTickCmd())
			}
		}
	case snapMsg:
		g.snap = sim.Snapshot(m)
		if g.snap.State.Run != nil && g.scr != scrMining {
			g.scr = scrMining
		}
		cmds = append(cmds, listenSnaps(g.sess))
	case kickedMsg:
		if m != "" {
			g.kickReason = string(m)
			g.overlay = ovKicked
		}
	case tea.KeyPressMsg:
		g.lastInput = g.now
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
	}
	return nil
}

func (g *Game) updateKey(m tea.KeyPressMsg) []tea.Cmd {
	k := m.String()
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

func (g *Game) setFlash(text string) {
	g.flash = flash{text: text, expires: g.now + 2}
}

func (g *Game) refreshSnap(snap sim.Snapshot, err error) []tea.Cmd {
	if err != nil {
		g.setFlash(err.Error())
		return nil
	}
	g.snap = snap
	if g.snap.State.Run != nil {
		g.scr = scrMining
	}
	return nil
}

func (g *Game) View() tea.View {
	g.hits.Clear()
	var body string
	if g.width < minWidth || g.height < minHeight {
		body = lipgloss.Place(g.width, g.height, lipgloss.Center, lipgloss.Center,
			fmt.Sprintf("RESIZE TERMINAL — need %dx%d, have %dx%d", minWidth, minHeight, g.width, g.height))
	} else if g.scr == scrDeath {
		body = g.renderDeath()
	} else {
		base := g.renderChrome() + "\n" + g.renderScreen()
		body = g.compositeView(base)
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
	fuelPct := st.Fuel / sim.TankSize(&st, g.content) * 100
	accent := g.screenAccent()
	accentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(accent))

	title := theme.Cyan.Render("MOON MINER")
	pilot := theme.Violet.Render("PILOT: " + strings.ToUpper(g.id.Slot))
	credits := theme.Gold.Render(fmt.Sprintf("%s %d", theme.Glyph("credit", st.Settings.ASCIISafe), st.Credits))
	fuel := theme.FuelStyle(fuelPct).Render(fmt.Sprintf("FUEL %.0f%%", fuelPct)) + " " + theme.FuelBar(fuelPct, 8)
	hullPct := sim.HullPct(&st, g.content)
	hull := theme.HullStyle(hullPct).Render(fmt.Sprintf("HULL %.0f%%", hullPct))

	hud := fmt.Sprintf(" %s — %s — %s  %s  %s ", title, pilot, credits, fuel, hull)
	hudLine := accentStyle.Render(strings.Repeat("─", g.width))
	return hudLine + "\n" + hud + "\n" + hudLine
}

func (g *Game) renderKeybar(hint string) string {
	cursor := theme.Cursor(g.tickCount, g.snap.State.Settings.ReducedMotion)
	bar := hint
	if g.flash.text == "" {
		bar = theme.KeybarHint(hint)
	} else {
		bar = theme.Red.Render(g.flash.text)
	}
	pad := g.width - lipgloss.Width(bar) - lipgloss.Width(cursor)
	if pad < 0 {
		pad = 0
	}
	bar = bar + strings.Repeat(" ", pad) + cursor
	accent := g.screenAccent()
	ruleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(accent)).Faint(true)
	return ruleStyle.Render(strings.Repeat("─", g.width)) + "\n" + bar
}
