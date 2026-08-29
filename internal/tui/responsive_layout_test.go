package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
)

func TestResponsiveFrameContract(t *testing.T) {
	for _, vp := range responsiveViewports {
		t.Run(vp.String(), func(t *testing.T) {
			g := newResponsiveTestGame(t, vp.w, vp.h)
			out := g.View().Content
			assertPhysicalBounds(t, out, vp.w, vp.h)

			if vp.w < minWidth || vp.h < minHeight {
				assertSemanticAnchors(t, out, "RESIZE TERMINAL", "need 80x24", "have 79x23")
				return
			}

			frameW := min(vp.w, 144)
			frameH := min(vp.h, 48)
			wantX := (vp.w - frameW) / 2
			wantY := (vp.h - frameH) / 2
			bounds := nonBlankBounds(out)
			if !bounds.ok {
				t.Fatal("responsive frame rendered no visible content")
			}
			if bounds.left != wantX || bounds.right != wantX+frameW-1 {
				t.Errorf("frame horizontal bounds = [%d,%d], want [%d,%d]", bounds.left, bounds.right, wantX, wantX+frameW-1)
			}
			if bounds.top != wantY || bounds.bottom != wantY+frameH-1 {
				t.Errorf("frame vertical bounds = [%d,%d], want [%d,%d]", bounds.top, bounds.bottom, wantY, wantY+frameH-1)
			}
		})
	}
}

func TestResponsivePaneMinimumsAndWeights(t *testing.T) {
	for _, width := range []int{80, 100, 120, 144, 160, 200} {
		t.Run(fmt.Sprintf("chart/%d", width), func(t *testing.T) {
			g := newResponsiveTestGame(t, width, 40)
			top := firstLineContaining(g.renderChart(), "STAR CHART")
			starts := panelStarts(top)
			if len(starts) != 2 {
				t.Fatalf("chart rendered %d pane starts, want 2: %q", len(starts), ansiPlain(top))
			}
			frameW := min(width, 144)
			panes := []int{starts[1] - starts[0], frameW - starts[1]}
			assertPaneContract(t, panes, []int{40, 40}, []int{1, 1}, frameW)
		})

		t.Run(fmt.Sprintf("shipyard/%d", width), func(t *testing.T) {
			g := newResponsiveTestGame(t, width, 40)
			g.scr = scrShipyard
			top := firstLineContaining(g.renderShipyard(), "HANGAR")
			starts := panelStarts(top)
			if len(starts) != 3 {
				t.Fatalf("shipyard rendered %d pane starts, want 3: %q", len(starts), ansiPlain(top))
			}
			frameW := min(width, 144)
			panes := []int{starts[1] - starts[0], starts[2] - starts[1], frameW - starts[2]}
			assertPaneContract(t, panes, []int{24, 32, 24}, []int{3, 4, 3}, frameW)
		})

		t.Run(fmt.Sprintf("mining/%d", width), func(t *testing.T) {
			g := newResponsiveTestGame(t, width, 40)
			left, right := g.miningColumnWidths()
			frameW := min(width, 144)
			assertPaneContract(t, []int{left, right}, []int{46, 34}, []int{3, 2}, frameW)
		})
	}
}

func TestRenderableVariantsRespectViewportAndKeepSemanticAnchors(t *testing.T) {
	variants := responsiveRenderVariants()
	for _, vp := range responsiveViewports[1:] {
		for _, variant := range variants {
			t.Run(vp.String()+"/"+variant.name, func(t *testing.T) {
				g := variant.build(t, vp.w, vp.h)
				out := g.View().Content
				assertPhysicalBounds(t, out, vp.w, vp.h)
				assertSemanticAnchors(t, out, variant.anchors...)
			})
		}
	}
}

func TestSharedVerticalBudgetPinsEveryNormalScreenToFrameBottom(t *testing.T) {
	for _, vp := range []testViewport{{120, 40}, {160, 60}, {200, 60}} {
		for _, variant := range responsiveRenderVariants() {
			if variant.overlay || variant.fullScreen {
				continue
			}
			t.Run(vp.String()+"/"+variant.name, func(t *testing.T) {
				g := variant.build(t, vp.w, vp.h)
				out := g.View().Content
				bounds := nonBlankBounds(out)
				wantBottom := (vp.h-min(vp.h, 48))/2 + min(vp.h, 48) - 1
				if !bounds.ok || bounds.bottom != wantBottom {
					t.Errorf("screen bottom = %d, want shared frame bottom %d", bounds.bottom, wantBottom)
				}
			})
		}
	}
}

type responsiveRenderVariant struct {
	name       string
	anchors    []string
	overlay    bool
	fullScreen bool
	build      func(testing.TB, int, int) *Game
}

func responsiveRenderVariants() []responsiveRenderVariant {
	base := func(t testing.TB, w, h int) *Game { return newResponsiveTestGame(t, w, h) }
	return []responsiveRenderVariant{
		{name: "chart-docked", anchors: []string{"STAR CHART", "PORT SERVICES", "SHIPYARD"}, build: base},
		{name: "chart-belt-side", anchors: []string{"PORT SERVICES — OFFLINE", "RETURN TO BELT"}, build: func(t testing.TB, w, h int) *Game {
			g := base(t, w, h)
			departResponsiveTestGame(t, g)
			return g
		}},
		{name: "shipyard-owned", anchors: []string{"HANGAR", "STATUS", "JUMP DRIVE"}, build: func(t testing.TB, w, h int) *Game {
			g := base(t, w, h)
			g.scr = scrShipyard
			return g
		}},
		{name: "shipyard-unowned", anchors: []string{"NOT OWNED", "ACQUIRE", "Purchase does not switch ships."}, build: func(t testing.TB, w, h int) *Game {
			g := base(t, w, h)
			g.scr = scrShipyard
			g.shipyardHangarSel = 1
			return g
		}},
		{name: "belt-tiles", anchors: []string{"ASTEROID BELT", "DATA TILES", "TARGET LOCK", "LOCK & FLY"}, build: func(t testing.TB, w, h int) *Game {
			g := base(t, w, h)
			departResponsiveTestGame(t, g)
			g.snap.State.Settings.BeltView = sim.BeltViewTiles
			g.scr = scrBelt
			return g
		}},
		{name: "belt-ore-scan", anchors: []string{"ASTEROID BELT", "ORE SCAN", "TARGET LOCK"}, build: func(t testing.TB, w, h int) *Game {
			g := base(t, w, h)
			departResponsiveTestGame(t, g)
			g.snap.State.Settings.BeltView = sim.BeltViewOreScan
			g.scr = scrBelt
			return g
		}},
		{name: "belt-radar", anchors: []string{"ASTEROID BELT", "RADAR SCOPE", "TARGET LOCK"}, build: func(t testing.TB, w, h int) *Game {
			g := base(t, w, h)
			departResponsiveTestGame(t, g)
			g.snap.State.Settings.BeltView = sim.BeltViewRadar
			g.scr = scrBelt
			return g
		}},
		{name: "belt-depleted", anchors: []string{"ASTEROID BELT", "Belt depleted", "Q DOCK"}, build: func(t testing.TB, w, h int) *Game {
			g := base(t, w, h)
			departResponsiveTestGame(t, g)
			g.snap.State.Belt = nil
			g.scr = scrBelt
			return g
		}},
		{name: "mining-drilling", anchors: []string{"MINING SITE", "RESOURCE LEFT", "PIRATE ETA", "BAIL"}, build: func(t testing.TB, w, h int) *Game {
			g := base(t, w, h)
			startResponsiveTestRun(t, g)
			return g
		}},
		{name: "mining-skill-check", anchors: []string{"PRESSURE POINT ACTIVE", "FRACTURE", "BAIL"}, build: func(t testing.TB, w, h int) *Game {
			g := base(t, w, h)
			_, run, _ := startResponsiveTestRun(t, g)
			run.SkillCheck = &sim.SkillCheck{Window: 3}
			return g
		}},
		{name: "mining-tribute", anchors: []string{"TRANSMISSION", "DROP CARGO", "REFUSE / RUN"}, build: func(t testing.TB, w, h int) *Game {
			g := base(t, w, h)
			_, run, _ := startResponsiveTestRun(t, g)
			run.Phase = sim.PhaseTribute
			return g
		}},
		{name: "mining-escape", anchors: []string{"ESCAPE BURN", "ESCAPE VECTOR", "no menu actions accepted"}, build: func(t testing.TB, w, h int) *Game {
			g := base(t, w, h)
			_, run, _ := startResponsiveTestRun(t, g)
			run.Phase = sim.PhaseEscaping
			run.EscapeSecondsRequired = 10
			run.EscapeSecondsElapsed = 3
			return g
		}},
		{name: "mining-combat", anchors: []string{"TACTICAL SCOPE", "TEST RAIDER", "BAIL"}, build: func(t testing.TB, w, h int) *Game {
			g := base(t, w, h)
			_, run, _ := startResponsiveTestRun(t, g)
			run.Phase = sim.PhaseCombat
			run.Combat = &sim.CombatState{PirateName: "TEST RAIDER", PirateHull: 40, PirateMaxHull: 50, Bounty: 75, OddsPct: 45, Bearing: .4, Range: .6}
			return g
		}},
		{name: "summary-departed", anchors: []string{"RUN SUMMARY", "DEPARTED", "CARGO SEALED", "RETURN TO BELT"}, build: func(t testing.TB, w, h int) *Game {
			g := base(t, w, h)
			g.scr = scrSummary
			g.lastOutcome = testOutcome(sim.OutcomeDeparted)
			return g
		}},
		{name: "summary-bailed", anchors: []string{"RUN SUMMARY", "BAILED", "CARGO SEALED", "RETURN TO BELT"}, build: func(t testing.TB, w, h int) *Game {
			g := base(t, w, h)
			g.scr = scrSummary
			g.lastOutcome = testOutcome(sim.OutcomeBailed)
			return g
		}},
		{name: "summary-tribute-paid", anchors: []string{"RUN SUMMARY", "TRIBUTE PAID", "CARGO SEALED", "RETURN TO BELT"}, build: func(t testing.TB, w, h int) *Game {
			g := base(t, w, h)
			g.scr = scrSummary
			g.lastOutcome = testOutcome(sim.OutcomeTributePaid)
			return g
		}},
		{name: "summary-escaped-under-fire", anchors: []string{"RUN SUMMARY", "ESCAPED UNDER FIRE", "CARGO SEALED", "RETURN TO BELT"}, build: func(t testing.TB, w, h int) *Game {
			g := base(t, w, h)
			g.scr = scrSummary
			g.lastOutcome = testOutcome(sim.OutcomeEscapedUnderFire)
			return g
		}},
		{name: "summary-pirate-destroyed", anchors: []string{"RUN SUMMARY", "PIRATE DESTROYED", "BOUNTY", "RETURN TO BELT"}, build: func(t testing.TB, w, h int) *Game {
			g := base(t, w, h)
			g.scr = scrSummary
			g.lastOutcome = testOutcome(sim.OutcomePirateDestroyed)
			g.lastOutcome.Record.PirateDestroyed = "TEST RAIDER"
			g.lastOutcome.Record.BountyEarned = 75
			return g
		}},
		{name: "summary-ship-lost", anchors: []string{"RESPAWN — SHIP LOST", "CARGO LOST", "RESPAWN AT DOCK"}, build: func(t testing.TB, w, h int) *Game {
			g := base(t, w, h)
			g.scr = scrSummary
			g.lastOutcome = testOutcome(sim.OutcomeShipLost)
			return g
		}},
		{name: "ships-log", anchors: []string{"SHIP'S LOG", "SERVICE RECORD", "RECENT RUNS"}, build: func(t testing.TB, w, h int) *Game {
			g := base(t, w, h)
			g.scr = scrLog
			return g
		}},
		{name: "death-final", anchors: []string{"CONNECTION LOST", "press any key"}, fullScreen: true, build: func(t testing.TB, w, h int) *Game {
			g := base(t, w, h)
			g.scr = scrDeath
			return g
		}},
		{name: "combat-intro", anchors: []string{"COMBAT MODE ENGAGED", "TEST RAIDER", "brace for impact"}, fullScreen: true, build: func(t testing.TB, w, h int) *Game {
			g := base(t, w, h)
			g.combatIntroActive = true
			g.snap.State.Run = &sim.ActiveRun{Combat: &sim.CombatState{PirateName: "TEST RAIDER", Bounty: 75}}
			return g
		}},
		{name: "overlay-help", anchors: []string{"HELP — STAR CHART", "GLOBAL", "close"}, overlay: true, build: overlayVariant(ovHelp)},
		{name: "overlay-tweaks", anchors: []string{"TWEAKS", "BELT VIEW", "LONG TEXT"}, overlay: true, build: overlayVariant(ovTweaks)},
		{name: "overlay-dev", anchors: []string{"DEV TOOLS", "DEV ONLY", "GOD MODE"}, overlay: true, build: overlayVariant(ovDev)},
		{name: "overlay-onboarding", anchors: []string{"ONBOARDING", "Welcome, pilot", "Press any key"}, overlay: true, build: overlayVariant(ovOnboard)},
		{name: "overlay-kicked", anchors: []string{"NOTICE", "duplicate session", "Press any key"}, overlay: true, build: func(t testing.TB, w, h int) *Game {
			g := overlayVariant(ovKicked)(t, w, h)
			g.kickReason = "duplicate session"
			return g
		}},
		{name: "overlay-signal-lost", anchors: []string{"RECONNECTED", "SIGNAL LOST", "CARGO SEALED"}, overlay: true, build: func(t testing.TB, w, h int) *Game {
			g := overlayVariant(ovSignalLost)(t, w, h)
			g.signalLost = &sim.RunRecord{Outcome: string(sim.OutcomeBailed), Asteroid: "A-01", CargoValueRecovered: 50}
			return g
		}},
		{name: "overlay-permit", anchors: []string{"PERMIT GATE", "ROUTE PERMIT REQUIRED", "BUY PERMIT"}, overlay: true, build: func(t testing.TB, w, h int) *Game {
			g := overlayVariant(ovPermit)(t, w, h)
			g.pendingPermitWorld = 0
			return g
		}},
		{name: "overlay-slot-picker", anchors: []string{"INSTALL — UTILITY 1", "CATALOG", "install"}, overlay: true, build: func(t testing.TB, w, h int) *Game {
			g := base(t, w, h)
			g.scr = scrShipyard
			g.shipyardPane = shipyardPaneLoadout
			rows := shipyardRows(g.content.ShipByID("skiff"))
			for i, row := range rows {
				if row.kind == rowUtility {
					g.shipyardRowSel = i
					g.openSlotPicker(g.snap.State.Ships["skiff"], row)
					break
				}
			}
			return g
		}},
		{name: "overlay-slot-remove", anchors: []string{"REMOVE MODULE", "STORE", "SELL"}, overlay: true, build: func(t testing.TB, w, h int) *Game {
			g := base(t, w, h)
			g.scr = scrShipyard
			if err := sim.InstallSlotDevice(&g.snap.State, g.content, "skiff", sim.SlotUtility, 0, sim.ItemShield, 0); err != nil {
				t.Fatal(err)
			}
			g.shipyardPane = shipyardPaneLoadout
			rows := shipyardRows(g.content.ShipByID("skiff"))
			for i, row := range rows {
				if row.kind == rowUtility {
					g.shipyardRowSel = i
					g.overlay = ovSlotRemove
					break
				}
			}
			return g
		}},
	}
}

func overlayVariant(ov overlay) func(testing.TB, int, int) *Game {
	return func(t testing.TB, w, h int) *Game {
		g := newResponsiveTestGame(t, w, h)
		g.overlay = ov
		return g
	}
}

func testOutcome(kind sim.OutcomeKind) *sim.RunOutcome {
	label := sim.OutcomeLabel(kind)
	desc := "Cargo recovered."
	if kind == sim.OutcomeShipLost {
		desc = "The ship and its hold are gone."
	}
	return &sim.RunOutcome{
		Kind:        kind,
		Label:       label,
		Description: desc,
		Record: sim.RunRecord{
			Asteroid:            "A-01",
			CargoValueRecovered: 50,
			CargoValueLost:      50,
		},
	}
}

func assertPaneContract(t testing.TB, panes, minimums, weights []int, total int) {
	t.Helper()
	sum := 0
	for i, pane := range panes {
		sum += pane
		if pane < minimums[i] {
			t.Errorf("pane %d width = %d, want at least %d", i, pane, minimums[i])
		}
	}
	if sum != total {
		t.Errorf("pane widths sum to %d, want effective frame width %d", sum, total)
	}

	minTotal := 0
	for _, minimum := range minimums {
		minTotal += minimum
	}
	if total == minTotal {
		return
	}
	want := expectedPaneWidths(minimums, weights, total)
	for i := range panes {
		if panes[i] != want[i] {
			t.Errorf("pane widths = %v, want exact largest-remainder allocation %v", panes, want)
			return
		}
	}
}

func expectedPaneWidths(minimums, weights []int, total int) []int {
	widths := append([]int(nil), minimums...)
	minTotal, weightTotal := 0, 0
	for i := range minimums {
		minTotal += minimums[i]
		weightTotal += weights[i]
	}
	remaining := total - minTotal
	remainders := make([]int, len(widths))
	assigned := 0
	for i := range widths {
		numerator := remaining * weights[i]
		extra := numerator / weightTotal
		widths[i] += extra
		assigned += extra
		remainders[i] = numerator % weightTotal
	}
	awarded := make([]bool, len(widths))
	for cells := remaining - assigned; cells > 0; cells-- {
		best := -1
		for i := range widths {
			if !awarded[i] && (best < 0 || remainders[i] > remainders[best]) {
				best = i
			}
		}
		widths[best]++
		awarded[best] = true
	}
	return widths
}

func firstLineContaining(out, text string) string {
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(ansiPlain(line), text) {
			return line
		}
	}
	return ""
}

func ansiPlain(s string) string {
	return strings.Join(strippedLines(s), "\n")
}
