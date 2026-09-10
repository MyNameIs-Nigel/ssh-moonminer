package tui

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/mynameis-nigel/ssh-moonminer/internal/game"
	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
	"github.com/mynameis-nigel/ssh-moonminer/internal/store"
	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/hitbox"
	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/theme"
)

// Find controls through their rendered label, not the renderer's coordinates.
func chartControl(t *testing.T, g *Game, label, id string) hitbox.Box {
	t.Helper()
	for y, line := range strings.Split(g.View().Content, "\n") {
		plain := ansi.Strip(line)
		if i := strings.Index(plain, label); i >= 0 {
			x := ansi.StringWidth(plain[:i])
			b, ok := g.hitAt(x, y)
			if !ok || b.ID != id {
				t.Fatalf("%q has wrong hit at %d,%d: %+v", label, x, y, b)
			}
			return b
		}
	}
	t.Fatalf("control %q missing", label)
	return hitbox.Box{}
}

func TestChartServiceRectanglesAndGauges(t *testing.T) {
	controls := []struct{ label, id string }{
		{"[F] REFUEL", "svc:refuel"}, {"[R] REPAIR", "svc:repair"},
		{"[C] SELL CARGO", "svc:sell"}, {"[S] SHIPYARD", "btn:shipyard"}, {"[L] SHIP'S LOG", "btn:log"},
	}
	for _, size := range [][2]int{{80, 24}, {108, 32}, {144, 48}, {180, 60}} {
		for _, ascii := range []bool{false, true} {
			t.Run(fmt.Sprintf("%dx%d/ascii=%v", size[0], size[1], ascii), func(t *testing.T) {
				g := newChartGame(t, size[0], size[1])
				g.snap.State.Settings.ASCIISafe = ascii
				out := g.View().Content
				assertPhysicalBounds(t, out, size[0], size[1])
				rows := strings.Split(out, "\n")
				var previous hitbox.Box
				var firstGauge string
				for i, c := range controls {
					b := chartControl(t, g, c.label, c.id)
					if b.W != 20 || b.H != 3 {
						t.Fatalf("%s dimensions %dx%d, want 20x3", c.id, b.W, b.H)
					}
					if b.X+b.W != g.contentWidth()-1 {
						t.Fatalf("%s not right aligned: %+v", c.id, b)
					}
					if i > 0 && (b.X != previous.X || b.Y != previous.Y+3) {
						t.Fatalf("buttons not stacked: %+v after %+v", b, previous)
					}
					for y := b.Y; y < b.Y+b.H; y++ {
						for x := b.X; x < b.X+b.W; x++ {
							got, ok := g.hitAt(x+g.contentX(), y+g.contentY())
							if !ok || got.ID != c.id {
								t.Fatalf("%s missing rectangle cell %d,%d", c.id, x, y)
							}
						}
						if _, ok := g.hitAt(b.X-1+g.contentX(), y+g.contentY()); ok {
							t.Fatal("button gutter is clickable")
						}
					}
					top := ansi.Strip(ansi.Cut(rows[b.Y+g.contentY()], b.X+g.contentX(), b.X+g.contentX()+20))
					want := "┌" + strings.Repeat("─", 18) + "┐"
					if ascii {
						want = "+" + strings.Repeat("-", 18) + "+"
					}
					if top != want {
						t.Fatalf("button border %q, want %q", top, want)
					}
					if i < 2 {
						leftW, _ := chartPaneWidths(g.contentWidth())
						start := g.contentX() + leftW + 1
						end := b.X + g.contentX() - 1
						gauge := ansi.Strip(ansi.Cut(rows[b.Y+g.contentY()], start, end))
						idx := strings.Index(gauge, "█")
						if idx < 0 {
							t.Fatal("gauge missing")
						}
						gauge = strings.Repeat(" ", ansi.StringWidth(gauge[:idx])) + gauge[idx:]
						if i == 0 {
							firstGauge = gauge
						} else if gauge != firstGauge {
							t.Fatalf("gauge spans differ: %q versus %q", firstGauge, gauge)
						}
						for y := b.Y + 1; y < b.Y+3; y++ {
							detail := ansi.Strip(ansi.Cut(rows[y+g.contentY()], start, end))
							if strings.Contains(detail, "█") {
								t.Fatal("gauge must occupy only one row")
							}
						}
						if _, ok := g.hitAt(end-1, b.Y+g.contentY()); ok {
							t.Fatal("gauge is clickable")
						}
					}
					previous = b
				}
			})
		}
	}
}

func TestChartProgressionLivesInPortFooter(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {108, 32}, {144, 48}, {180, 60}} {
		g := newChartGame(t, size[0], size[1])
		out := g.View().Content
		leftW, _ := chartPaneWidths(g.contentWidth())
		for _, label := range []string{"JUMP RATING", "[J] CERTIFY", "[H] HAULER FERRY", "ENTER REMOTE: JUMP"} {
			found := 0
			for y, row := range strings.Split(out, "\n") {
				plain := ansi.Strip(row)
				i := strings.Index(plain, label)
				if i < 0 {
					continue
				}
				found++
				if ansi.StringWidth(plain[:i]) < g.contentX()+leftW {
					t.Fatalf("%s remains in chart", label)
				}
				if y < g.contentY()+g.contentHeight()-5 || y > g.contentY()+g.contentHeight()-4 {
					t.Fatalf("%s not in footer row: %d", label, y)
				}
			}
			if found != 1 {
				t.Fatalf("%s count=%d", label, found)
			}
		}
		if strings.Contains(ansi.Strip(out), "Bigger rocks pay") {
			t.Fatal("old tip remains")
		}
		chartControl(t, g, "[J] CERTIFY", "svc:certify")
		chartControl(t, g, "[H] HAULER FERRY", "svc:ferry")
	}
}

func TestChartServiceDetails(t *testing.T) {
	for _, poor := range []bool{false, true} {
		g := newChartGame(t, 80, 24)
		st := &g.snap.State
		st.Credits = 0
		st.Ships[st.ActiveShipID].BaseFuel = 0
		st.Ships[st.ActiveShipID].Hull = sim.MaxHull(st, g.content) / 2
		if !poor {
			st.Credits = 100000
			st.CargoUnits = 3
			st.CargoValue = 1234567
			st.BountyVouchers = 7654321
		}
		out := ansi.Strip(g.renderChart())
		for _, want := range []string{fmt.Sprintf("0/%.0f", sim.TankSize(st, g.content)), fmt.Sprintf("%d/%d", st.Ships[st.ActiveShipID].Hull, sim.MaxHull(st, g.content)), "[C] SELL CARGO"} {
			if !strings.Contains(out, want) {
				t.Fatalf("missing %q:\n%s", want, out)
			}
		}
		if poor {
			if !strings.Contains(out, "[I] INSURANCE") {
				t.Fatalf("insurance missing:\n%s", out)
			}
			chartControl(t, g, "[I] INSURANCE", "svc:insurance")
		} else {
			for _, want := range []string{"1234567", "7654321"} {
				if !strings.Contains(out, want) {
					t.Fatalf("sale detail %s missing", want)
				}
			}
			if strings.Contains(out, "[I] INSURANCE") {
				t.Fatal("insurance shown when ineligible")
			}
		}
	}
}

func TestChartNavigationClicksAndKeys(t *testing.T) {
	for _, tc := range []struct {
		key, label, id string
		screen         screen
		overlay        overlay
	}{
		{"s", "[S] SHIPYARD", "btn:shipyard", scrShipyard, ovNone},
		{"l", "[L] SHIP'S LOG", "btn:log", scrLog, ovNone},
		{"j", "[J] CERTIFY", "svc:certify", scrChart, ovCertify},
		{"h", "[H] HAULER FERRY", "svc:ferry", scrChart, ovFerry},
	} {
		for _, mouse := range []bool{false, true} {
			g := newChartGame(t, 108, 32)
			if mouse {
				b := chartControl(t, g, tc.label, tc.id)
				g.updateClick(tea.MouseClickMsg{X: b.X, Y: b.Y})
			} else {
				g.keyChart(tc.key)
			}
			if g.scr != tc.screen || g.overlay != tc.overlay {
				t.Fatalf("%s mouse=%v: wrong navigation", tc.key, mouse)
			}
		}
	}
}

func TestChartOfflineMouseActions(t *testing.T) {
	g := newChartGame(t, 80, 24)
	g.snap.State.WorldIdx = 1
	for _, tc := range []struct{ label, id string }{{"REFUEL", "svc:refuel"}, {"REPAIR", "svc:repair"}, {"SELL CARGO", "svc:sell"}, {"SHIPYARD", "btn:shipyard"}, {"[J] CERTIFY", "svc:certify"}, {"[H] HAULER FERRY", "svc:ferry"}} {
		b := chartControl(t, g, tc.label, tc.id)
		g.flash.text = ""
		g.updateClick(tea.MouseClickMsg{X: b.X, Y: b.Y})
		if g.scr != scrChart || g.overlay != ovNone || g.flash.text == "" {
			t.Fatalf("offline %s was not rejected", tc.id)
		}
	}
	b := chartControl(t, g, "RETURN TO BELT", "btn:return-belt")
	g.updateClick(tea.MouseClickMsg{X: b.X, Y: b.Y})
	if g.scr != scrBelt || g.snap.State.IsDocked() {
		t.Fatal("return should restore belt screen without docking")
	}
}

// A real save actor proves both input paths still purchase services and settle
// cargo plus vouchers, rather than merely finding a correctly named hitbox.
func TestChartServicePurchasesByMouseAndKey(t *testing.T) {
	for _, mouse := range []bool{false, true} {
		t.Run(fmt.Sprintf("mouse=%v", mouse), func(t *testing.T) {
			g := newChartGame(t, 108, 32)
			g.now = 1000
			st := &g.snap.State
			st.Credits = 100000
			st.Ships[st.ActiveShipID].BaseFuel = 0
			st.Ships[st.ActiveShipID].Hull = sim.MaxHull(st, g.content) / 2
			st.CargoUnits = 3
			st.CargoValue = 120
			st.BountyVouchers = 80
			ctx := context.Background()
			db, err := store.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			g.id.Fingerprint = "chart-services"
			if err = db.TouchAccount(ctx, g.id.Fingerprint, "test-key", g.now); err != nil {
				t.Fatal(err)
			}
			_, _, err = db.LoadOrCreateSave(ctx, g.id.Fingerprint, g.id.Slot, g.now, func() ([]byte, int, error) { b, e := st.Encode(); return b, st.Version, e })
			if err != nil {
				t.Fatal(err)
			}
			mgr := game.NewManager(db, g.content, slog.Default(), time.Hour, game.PolicyTakeover)
			attached, err := mgr.Attach(ctx, g.id, "test-key", g.now, nil)
			if err != nil {
				t.Fatal(err)
			}
			g.sess = attached.Session
			defer g.sess.Detach()
			expected := st.Credits - sim.RefuelCost(st, g.content) - sim.RepairCost(st, g.content) + 200
			for _, tc := range []struct{ key, label, id string }{{"f", "REFUEL", "svc:refuel"}, {"r", "REPAIR", "svc:repair"}, {"c", "SELL CARGO", "svc:sell"}} {
				if mouse {
					b := chartControl(t, g, tc.label, tc.id)
					g.updateClick(tea.MouseClickMsg{X: b.X + 1, Y: b.Y + 2})
				} else {
					g.keyChart(tc.key)
				}
			}
			st = &g.snap.State
			if st.Credits != expected || sim.FuelAmount(st, g.content) != sim.TankSize(st, g.content) || sim.HullPct(st, g.content) != 100 || st.CargoValue != 0 || st.BountyVouchers != 0 {
				t.Fatalf("services did not settle correctly: credits=%d expected=%d fuel=%v hull=%v cargo=%d vouchers=%d", st.Credits, expected, sim.FuelAmount(st, g.content), sim.HullPct(st, g.content), st.CargoValue, st.BountyVouchers)
			}
		})
	}
}

// Inspect cell positions in the rendered service pane, including the narrow
// layout where complete prices need the otherwise-unused third band row.
func TestChartServicePointsAndCreditAlignment(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {108, 32}, {144, 48}, {180, 60}} {
		for _, ascii := range []bool{false, true} {
			for _, expensive := range []bool{false, true} {
				t.Run(fmt.Sprintf("%dx%d/ascii=%v/expensive=%v", size[0], size[1], ascii, expensive), func(t *testing.T) {
					g := newChartGame(t, size[0], size[1])
					st := &g.snap.State
					st.Settings.ASCIISafe = ascii
					st.Ships[st.ActiveShipID].BaseFuel = 25
					st.Ships[st.ActiveShipID].Hull = sim.MaxHull(st, g.content) / 2
					st.CargoValue = 1234567
					st.BountyVouchers = 7654321
					if expensive {
						g.content.Port.RefuelPerPoint = 12345
					}
					out := g.View().Content
					assertPhysicalBounds(t, out, size[0], size[1])
					rows := strings.Split(out, "\n")
					leftW, _ := chartPaneWidths(g.contentWidth())
					start := g.contentX() + leftW + 1
					b := chartControl(t, g, "[F] REFUEL", "svc:refuel")
					end := g.contentX() + b.X - 1
					detail := func(row int) string { return ansi.Strip(ansi.Cut(rows[g.contentY()+b.Y+row], start, end)) }
					fuelLabel := theme.Glyph("fuel", ascii)
					barOffset := max(ansi.StringWidth(fuelLabel), 4) + 1
					for _, tc := range []struct {
						row    int
						points string
						cost   int
					}{
						{0, fmt.Sprintf("%.0f/%.0f", sim.FuelAmount(st, g.content), sim.TankSize(st, g.content)), sim.RefuelCost(st, g.content)},
						{3, fmt.Sprintf("%d/%d", st.Ships[st.ActiveShipID].Hull, sim.MaxHull(st, g.content)), sim.RepairCost(st, g.content)},
					} {
						gauge := detail(tc.row)
						idx := strings.Index(gauge, "█")
						if idx < 0 || ansi.StringWidth(gauge[:idx]) != barOffset {
							t.Fatalf("wrong gauge start: %q", gauge)
						}
						pointsRow := detail(tc.row + 1)
						if !strings.HasPrefix(pointsRow, strings.Repeat(" ", barOffset)+tc.points) {
							t.Fatalf("points not below gauge: %q", pointsRow)
						}
						if strings.Contains(pointsRow, "%") {
							t.Fatalf("redundant percentage: %q", pointsRow)
						}
						price := fmt.Sprintf("%s %d", theme.Glyph("credit", ascii), tc.cost)
						row := tc.row + 1
						if barOffset+ansi.StringWidth(tc.points)+1+ansi.StringWidth(price) > end-start {
							row++
						}
						if !strings.HasSuffix(detail(row), price) {
							t.Fatalf("price must be complete and right aligned: %q want %q", detail(row), price)
						}
					}
					for _, tc := range []struct {
						row   int
						label string
						value int
					}{{7, "SALE", 1234567}, {8, "BOUNTY", 7654321}} {
						got := detail(tc.row)
						price := fmt.Sprintf("%s %d", theme.Glyph("credit", ascii), tc.value)
						if !strings.HasPrefix(got, tc.label) || !strings.HasSuffix(got, price) {
							t.Fatalf("sale alignment incorrect: %q", got)
						}
					}
				})
			}
		}
	}
}
