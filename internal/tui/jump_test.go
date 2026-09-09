package tui

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
)

func TestJumpAndCertificationFrames(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {144, 48}} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			c := shipyardTestContent(t)
			s := sim.New(c, 1, 0)
			s.JumpClass = 0
			g := newShipyardGame(t, s)
			g.width = size[0]
			g.height = size[1]
			g.scr = scrChart
			g.jumpTarget = "eridani"
			view := ansi.Strip(g.renderJumpConfirm())
			for _, want := range []string{"FUEL", "HULL", "DRIFT", "ERIDANI"} {
				if !strings.Contains(view, want) {
					t.Fatalf("jump confirm missing %s: %s", want, view)
				}
			}
			g.jumpBeat = 3
			g.scr = scrJump
			view = ansi.Strip(g.renderJump())
			if !strings.Contains(view, "ALIGNMENT LOCKED") {
				t.Fatal(view)
			}
			g.jumpBeat = 0
			g.jumpResult = &sim.JumpResult{SystemID: "eridani", Drift: "HARD TRANSLATION", HullAfter: 42, FirstArrival: true}
			view = ansi.Strip(g.renderJump())
			for _, want := range []string{"HARD TRANSLATION", "ERIDANI", "Fight"} {
				if !strings.Contains(view, want) {
					t.Fatalf("arrival missing %s: %s", want, view)
				}
			}
			for _, line := range strings.Split(view, "\n") {
				if ansi.StringWidth(line) > g.contentWidth() {
					t.Fatal("jump frame overflow")
				}
			}
			g.keyJump("x")
			if g.scr != scrChart {
				t.Fatal("arrival key did not return to chart")
			}
			g.scr = scrChart
			g.keyChart("j")
			if g.overlay != ovCertify {
				t.Fatal("J did not open certification")
			}
			view = ansi.Strip(g.renderCertify())
			if !strings.Contains(view, "SABLE") || !strings.Contains(view, "PIRATES") {
				t.Fatal(view)
			}
		})
	}
}

func TestJumpSkipIgnoresStaleCountdownAndMouseCancel(t *testing.T) {
	g := newChartGame(t, 80, 24)
	g.snap.State.JumpClass = 0
	g.jumpTarget = "eridani"
	g.overlay = ovJump
	out := g.View().Content
	assertHitboxAtText(t, g, out, "CANCEL", "jump:cancel")
	g.updateJumpConfirm("esc")
	if g.overlay != ovNone {
		t.Fatal("cancel did not close")
	}
	g.scr = scrJump
	g.jumpBeat = 3
	g.jumpGeneration = 1
	g.jumpResult = &sim.JumpResult{SystemID: "eridani"}
	g.keyJump("?")
	if g.jumpBeat != 0 || g.scr != scrJump {
		t.Fatal("any key did not skip to arrival")
	}
	g.Update(jumpTickMsg{generation: 1})
	if g.jumpBeat != 0 {
		t.Fatal("stale timer advanced skipped cinematic")
	}
}

func TestReducedMotionBypassesJumpTimers(t *testing.T) {
	g := newChartGame(t, 80, 24)
	g.snap.State.Settings.ReducedMotion = true
	cmds := g.beginJump(&sim.JumpResult{SystemID: "eridani"})
	if len(cmds) != 0 || g.jumpBeat != 0 || g.scr != scrJump {
		t.Fatal("reduced motion scheduled countdown")
	}
	if !strings.Contains(ansi.Strip(g.renderJump()), "ARRIVAL") {
		t.Fatal("reduced motion did not show arrival")
	}
}

func TestJumpGoldenFrames(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {144, 48}} {
		for _, stage := range []string{"departure", "arrival", "drift"} {
			t.Run(fmt.Sprintf("%dx%d/%s", size[0], size[1], stage), func(t *testing.T) {
				g := newChartGame(t, size[0], size[1])
				g.scr = scrJump
				g.jumpTarget = "kepler"
				g.jumpResult = &sim.JumpResult{SystemID: "kepler", FuelAfter: 54.8, HullAfter: 93, FirstArrival: true}
				if stage == "departure" {
					g.jumpBeat = 3
				}
				if stage == "drift" {
					g.jumpResult.Drift = "HARD TRANSLATION"
					g.jumpResult.HullAfter = 74
				}
				view := ansi.Strip(g.View().Content)
				lines := strings.Split(view, "\n")
				if len(lines) != size[1] {
					t.Fatalf("frame height %d, want %d", len(lines), size[1])
				}
				for i, line := range lines {
					if ansi.StringWidth(line) > size[0] {
						t.Fatal("frame exceeds viewport")
					}
					lines[i] = strings.TrimRight(line, " ")
				}
				normalized := strings.TrimRight(strings.Join(lines, "\n"), "\n") + "\n"
				path := fmt.Sprintf("testdata/jump-%dx%d-%s.golden", size[0], size[1], stage)
				if os.Getenv("UPDATE_JUMP_GOLDENS") == "1" {
					if err := os.MkdirAll("testdata", 0755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, []byte(normalized), 0644); err != nil {
						t.Fatal(err)
					}
				}
				expected, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if string(expected) != normalized {
					t.Fatalf("jump frame differs from %s", path)
				}
			})
		}
	}
}

func TestRemoteHangarNamesLocationAndRejectsActivation(t *testing.T) {
	c := shipyardTestContent(t)
	s := sim.New(c, 1, 0)
	s.Credits = 100000
	if err := sim.AcquireShip(s, c, "cicada"); err != nil {
		t.Fatal(err)
	}
	s.Ships["cicada"].SystemID = "eridani"
	g := newShipyardGame(t, s)
	g.shipyardHangarSel = 1
	out := ansi.Strip(g.renderShipyard())
	if !strings.Contains(out, "AT ERIDANI") {
		t.Fatal("remote hull location hidden")
	}
	g.shipyardActivate()
	if g.snap.State.ActiveShipID != "skiff" || !strings.Contains(g.flash.text, "ERIDANI") {
		t.Fatal("remote activation did not explain its location")
	}
}
