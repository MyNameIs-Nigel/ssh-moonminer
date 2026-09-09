package sim

import (
	"encoding/json"
	"math"

	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
)

// Retired fields are read only at the serialization boundary; gameplay cannot
// buy, sell, or accidentally equip the old route key.
func retireLegacyDrives(b []byte) ([]byte, int, bool, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(b, &root); err != nil {
		return nil, 0, false, err
	}
	var version int
	if raw, ok := root["version"]; ok {
		if err := json.Unmarshal(raw, &version); err != nil {
			return nil, 0, false, err
		}
	}
	if version >= 8 {
		return b, 0, false, nil
	}
	var permits map[string]bool
	if raw, ok := root["system_permits"]; ok {
		if err := json.Unmarshal(raw, &permits); err != nil {
			return nil, 0, false, err
		}
	}
	count := 0
	remove := func(raw json.RawMessage) bool {
		var d SlotDevice
		if json.Unmarshal(raw, &d) == nil && d.ItemID == "jump_drive" {
			count++
			return true
		}
		return false
	}
	filter := func(raw json.RawMessage) json.RawMessage {
		var list []json.RawMessage
		if json.Unmarshal(raw, &list) != nil {
			return raw
		}
		for i, d := range list {
			if remove(d) {
				list[i] = json.RawMessage("null")
			}
		}
		v, _ := json.Marshal(list)
		return v
	}
	var ships map[string]map[string]json.RawMessage
	if raw, ok := root["ships"]; ok {
		if err := json.Unmarshal(raw, &ships); err != nil {
			return nil, 0, false, err
		}
	}
	for _, ship := range ships {
		remove(ship["jump_drive"])
		delete(ship, "jump_drive")
		if remove(ship["internal"]) {
			delete(ship, "internal")
		}
		for _, slot := range []string{"utility", "weapon"} {
			if raw, ok := ship[slot]; ok {
				ship[slot] = filter(raw)
			}
		}
	}
	if ships != nil {
		root["ships"], _ = json.Marshal(ships)
	}
	var inv []json.RawMessage
	if raw, ok := root["inventory"]; ok {
		if err := json.Unmarshal(raw, &inv); err != nil {
			return nil, 0, false, err
		}
	}
	kept := []json.RawMessage{}
	for _, d := range inv {
		if !remove(d) {
			kept = append(kept, d)
		}
	}
	if len(kept) == 0 {
		delete(root, "inventory")
	} else if len(inv) > 0 {
		root["inventory"], _ = json.Marshal(kept)
	}
	delete(root, "system_permits")
	out, err := json.Marshal(root)
	return out, count, permits["eridani"], err
}
func migrateFrontier(s *State, c *content.Content, drives int, permit bool) {
	s.JumpClass = -1
	if drives > 0 || permit {
		s.JumpClass = 0
	}
	if drives > 1 {
		s.Credits += (drives - 1) * int(math.Round(float64(c.Slots.LegacyDriveBasePrice)*c.Slots.SellValuePct))
	}
	for _, ship := range s.Ships {
		ship.SystemID = s.SystemID
	}
	visitSystem(s, "sol", s.Stats.FirstSeen)
	visitSystem(s, s.SystemID, s.Stats.LastSeen)
	// Legacy logs cannot recover cargo provenance across a sale. Do not invent
	// sold value from recovered ore; only unambiguous run counters are seeded.
	for _, rec := range s.RunLog {
		// Dock-sale records name a system, proving a visit even when the older
		// mining runs have fallen out of the bounded log. They cannot prove origin.
		for _, sys := range c.Systems {
			if rec.World == sys.Name {
				visitSystem(s, sys.ID, rec.When)
			}
		}
		var w *content.World
		for i := range c.Worlds {
			if c.Worlds[i].Name == rec.World {
				w = &c.Worlds[i]
				break
			}
		}
		if w == nil {
			continue
		}
		visitSystem(s, w.SystemID, rec.When)
		r := frontierRecord(s, w.SystemID)
		switch OutcomeKind(rec.Outcome) {
		case OutcomeShipLost:
			r.ShipsLost++
		case OutcomeDeparted, OutcomeBailed, OutcomeTributePaid, OutcomeEscapedUnderFire:
			r.RunsSurvived++
			r.RunsByDestination[w.ID]++
		}
		if rec.Tier == 3 && rec.CargoValueRecovered > 0 {
			r.LegendariesMined++
		}
		if rec.PirateDestroyed != "" {
			r.PiratesDestroyed++
		}
	}
}
