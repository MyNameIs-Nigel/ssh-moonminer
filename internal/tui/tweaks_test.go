package tui

import (
	"strings"
	"testing"

	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/hitbox"
)

func TestTweaksOverlayHidesPirateThreatAssistOutsideDevMode(t *testing.T) {
	c, err := content.Load("")
	if err != nil {
		t.Fatal(err)
	}
	production := &Game{
		content:   c,
		width:     80,
		height:    24,
		overlay:   ovTweaks,
		tweaksSel: 0,
		hits:      hitbox.New(),
		snap:      sim.Snapshot{State: *sim.New(c, 1, 1000)},
	}
	out := production.renderTweaksOverlay()
	for _, want := range []string{
		"BELT VIEW", "HIGH CONTRAST", "ASCII SAFE MODE", "REDUCED MOTION", "LONG TEXT", "orescan", "scroll",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("tweaks overlay missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "PIRATE THREAT ASSIST") || strings.Contains(out, "Pirate speed only.") {
		t.Fatalf("production tweaks exposed dev-only pirate assist:\n%s", out)
	}

	dev := *production
	dev.devMode = true
	dev.hits = hitbox.New()
	out = dev.renderTweaksOverlay()
	for _, want := range []string{"PIRATE THREAT ASSIST", "Pirate speed only.", "1.0"} {
		if !strings.Contains(out, want) {
			t.Fatalf("dev tweaks overlay missing %q:\n%s", want, out)
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
	applyTweakDelta(&s, tweakWrapLongText, 1)
	if !s.WrapLongText {
		t.Fatal("expected long text wrapping on")
	}
}

func TestProductionNormalizesPersistedPirateThreatAssist(t *testing.T) {
	settings := sim.Settings{PirateAggression: 0.5}
	if changed := normalizePirateThreatAssist(&settings, false); !changed {
		t.Fatal("production did not normalize a persisted dev-assist value")
	}
	if settings.PirateAggression != 1.0 {
		t.Fatalf("production pirate aggression = %v, want 1.0", settings.PirateAggression)
	}
	if changed := normalizePirateThreatAssist(&settings, false); changed {
		t.Fatal("standard production value should not be rewritten")
	}
	settings.PirateAggression = 0.5
	if changed := normalizePirateThreatAssist(&settings, true); changed || settings.PirateAggression != 0.5 {
		t.Fatalf("dev mode unexpectedly changed pirate assist: changed=%v value=%v", changed, settings.PirateAggression)
	}
}

func TestTweaksNavigationSkipsPirateThreatAssistOutsideDevMode(t *testing.T) {
	production := &Game{overlay: ovTweaks, tweaksSel: tweakBeltView}
	production.updateTweaksOverlay("down")
	if production.tweaksSel != tweakHighContrast {
		t.Fatalf("production selection = %d, want high contrast row %d", production.tweaksSel, tweakHighContrast)
	}

	dev := &Game{overlay: ovTweaks, tweaksSel: tweakBeltView, devMode: true}
	dev.updateTweaksOverlay("down")
	if dev.tweaksSel != tweakPirateAgg {
		t.Fatalf("dev selection = %d, want pirate assist row %d", dev.tweaksSel, tweakPirateAgg)
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
	st.Settings.WrapLongText = true

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
	if !out.Settings.HighContrast || !out.Settings.ASCIISafe || !out.Settings.ReducedMotion || !out.Settings.WrapLongText {
		t.Fatalf("toggles not persisted: %+v", out.Settings)
	}
}

func TestTweaksOverlayNavigation(t *testing.T) {
	g := &Game{overlay: ovTweaks, tweaksSel: 0}
	g.updateTweaksOverlay("down")
	if g.tweaksSel != tweakHighContrast {
		t.Fatalf("tweaksSel: got %d want high contrast row %d", g.tweaksSel, tweakHighContrast)
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
