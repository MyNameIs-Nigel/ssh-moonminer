package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
)

func (g *Game) renderCertify() string {
	st := &g.snap.State
	r := sim.NextRating(st, g.content)
	if r == nil {
		return g.progressionPanel("CERTIFICATION", []string{"CLASS C — ALL CHARTED GATES CERTIFIED", "NO CHARTED ROUTE BEYOND REDLINE EXPANSE", "[ESC] CLOSE"}, "certify:confirm")
	}
	lines := []string{"JUMP RATING — " + r.Name, g.content.SystemByID(r.CertifyIn).Name + " → " + g.content.SystemByID(r.Opens).Name, ""}
	missing := 0
	for _, q := range sim.RatingRequirements(st, g.content) {
		mark := "✓"
		if q.Have < q.Need {
			mark = "✗"
			missing++
		}
		if st.Settings.ASCIISafe {
			if q.Have < q.Need {
				mark = "[ ]"
			} else {
				mark = "[x]"
			}
		}
		lines = append(lines, fmt.Sprintf("%s %s  %d / %d", mark, q.Label, q.Have, q.Need))
	}
	if st.SystemID != r.CertifyIn {
		lines = append(lines, "CERTIFY AT AN "+g.content.SystemByID(r.CertifyIn).Name+" DOCK")
		missing++
	}
	lines = append(lines, fmt.Sprintf("%d REQUIREMENTS OUTSTANDING", missing), "[ENTER] CERTIFY", "[ESC] CLOSE")
	return g.progressionPanel("CERTIFICATION", lines, "certify:confirm")
}
func (g *Game) updateCertify(k string) []tea.Cmd {
	if k == "esc" || k == "q" {
		g.overlay = ovNone
	}
	if k == "enter" {
		snap, err := g.sess.CertifyRating(g.now)
		if err == nil {
			g.overlay = ovNone
		}
		return g.refreshSnap(snap, err)
	}
	return nil
}
func (g *Game) renderFerry() string {
	st := &g.snap.State
	lines := []string{"HAULER — DELIVER TO " + g.content.SystemByID(st.SystemID).Name, "Select a parked hull with ↑/↓.", ""}
	ids := g.ferryShips()
	for i, id := range ids {
		mark := "  "
		if i == g.ferrySel {
			mark = "> "
		}
		ship := st.Ships[id]
		price, err := sim.FerryPrice(st, g.content, id, st.SystemID)
		label := fmt.Sprintf("%d cr", price)
		if err != nil {
			label = err.Error()
		}
		lines = append(lines, mark+g.content.ShipByID(ship.ModelID).Name+" AT "+g.content.SystemByID(ship.SystemID).Name+" · "+label)
	}
	if len(ids) == 0 {
		lines = append(lines, "No remote hulls to ferry.")
	}
	lines = append(lines, "[ENTER] FERRY SELECTED", "[ESC] CLOSE")
	return g.progressionPanel("HAULER FERRY", lines, "ferry:confirm")
}
func (g *Game) ferryShips() []string {
	ids := []string{}
	for _, id := range sim.HangarIDs(&g.snap.State, g.content) {
		if ship := g.snap.State.Ships[id]; ship != nil && id != g.snap.State.ActiveShipID && ship.SystemID != g.snap.State.SystemID {
			ids = append(ids, id)
		}
	}
	return ids
}
func (g *Game) updateFerry(k string) []tea.Cmd {
	ids := g.ferryShips()
	if k == "esc" || k == "q" {
		g.overlay = ovNone
		return nil
	}
	if len(ids) == 0 {
		return nil
	}
	if k == "up" {
		g.ferrySel = (g.ferrySel + len(ids) - 1) % len(ids)
	}
	if k == "down" {
		g.ferrySel = (g.ferrySel + 1) % len(ids)
	}
	g.ferrySel = min(g.ferrySel, len(ids)-1)
	if k == "enter" {
		snap, err := g.sess.FerryShip(g.now, ids[g.ferrySel], g.snap.State.SystemID)
		if err == nil {
			g.overlay = ovNone
		}
		return g.refreshSnap(snap, err)
	}
	return nil
}
