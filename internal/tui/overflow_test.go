package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
)

func TestMiningMinimumViewportDoesNotSilentlyClipStatusMeaning(t *testing.T) {
	g := newResponsiveTestGame(t, 80, 24)
	_, run, ast := startResponsiveTestRun(t, g)
	run.CargoValue = 123

	out := g.View().Content
	assertPhysicalBounds(t, out, 80, 24)
	assertSemanticAnchors(t, out,
		"CURRENT CUT VALUE",
		"123",
		fmt.Sprintf("of ◈ %d", ast.Value),
		"(not sold)",
		"PIRATE ETA",
		"[B] BAIL",
	)
}

func TestShipLogLongTextWrapsInsteadOfDroppingItsTail(t *testing.T) {
	const tail = "END-OF-ENTRY-MUST-REMAIN-VISIBLE"
	for _, vp := range supportedViewports(responsiveViewports) {
		t.Run(vp.String(), func(t *testing.T) {
			g := newResponsiveTestGame(t, vp.w, vp.h)
			g.scr = scrLog
			g.snap.State.Settings.WrapLongText = true
			g.snap.State.RunLog = []sim.RunRecord{{
				World:               "Vesta",
				Asteroid:            strings.Repeat("very-long-asteroid-name ", 8) + tail,
				Outcome:             string(sim.OutcomeDeparted),
				CargoValueRecovered: 42,
			}}

			out := g.View().Content
			assertPhysicalBounds(t, out, vp.w, vp.h)
			assertSemanticAnchors(t, out, "RECENT RUNS", tail)
		})
	}
}

func TestResponsiveHitboxesFollowRenderedControls(t *testing.T) {
	for _, vp := range supportedViewports(responsiveViewports) {
		t.Run(vp.String()+"/chart", func(t *testing.T) {
			g := newResponsiveTestGame(t, vp.w, vp.h)
			out := g.View().Content
			assertHitboxAtText(t, g, out, g.content.Worlds[0].Name, "world:0")
			assertHitboxAtText(t, g, out, "REFUEL", "svc:refuel")
			assertHitboxAtText(t, g, out, "SHIPYARD", "btn:shipyard")
		})

		t.Run(vp.String()+"/shipyard", func(t *testing.T) {
			g := newResponsiveTestGame(t, vp.w, vp.h)
			g.scr = scrShipyard
			g.shipyardPane = shipyardPaneLoadout
			out := g.View().Content
			assertHitboxAtText(t, g, out, "▸ "+g.content.Ships[0].Name, "hangar:0")
			assertHitboxAtText(t, g, out, "THRUSTERS", "loadout:0")
		})

		t.Run(vp.String()+"/mining", func(t *testing.T) {
			g := newResponsiveTestGame(t, vp.w, vp.h)
			_, run, _ := startResponsiveTestRun(t, g)
			run.SkillCheck = &sim.SkillCheck{Window: 3}
			out := g.View().Content
			assertHitboxAtText(t, g, out, "PRESSURE POINT ACTIVE", "btn:skillcheck")
			assertHitboxAtText(t, g, out, "BAIL", "btn:bail")
		})

		t.Run(vp.String()+"/tweaks-overlay", func(t *testing.T) {
			g := newResponsiveTestGame(t, vp.w, vp.h)
			g.overlay = ovTweaks
			out := g.View().Content
			assertHitboxAtText(t, g, out, "BELT VIEW", "tweak:0")
		})

		t.Run(vp.String()+"/dev-overlay", func(t *testing.T) {
			g := newResponsiveTestGame(t, vp.w, vp.h)
			g.overlay = ovDev
			out := g.View().Content
			assertHitboxAtText(t, g, out, "GOD MODE", "dev:"+itoa(devGodMode))
		})

		t.Run(vp.String()+"/permit-overlay", func(t *testing.T) {
			g := newResponsiveTestGame(t, vp.w, vp.h)
			g.overlay = ovPermit
			g.pendingPermitWorld = 0
			out := g.View().Content
			assertHitboxAtText(t, g, out, "BUY PERMIT", "permit:buy")
			assertHitboxAtText(t, g, out, "CANCEL", "permit:cancel")
		})
	}
}

func TestChartDescriptionKeepsItsFinalClauseAtEveryViewport(t *testing.T) {
	const finalClause = "FINAL-CLAUSE-MUST-NOT-BE-CLIPPED"
	for _, vp := range supportedViewports(responsiveViewports) {
		t.Run(vp.String(), func(t *testing.T) {
			g := newResponsiveTestGame(t, vp.w, vp.h)
			g.snap.State.Settings.WrapLongText = true
			g.worldSel = 0
			g.content.Worlds[0].Desc = "A deliberately long navigation warning that must wrap within the shared vertical budget. " + finalClause

			out := g.View().Content
			assertPhysicalBounds(t, out, vp.w, vp.h)
			assertSemanticAnchors(t, out, "STAR CHART", finalClause, "PORT SERVICES")
		})
	}
}
