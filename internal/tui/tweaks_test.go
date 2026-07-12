package tui

import (
	"strings"
	"testing"

	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/hitbox"
)

func TestTweaksOverlayRendersAllSettings(t *testing.T) {
	c, err := content.Load("")
	if err != nil {
		t.Fatal(err)
	}
	g := &Game{
		content:   c,
		width:     80,
		height:    24,
		overlay:   ovTweaks,
		tweaksSel: 0,
		hits:      hitbox.New(),
		snap:      sim.Snapshot{State: *sim.New(c, 1, 1000)},
	}
	out := g.renderTweaksOverlay()
	for _, want := range []string{
		"BELT VIEW", "PIRATE THREAT ASSIST", "Rewards unchanged.", "HIGH CONTRAST",
		"ASCII SAFE MODE", "REDUCED MOTION", "tiles", "1.0",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("tweaks overlay missing %q:\n%s", want, out)
		}
	}
}

func TestApplyTweakDeltaCyclesSettings(t *testing.T) {
	s := sim.Settings{
		BeltView:         sim.BeltViewTiles,
		PirateAggression: 1.0,
	}
	applyTweakDelta(&s, tweakBeltView, 1)
	if s.BeltView != sim.BeltViewOreScan {
		t.Fatalf("belt view: got %v want orescan", s.BeltView)
	}
	applyTweakDelta(&s, tweakPirateAgg, 1)
	if s.PirateAggression != 1.5 {
		t.Fatalf("pirate agg: got %v want 1.5", s.PirateAggression)
	}
	applyTweakDelta(&s, tweakASCII, 1)
	if !s.ASCIISafe {
		t.Fatal("expected ascii safe on")
	}
	applyTweakDelta(&s, tweakReducedMotion, 1)
	if !s.ReducedMotion {
		t.Fatal("expected reduced motion on")
	}
}

func TestSettingsPersistThroughEncodeDecode(t *testing.T) {
	c, err := content.Load("")
	if err != nil {
		t.Fatal(err)
	}
	st := sim.New(c, 99, 1000)
	st.Settings.BeltView = sim.BeltViewRadar
	st.Settings.PirateAggression = 2.0
	st.Settings.HighContrast = true
	st.Settings.ASCIISafe = true
	st.Settings.ReducedMotion = true

	b, err := st.Encode()
	if err != nil {
		t.Fatal(err)
	}
	out, err := sim.DecodeState(b, c)
	if err != nil {
		t.Fatal(err)
	}
	if out.Settings.BeltView != sim.BeltViewRadar {
		t.Fatalf("belt view: got %v", out.Settings.BeltView)
	}
	if out.Settings.PirateAggression != 2.0 {
		t.Fatalf("pirate agg: got %v", out.Settings.PirateAggression)
	}
	if !out.Settings.HighContrast || !out.Settings.ASCIISafe || !out.Settings.ReducedMotion {
		t.Fatalf("toggles not persisted: %+v", out.Settings)
	}
}

func TestTweaksOverlayNavigation(t *testing.T) {
	g := &Game{overlay: ovTweaks, tweaksSel: 0}
	g.updateTweaksOverlay("down")
	if g.tweaksSel != 1 {
		t.Fatalf("tweaksSel: got %d want 1", g.tweaksSel)
	}
	g.updateTweaksOverlay("t")
	if g.overlay != ovNone {
		t.Fatal("expected tweaks overlay closed with T")
	}
	g.overlay = ovTweaks
	g.updateTweaksOverlay("q")
	if g.overlay != ovNone {
		t.Fatal("expected tweaks overlay closed with Q")
	}
}
