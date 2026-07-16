package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
)

func TestMiningCargoFullKeepsActualAsteroidResourceMeter(t *testing.T) {
	g := newChartGame(t, 100, 24)
	st := &g.snap.State
	if err := sim.Depart(st, g.content, 1); err != nil {
		t.Fatal(err)
	}
	ast := st.Belt[0]
	ast.Volume = 2400
	ast.Scanned = true
	st.Belt[0] = ast
	st.Run = &sim.ActiveRun{
		AsteroidID: ast.ID,
		Phase:      sim.PhaseMining,
		// The skiff's 1400-unit hold is full, but 1000 units (42%) are
		// still inside the asteroid.
		ExtractedUnits: 1400,
		HeldUnits:      1400,
		MinedUnits:     1400,
	}

	out := ansi.Strip(g.renderDrilling(st, st.Run, &ast, ast.Name, ast.Tier, ast.Value))
	if !strings.Contains(out, "42%") {
		t.Fatalf("resource meter hid the remaining asteroid ore:\n%s", out)
	}
	if !strings.Contains(out, "HOLD FULL") || strings.Contains(out, "DEPART") {
		t.Fatalf("full-hold action should require bail, not departure:\n%s", out)
	}
}

func TestMiningChromeAlwaysShowsLiveCargo(t *testing.T) {
	g := newChartGame(t, 80, 24)
	g.snap.State.CargoUnits = 123
	g.snap.State.Run = &sim.ActiveRun{HeldUnits: 77, MinedUnits: 77}
	out := ansi.Strip(g.renderChrome())
	if !strings.Contains(out, "CARGO 200/1400") {
		t.Fatalf("shared nav did not show combined cargo: %q", out)
	}
}

func TestPressurePointAsteroidRendersAllStates(t *testing.T) {
	g := newChartGame(t, 100, 24)
	run := &sim.ActiveRun{
		AsteroidSprite: 1,
		SkillCheck:     &sim.SkillCheck{PressurePoint: 0},
		PressurePoints: []sim.PressurePoint{
			{Position: 0, Status: sim.PressurePointActive},
			{Position: 1, Status: sim.PressurePointHit},
			{Position: 2, Status: sim.PressurePointMissed},
		},
	}
	out, active := g.renderAsteroidField(&g.snap.State, run, 19)
	plain := ansi.Strip(out)
	if !active || !strings.Contains(plain, "*") || !strings.Contains(plain, "+") || !strings.Contains(plain, "x") {
		t.Fatalf("asteroid did not render active/hit/missed pressure points:\n%s", plain)
	}
}

func TestMiningUsesRightCockpitViewportForScannerAndAsteroid(t *testing.T) {
	g := newChartGame(t, 100, 24)
	st := &g.snap.State
	if err := sim.Depart(st, g.content, 1); err != nil {
		t.Fatal(err)
	}
	ast := st.Belt[0]
	ast.Scanned = true
	st.Belt[0] = ast
	st.Run = &sim.ActiveRun{
		AsteroidID:     ast.ID,
		Phase:          sim.PhaseMining,
		AsteroidSprite: 0,
		SkillCheck:     &sim.SkillCheck{Window: 3},
	}

	plain := ansi.Strip(g.renderDrilling(st, st.Run, &ast, ast.Name, ast.Tier, ast.Value))
	lines := strings.Split(plain, "\n")
	leftW, _ := g.miningColumnWidths()
	var scannerLine, artLine, countdownLine string
	for _, line := range lines {
		switch {
		case strings.Contains(line, "PIRATE ETA"):
			scannerLine = line
		case strings.Contains(line, ".-~~~~"):
			artLine = line
		case strings.Contains(line, "PRESSURE POINT ACTIVE"):
			countdownLine = line
		}
	}
	if strings.Index(scannerLine, "PIRATE ETA") <= leftW || strings.Index(artLine, ".-~~~~") <= leftW {
		t.Fatalf("scanner/asteroid escaped the right cockpit viewport:\n%s", plain)
	}
	if idx := strings.Index(countdownLine, "PRESSURE POINT ACTIVE"); idx < 0 || idx >= leftW {
		t.Fatalf("pressure-point countdown was not kept in the left status column:\n%s", plain)
	}
}
