package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/theme"
)

const (
	permitPanelW = 52
	permitPanelH = 10
)

type permitPurchase struct {
	worldIdx int
	name     string
	kind     string
	price    int
	system   bool
	id       string
}

// permitForWorld identifies the only two lock states that can be resolved by
// a purchase. Other route locks (ship, fuel capacity, and so on) must remain
// ordinary error feedback rather than presenting an offer that cannot work.
func (g *Game) permitForWorld(worldIdx int) (permitPurchase, bool) {
	st := g.snap.State
	if worldIdx < 0 || worldIdx >= len(g.content.Worlds) {
		return permitPurchase{}, false
	}
	w := g.content.Worlds[worldIdx]
	reason := sim.RouteLockReason(&st, g.content, worldIdx)
	if strings.HasPrefix(reason, "BUY NAV PERMIT ") {
		return permitPurchase{
			worldIdx: worldIdx,
			name:     w.Name,
			kind:     "NAVIGATION",
			price:    w.PermitFee,
			id:       w.ID,
		}, true
	}
	return permitPurchase{}, false
}

// openPermitPrompt takes the place of the former permanent P action and the
// always-visible permit button. Trying to depart is now the natural point to
// explain the gate and request confirmation.
func (g *Game) openPermitPrompt() bool {
	if _, ok := g.permitForWorld(g.worldSel); !ok {
		return false
	}
	g.pendingPermitWorld = g.worldSel
	g.overlay = ovPermit
	return true
}

func (g *Game) updatePermitOverlay(k string) []tea.Cmd {
	switch k {
	case "esc", "q", "n":
		g.overlay = ovNone
	case "enter", "y":
		return g.confirmPermitPurchase()
	}
	return nil
}

func (g *Game) confirmPermitPurchase() []tea.Cmd {
	purchase, ok := g.permitForWorld(g.pendingPermitWorld)
	if !ok {
		g.overlay = ovNone
		return nil
	}
	snap, err := g.sess.BuyDestinationPermit(g.now, purchase.id)
	if err == nil {
		g.overlay = ovNone
	}
	return g.refreshSnap(snap, err)
}

func (g *Game) renderPermitOverlay() string {
	purchase, ok := g.permitForWorld(g.pendingPermitWorld)
	if !ok {
		return ""
	}
	st := g.snap.State
	panelW, panelH := g.layoutOverlaySize(permitPanelW, permitPanelH)
	innerW := panelW - 4
	price := fmt.Sprintf("%s %d", theme.Glyph("credit", st.Settings.ASCIISafe), purchase.price)
	buy := theme.Button("ENTER", "BUY PERMIT", price, st.Credits >= purchase.price, theme.HueRed)
	cancel := theme.Button("ESC", "CANCEL", "", true, theme.HueRed)
	lines := []string{
		theme.Red.Bold(true).Render("ROUTE PERMIT REQUIRED"),
		"",
		theme.TxtStyle.Render("TARGET: " + purchase.name),
		theme.Red.Render(purchase.kind + " PERMIT: " + price),
		theme.TxtStyle.Render(fmt.Sprintf("BALANCE: %s %d", theme.Glyph("credit", st.Settings.ASCIISafe), st.Credits)),
		"",
		buy,
		cancel,
	}
	bodyLines := reflowOverlayLines(lines, innerW)
	for i, line := range bodyLines {
		plain := ansi.Strip(line)
		if strings.Contains(plain, "BUY PERMIT") {
			g.overlayHitLine(panelW, panelH, i, line, "permit", "buy")
		}
		if strings.Contains(plain, "CANCEL") {
			g.overlayHitLine(panelW, panelH, i, line, "permit", "cancel")
		}
	}
	return theme.Panel("PERMIT GATE", panelW, panelH, strings.Join(bodyLines, "\n"), theme.Accent(theme.HueRed))
}
