package sim_test

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
)

func TestFrontierContentAndDepartureBoundary(t *testing.T) {
	c := testContent(t)
	if len(c.Systems) != 4 || len(c.Ships) != 7 {
		t.Fatalf("beta requires four systems and seven hulls, got %d/%d", len(c.Systems), len(c.Ships))
	}
	s := sim.New(c, 1, 10)
	before := s.Clone()
	if err := sim.Depart(s, c, 4); err != sim.ErrRouteLocked {
		t.Fatalf("cross-system Depart = %v", err)
	}
	if !reflect.DeepEqual(s, before) {
		t.Fatal("rejected departure mutated save")
	}
	if !strings.Contains(sim.RouteLockReason(s, c, 4), "JUMP TO ERIDANI DRIFT FIRST") {
		t.Fatal("departure must explain separate jump")
	}
}

func TestJumpGateRatingsCostsAndReplay(t *testing.T) {
	c := testContent(t)
	for _, gate := range c.Gates {
		for _, reverse := range []bool{false, true} {
			for class := -1; class <= 2; class++ {
				from, to := gate.From, gate.To
				if reverse {
					from, to = to, from
				}
				s := sim.New(c, 37, 10)
				s.SystemID = from
				sim.ActiveShip(s).SystemID = from
				s.JumpClass = class
				s.Hull = 1
				sim.ActiveShip(s).Hull = 1
				before := s.Clone()
				preview, err := sim.PreviewJump(s, c, to)
				if class < gate.RequiredRating {
					if err == nil {
						t.Fatalf("gate %s -> %s admitted class %d", from, to, class)
					}
					if _, err = sim.Jump(s, c, to, 20); err == nil {
						t.Fatal("jump admitted unrated pilot")
					}
					if !reflect.DeepEqual(s, before) {
						t.Fatal("rejected jump mutated state")
					}
					continue
				}
				if err != nil {
					t.Fatal(err)
				}
				if preview.FuelCost >= 100 {
					t.Fatalf("free skiff cannot cross %s", to)
				}
				a, b := s.Clone(), s.Clone()
				ra, err := sim.Jump(a, c, to, 20)
				if err != nil {
					t.Fatal(err)
				}
				rb, err := sim.Jump(b, c, to, 20)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(a, b) || !reflect.DeepEqual(ra, rb) {
					t.Fatal("jump replay differs")
				}
				if a.Hull < 1 || !a.IsDocked() || sim.ActiveShip(a).SystemID != a.SystemID {
					t.Fatal("jump broke hull/location invariant")
				}
				if ra.FuelAfter != a.Fuel || ra.HullAfter != a.Hull {
					t.Fatal("arrival result differs from state")
				}
				if !reflect.DeepEqual(s, before) {
					t.Fatal("preview mutated state")
				}
			}
		}
	}
}

func TestCertificationFrontierAndAtomicRejection(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 0)
	if s.JumpClass != -1 {
		t.Fatal("new pilot already certified")
	}
	for _, r := range c.Ratings {
		s.SystemID = r.CertifyIn
		sim.ActiveShip(s).SystemID = s.SystemID
		s.Credits = r.Price
		before := s.Clone()
		if err := sim.CertifyRating(s, c); err == nil {
			t.Fatal("credits alone qualified")
		}
		if !reflect.DeepEqual(s, before) {
			t.Fatal("rejected certification mutated save")
		}
		s.Frontier[r.CertifyIn] = &sim.FrontierRecord{Visited: true, CargoValueSold: r.ReqCargoSoldValue, RunsByDestination: map[string]int{r.ReqRunsDestination: r.ReqRunsCount}, PiratesDestroyed: r.ReqPiratesDestroyed, LegendariesMined: r.ReqLegendariesMined}
		if err := sim.CertifyRating(s, c); err != nil {
			t.Fatal(err)
		}
		if s.JumpClass != r.Class || s.Credits != 0 {
			t.Fatal("certification failed accounting")
		}
	}
}

func TestJumpMassTankBoundaryAndFerry(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 0)
	s.Credits = 1000000
	s.JumpClass = 2
	if err := sim.AcquireShip(s, c, "mule"); err != nil {
		t.Fatal(err)
	}
	gate := c.Gates[2]
	for i := 0; i < 2; i++ {
		if err := sim.InstallSlotDevice(s, c, "mule", sim.SlotUtility, i, sim.ItemFuelTank, 5); err != nil {
			t.Fatal(err)
		}
	}
	if sim.JumpFuel(s, c, "mule", gate) <= sim.ShipFuelCapacity(s, c, "mule") {
		t.Fatal("two-tank Mule must not cross Redline")
	}
	if err := sim.InstallSlotDevice(s, c, "mule", sim.SlotUtility, 2, sim.ItemFuelTank, 5); err != nil {
		t.Fatal(err)
	}
	if sim.JumpFuel(s, c, "mule", gate) >= sim.ShipFuelCapacity(s, c, "mule") {
		t.Fatal("three-tank Mule must cross Redline")
	}
	before := s.Clone()
	if err := sim.FerryShip(s, c, "mule", "redline"); err == nil {
		t.Fatal("ferry admitted unvisited destination")
	}
	if !reflect.DeepEqual(s, before) {
		t.Fatal("rejected ferry mutated state")
	}
	s.Frontier["redline"] = &sim.FrontierRecord{Visited: true}
	price, err := sim.FerryPrice(s, c, "mule", "redline")
	if err != nil {
		t.Fatal(err)
	}
	cost := 0.
	for _, g := range c.Gates {
		cost += sim.JumpFuel(s, c, "mule", g) * float64(c.Port.RefuelPerPoint)
	}
	if price <= int(math.Ceil(cost)) {
		t.Fatal("ferry no dearer than flying")
	}
	credits := s.Credits
	fuel := sim.ShipFuelAmount(s, c, "mule")
	if err := sim.FerryShip(s, c, "mule", "redline"); err != nil {
		t.Fatal(err)
	}
	if s.Ships["mule"].SystemID != "redline" || s.Credits != credits-price || sim.ShipFuelAmount(s, c, "mule") != fuel {
		t.Fatal("ferry accounting")
	}
	before = s.Clone()
	if err := sim.SwitchActiveShip(s, c, "mule"); err == nil {
		t.Fatal("remote hull activated")
	}
	if !reflect.DeepEqual(s, before) {
		t.Fatal("remote switch mutated state")
	}
}

func TestV7RatingMigrationAndClone(t *testing.T) {
	c := testContent(t)
	for _, tc := range []struct {
		name, extras   string
		credits, class int
	}{
		{"none", "", 100, -1}, {"permit", `,"system_permits":{"eridani":true}`, 100, 0},
		{"one", `,"inventory":[{"item_id":"jump_drive","grade":5}]`, 100, 0},
		{"two", `,"inventory":[{"item_id":"jump_drive","grade":0},{"item_id":"jump_drive","grade":3}]`, 23850, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			blob := []byte(`{"version":7,"seed":8,"world_idx":-1,"system_id":"eridani","credits":100,"active_ship_id":"skiff","ships":{"skiff":{"model_id":"skiff","hull":61,"base_fuel":42}}` + tc.extras + `}`)
			s, err := sim.DecodeState(blob, c)
			if err != nil {
				t.Fatal(err)
			}
			if s.Version != 8 || s.JumpClass != tc.class || s.Credits != tc.credits || s.Ships["skiff"].SystemID != "eridani" {
				t.Fatalf("bad migration: %+v", s)
			}
			b, err := s.Encode()
			if err != nil {
				t.Fatal(err)
			}
			again, err := sim.DecodeState(b, c)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(s, again) {
				t.Fatal("migration not idempotent")
			}
			clone := s.Clone()
			clone.Frontier["eridani"].RunsByDestination["eris_veil"] = 9
			if s.Frontier["eridani"].RunsByDestination["eris_veil"] != 0 {
				t.Fatal("frontier clone aliases")
			}
			var raw map[string]any
			if err := json.Unmarshal(b, &raw); err != nil {
				t.Fatal(err)
			}
			if _, ok := raw["system_permits"]; ok {
				t.Fatal("retired permits persisted")
			}
		})
	}
}

func TestJumpDriftOutcomesHullFloorAndPreviewBoundary(t *testing.T) {
	c := testContent(t)
	for _, outcome := range []string{"HARD TRANSLATION", "FUEL BLOOM", "HOT ARRIVAL", "MISALIGNMENT"} {
		t.Run(outcome, func(t *testing.T) {
			c.Jump.HardTranslationWeight = 0
			c.Jump.FuelBloomWeight = 0
			c.Jump.HotArrivalWeight = 0
			c.Gates[2].MisalignmentWeight = 0
			c.Gates[2].DriftChance = 1
			switch outcome {
			case "HARD TRANSLATION":
				c.Jump.HardTranslationWeight = 1
			case "FUEL BLOOM":
				c.Jump.FuelBloomWeight = 1
			case "HOT ARRIVAL":
				c.Jump.HotArrivalWeight = 1
			case "MISALIGNMENT":
				c.Gates[2].MisalignmentWeight = 1
			}
			s := sim.New(c, 12, 0)
			s.SystemID = "kepler"
			sim.ActiveShip(s).SystemID = "kepler"
			s.JumpClass = 2
			s.CrossedGates = map[string]bool{"kepler:redline": true}
			s.Hull = 1
			sim.ActiveShip(s).Hull = 1
			r, err := sim.Jump(s, c, "redline", 10)
			if err != nil {
				t.Fatal(err)
			}
			if r.Drift != outcome || s.Hull != 1 {
				t.Fatalf("outcome=%+v hull=%d", r, s.Hull)
			}
			if outcome == "MISALIGNMENT" && s.SystemID != "kepler" {
				t.Fatal("misalignment moved pilot forward")
			}
			if outcome == "HOT ARRIVAL" && !s.HotArrivals["redline"] {
				t.Fatal("hot arrival not persisted")
			}
		})
	}
	c = testContent(t)
	c.Gates[0].DriftChance = 0
	for _, hull := range []int{23, 24, 25} {
		s := sim.New(c, 1, 0)
		s.JumpClass = 0
		s.Hull = hull
		sim.ActiveShip(s).Hull = hull
		p, err := sim.PreviewJump(s, c, "eridani")
		if err != nil {
			t.Fatal(err)
		}
		if p.LowHull != (hull < 24) {
			t.Fatalf("warning at hull %d = %v", hull, p.LowHull)
		}
		r, err := sim.Jump(s, c, "eridani", 1)
		if err != nil {
			t.Fatal(err)
		}
		if r.FuelAfter != p.FuelAfter || r.HullAfter != p.HullAfter {
			t.Fatal("no-drift preview does not match action")
		}
	}
}

func TestEachRatingRequirementIsMandatory(t *testing.T) {
	c := testContent(t)
	for _, r := range c.Ratings {
		for _, missing := range []string{"credits", "location", "cargo", "runs", "pirates", "legendary"} {
			if missing == "cargo" && r.ReqCargoSoldValue == 0 || missing == "runs" && r.ReqRunsCount == 0 || missing == "pirates" && r.ReqPiratesDestroyed == 0 || missing == "legendary" && r.ReqLegendariesMined == 0 {
				continue
			}
			s := sim.New(c, 1, 0)
			s.JumpClass = r.Class - 1
			s.SystemID = r.CertifyIn
			sim.ActiveShip(s).SystemID = s.SystemID
			s.Credits = r.Price
			f := &sim.FrontierRecord{Visited: true, CargoValueSold: r.ReqCargoSoldValue, RunsByDestination: map[string]int{r.ReqRunsDestination: r.ReqRunsCount}, PiratesDestroyed: r.ReqPiratesDestroyed, LegendariesMined: r.ReqLegendariesMined}
			s.Frontier[r.CertifyIn] = f
			switch missing {
			case "credits":
				s.Credits--
			case "location":
				s.SystemID = r.Opens
				sim.ActiveShip(s).SystemID = r.Opens
			case "cargo":
				f.CargoValueSold--
			case "runs":
				f.RunsByDestination[r.ReqRunsDestination]--
			case "pirates":
				f.PiratesDestroyed--
			case "legendary":
				f.LegendariesMined--
			}
			before := s.Clone()
			if err := sim.CertifyRating(s, c); err == nil {
				t.Fatalf("class %d certified missing %s", r.Class, missing)
			}
			if !reflect.DeepEqual(before, s) {
				t.Fatal("rejection mutated save")
			}
		}
	}
}
func TestFirstCrossingNeverMisalignsAndScannerReducesDrift(t *testing.T) {
	c := testContent(t)
	c.Gates[2].MisalignmentWeight = 100000
	c.Gates[2].DriftChance = 1
	for seed := uint64(0); seed < 100; seed++ {
		s := sim.New(c, seed, 0)
		s.SystemID = "kepler"
		sim.ActiveShip(s).SystemID = "kepler"
		s.JumpClass = 2
		r, err := sim.Jump(s, c, "redline", 1)
		if err != nil {
			t.Fatal(err)
		}
		if r.Drift == "MISALIGNMENT" || s.SystemID != "redline" {
			t.Fatal("first crossing misaligned")
		}
	}
	s := sim.New(c, 1, 0)
	s.JumpClass = 0
	base, err := sim.PreviewJump(s, c, "eridani")
	if err != nil {
		t.Fatal(err)
	}
	sim.ActiveShip(s).Grades.Scanner = 5
	better, err := sim.PreviewJump(s, c, "eridani")
	if err != nil {
		t.Fatal(err)
	}
	if better.DriftChance != base.DriftChance/2 {
		t.Fatal("Scanner S does not halve drift")
	}
}
func TestFerryRejectsActiveAndEveryUnratedPathAtomically(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 0)
	s.Credits = 100000
	if err := sim.AcquireShip(s, c, "cicada"); err != nil {
		t.Fatal(err)
	}
	s.Frontier["redline"] = &sim.FrontierRecord{Visited: true}
	for class := -1; class < 2; class++ {
		s.JumpClass = class
		before := s.Clone()
		if err := sim.FerryShip(s, c, "cicada", "redline"); err == nil {
			t.Fatal("unrated ferry path accepted")
		}
		if !reflect.DeepEqual(s, before) {
			t.Fatal("ferry rejection changed save")
		}
	}
	s.JumpClass = 2
	before := s.Clone()
	if err := sim.FerryShip(s, c, "skiff", "redline"); err == nil {
		t.Fatal("active ship ferry accepted")
	}
	if !reflect.DeepEqual(s, before) {
		t.Fatal("active ferry changed save")
	}
}

func TestMigrationRejectsMalformedLegacyProperty(t *testing.T) {
	c := testContent(t)
	for _, field := range []string{`"ships":[1,2]`, `"inventory":{"item_id":"jump_drive"}`, `"system_permits":{"eridani":"yes"}`} {
		if _, err := sim.DecodeState([]byte(`{"version":7,`+field+`}`), c); err == nil {
			t.Fatalf("migration silently discarded malformed property: %s", field)
		}
	}
}

func TestMigrationRecognizesVisitedSystemFromDockSale(t *testing.T) {
	c := testContent(t)
	s, err := sim.DecodeState([]byte(`{"version":7,"world_idx":-1,"system_id":"sol","run_log":[{"when":12,"world":"ERIDANI DRIFT","outcome":"cargo_sold","cargo_value_sold":9000}]}`), c)
	if err != nil {
		t.Fatal(err)
	}
	f := s.Frontier["eridani"]
	if f == nil || !f.Visited {
		t.Fatal("known historical dock visit was lost")
	}
	if f.CargoValueSold != 0 {
		t.Fatal("migration invented mining provenance for a dock sale")
	}
}
