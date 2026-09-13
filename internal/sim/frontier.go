package sim

import (
	"sort"

	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
)

type FrontierRecord struct {
	Visited           bool           `json:"visited"`
	FirstArrivalAt    int64          `json:"first_arrival_at"`
	RunsSurvived      int            `json:"runs_survived"`
	RunsByDestination map[string]int `json:"runs_by_destination,omitempty"`
	CargoValueSold    int            `json:"cargo_value_sold"`
	PiratesDestroyed  int            `json:"pirates_destroyed"`
	LegendariesMined  int            `json:"legendaries_mined"`
	ShipsLost         int            `json:"ships_lost"`
}

func frontierRecord(s *State, id string) *FrontierRecord {
	if s.Frontier == nil {
		s.Frontier = map[string]*FrontierRecord{}
	}
	if s.Frontier[id] == nil {
		s.Frontier[id] = &FrontierRecord{}
	}
	r := s.Frontier[id]
	if r.RunsByDestination == nil {
		r.RunsByDestination = map[string]int{}
	}
	return r
}
func visitSystem(s *State, id string, now int64) bool {
	r := frontierRecord(s, id)
	first := !r.Visited
	if first {
		r.Visited = true
		r.FirstArrivalAt = now
	}
	return first
}
func NextRating(s *State, c *content.Content) *content.Rating {
	for i := range c.Ratings {
		if c.Ratings[i].Class == s.JumpClass+1 {
			return &c.Ratings[i]
		}
	}
	return nil
}

// Qualification is a presentation-neutral checklist row, reusable by future quests.
type Qualification struct {
	Label      string
	Have, Need int
}

func RatingRequirements(s *State, c *content.Content) []Qualification {
	r := NextRating(s, c)
	if r == nil {
		return nil
	}
	f := s.Frontier[r.CertifyIn]
	if f == nil {
		f = &FrontierRecord{}
	}
	rows := []Qualification{{"CREDITS", s.Credits, r.Price}}
	if r.ReqCargoSoldValue > 0 {
		rows = append(rows, Qualification{"CARGO SOLD — " + c.SystemByID(r.CertifyIn).Name, f.CargoValueSold, r.ReqCargoSoldValue})
	}
	if r.ReqRunsCount > 0 {
		w := c.WorldByID(r.ReqRunsDestination)
		rows = append(rows, Qualification{"RUNS SURVIVED — " + w.Name + " " + w.Sub, f.RunsByDestination[r.ReqRunsDestination], r.ReqRunsCount})
	}
	if r.ReqPiratesDestroyed > 0 {
		rows = append(rows, Qualification{"PIRATES DESTROYED", f.PiratesDestroyed, r.ReqPiratesDestroyed})
	}
	if r.ReqLegendariesMined > 0 {
		rows = append(rows, Qualification{"LEGENDARIES MINED", f.LegendariesMined, r.ReqLegendariesMined})
	}
	return rows
}
func CertifyRating(s *State, c *content.Content) error {
	if !s.IsDocked() {
		return ErrInBelt
	}
	r := NextRating(s, c)
	if r == nil {
		return ErrMaxUpgrade
	}
	if s.SystemID != r.CertifyIn {
		return ErrRouteLocked
	}
	for _, q := range RatingRequirements(s, c) {
		if q.Have < q.Need {
			return ErrNotEligible
		}
	}
	s.Credits -= r.Price
	s.Stats.CreditsSpent += r.Price
	s.JumpClass = r.Class
	return nil
}
func recordFrontierOutcome(s *State, c *content.Content, rec RunRecord, kind OutcomeKind, extracted float64) {
	f := frontierRecord(s, s.SystemID)
	if kind == OutcomeShipLost {
		f.ShipsLost++
	} else {
		f.RunsSurvived++
		f.RunsByDestination[rec.DestinationID]++
	}
	if rec.Tier == 3 && extracted > 0 {
		f.LegendariesMined++
	}
}

// Tribute proportionally removes provenance, using sorted IDs to make rounding
// deterministic. Unknown legacy cargo participates without inventing an origin.
func reduceCargoOrigins(s *State, lost int) {
	remainingValue, remainingLoss := s.CargoValue, lost
	ids := []string{}
	for id := range s.CargoOrigins {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		v := s.CargoOrigins[id]
		cut := proportionalCargoLoss(remainingLoss, v, remainingValue)
		s.CargoOrigins[id] -= cut
		remainingValue -= v
		remainingLoss -= cut
	}
}
