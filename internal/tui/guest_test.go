package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/mynameis-nigel/ssh-moonminer/internal/identity"
)

func TestGuestPresentationAndDiscardConfirm(t *testing.T) {
	for _, vp := range []testViewport{{80, 24}, {108, 32}, {144, 48}} {
		g := newResponsiveTestGame(t, vp.w, vp.h)
		g.id = identity.SessionIdentity{Kind: identity.Guest, GuestID: strings.Repeat("a", 32)}
		v := g.View()
		out := v.Content
		assertPhysicalBounds(t, out, vp.w, vp.h)
		assertSemanticAnchors(t, out, "PILOT: GUEST", "NOT SAVED")
		if !strings.Contains(v.WindowTitle, "GUEST") {
			t.Fatal(v.WindowTitle)
		}
		g.overlay = ovOnboard
		g.onboardPg = 0
		assertSemanticAnchors(t, g.renderOverlay(), "ssh-keygen", "visit")
		g.overlay = ovNone
		g.keyChart("q")
		if g.overlay != ovGuestQuit {
			t.Fatal("guest quit unconfirmed")
		}
		out = g.View().Content
		assertSemanticAnchors(t, out, "discarded", "LEAVE", "STAY")
		assertHitboxAtText(t, g, out, "[ENTER] LEAVE", "guest:leave")
		g.updateGuestQuit("esc")
		if g.overlay != ovNone {
			t.Fatal("cannot cancel")
		}
		g.scr = scrDeath
		g.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
		assertSemanticAnchors(t, g.View().Content, "LEAVE", "NOT SAVED")
		if cmds := g.updateGuestQuit("enter"); len(cmds) != 1 {
			t.Fatal("exit not confirmed")
		}
	}
}
func TestTweaksWheelSkipsHiddenDevRow(t *testing.T) {
	g := newResponsiveTestGame(t, 80, 24)
	g.overlay = ovTweaks
	g.tweaksSel = tweakBeltView
	g.updateOverlayWheel(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	if g.tweaksSel != tweakHighContrast {
		t.Fatalf("wheel selected hidden setting %d", g.tweaksSel)
	}
}
