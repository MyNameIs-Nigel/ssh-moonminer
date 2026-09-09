package sim_test

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"testing"

	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
)

func cloneFixture() *sim.State {
	d := func() *sim.SlotDevice { return &sim.SlotDevice{ItemID: sim.ItemFuelTank, Fuel: 7} }
	return &sim.State{
		Ships:        map[string]*sim.ShipInstance{"skiff": {ModelID: "skiff", Utility: []*sim.SlotDevice{d(), nil}, Weapon: []*sim.SlotDevice{d()}, Internal: d(), AdditionalInternal: []*sim.SlotDevice{d()}}},
		ActiveShipID: "skiff", ShipsUnlocked: map[string]bool{"skiff": true},
		Frontier:     map[string]*sim.FrontierRecord{"sol": {Visited: true, RunsByDestination: map[string]int{"vesta": 1}}},
		CargoOrigins: map[string]int{"sol": 7}, ShipsLostAt: map[string]string{"mule": "sol"}, HotArrivals: map[string]bool{"sol": true}, CrossedGates: map[string]bool{"sol:eridani": true}, LastJump: &sim.JumpResult{HullAfter: 5}, DestinationPermits: map[string]bool{"vesta": true},
		Belt: []sim.Asteroid{{ID: 1}}, Inventory: []*sim.SlotDevice{d()},
		RunLog:           []sim.RunRecord{{Events: []string{"event"}}},
		DisconnectNotice: &sim.RunRecord{Events: []string{"notice"}},
		Scan:             &sim.ActiveScan{AsteroidID: 1}, DevGodMode: true,
		Run: &sim.ActiveRun{PressurePoints: []sim.PressurePoint{{Position: 1}}, SkillCheck: &sim.SkillCheck{Window: 1}, ActiveEvent: &sim.RunEvent{Remaining: 1}, EventLog: []sim.RunEventRecord{{At: 1}}, Combat: &sim.CombatState{Log: []string{"combat"}}},
	}
}

func TestClonePreservesValuesAndIsolatesAllReferences(t *testing.T) {
	source := cloneFixture()
	before := cloneFixture()
	copy := source.Clone()
	if !reflect.DeepEqual(source, copy) {
		t.Fatal("clone changed values")
	}
	ship := copy.Ships["skiff"]
	ship.Hull++
	for _, d := range []*sim.SlotDevice{ship.Utility[0], ship.Weapon[0], ship.Internal, ship.AdditionalInternal[0], copy.Inventory[0]} {
		d.Fuel++
	}
	copy.Ships["new"] = &sim.ShipInstance{}
	copy.ShipsUnlocked["skiff"] = false
	copy.Frontier["sol"].Visited = false
	copy.Frontier["sol"].RunsByDestination["vesta"]++
	copy.CargoOrigins["sol"]++
	copy.ShipsLostAt["mule"] = "eridani"
	copy.HotArrivals["sol"] = false
	copy.CrossedGates["sol:eridani"] = false
	copy.LastJump.HullAfter++
	copy.DestinationPermits["vesta"] = false
	copy.Belt[0].ID++
	copy.RunLog[0].Events[0] = "changed"
	copy.DisconnectNotice.Events[0] = "changed"
	copy.Scan.Elapsed++
	copy.Run.PressurePoints[0].Position++
	copy.Run.SkillCheck.Window++
	copy.Run.ActiveEvent.Remaining++
	copy.Run.EventLog[0].At++
	copy.Run.Combat.Log[0] = "changed"
	if !reflect.DeepEqual(source, before) {
		t.Fatal("clone aliases source memory")
	}
}

func TestEncodeTransientStateAndInvalidNumbers(t *testing.T) {
	s := cloneFixture()
	before := s.Clone()
	b, err := s.Encode()
	if err != nil {
		t.Fatal(err)
	}
	var saved sim.State
	if err := json.Unmarshal(b, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Run != nil || saved.Scan != nil || saved.DevGodMode {
		t.Fatal("transient state persisted")
	}
	if !reflect.DeepEqual(s, before) {
		t.Fatal("encode mutated source")
	}
	s.CargoUnits = math.NaN()
	if _, err := s.Encode(); err == nil {
		t.Fatal("invalid number should return an encoding error")
	}
}

func TestDecodeNullParkedShip(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 42, 0)
	s.Ships["removed"] = nil
	b, err := s.Encode()
	if err != nil {
		t.Fatal(err)
	}
	got, err := sim.DecodeState(b, c)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.Ships["removed"]; ok {
		t.Fatal("null hangar entry retained")
	}
	if got.ActiveShipID != s.ActiveShipID {
		t.Fatal("valid active ship replaced")
	}
}

func TestLockScannerRangeBoundary(t *testing.T) {
	for _, extra := range []float64{0, 0.1} {
		t.Run(fmt.Sprintf("extra_%.1f_km", extra), func(t *testing.T) {
			c := testContent(t)
			s := sim.New(c, 1, 0)
			s.WorldIdx = 1
			s.Belt = []sim.Asteroid{{ID: 1, Distance: sim.ScannerLockKm(s, c) + extra, Scanned: true, Volume: 100, DrillSec: 10, FuelCost: 1}}
			before := s.Clone()
			err := sim.Lock(s, c, 1, 0)
			if extra == 0 {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err != sim.ErrOutOfRange {
				t.Fatalf("got %v, want ErrOutOfRange", err)
			}
			if !reflect.DeepEqual(s, before) {
				t.Fatal("rejected lock mutated state")
			}
		})
	}
}

func TestRunPredicatesDoNotMutateLegacyAccounting(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 0)
	r := &sim.ActiveRun{MinedUnits: 100}
	before := *r
	if !sim.RunDepleted(s, c, &sim.Asteroid{Volume: 100}, r) {
		t.Fatal("legacy depletion lost")
	}
	sim.RunCargoFull(s, c, r)
	if !reflect.DeepEqual(*r, before) {
		t.Fatal("rendering predicate mutated run")
	}
	if sim.RunDepleted(s, c, &sim.Asteroid{Volume: 100}, nil) {
		t.Fatal("absent run depleted a rock")
	}
}

func BenchmarkStateClone(b *testing.B) {
	c, err := content.Load("")
	if err != nil {
		b.Fatal(err)
	}
	s := cloneFixture()
	s.Belt = sim.GenerateBelt(s, c, 0)
	for len(s.RunLog) < 20 {
		s.RunLog = append(s.RunLog, sim.RunRecord{Events: []string{"power outage", "tribute"}})
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if s.Clone() == nil {
			b.Fatal("nil clone")
		}
	}
}

func TestEmergencyResolveHonorsFuelOutPenalty(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 0)
	s.WorldIdx = 1
	s.Belt = []sim.Asteroid{{ID: 1, Volume: 100, DrillSec: 10, Value: 100}}
	s.Fuel = 0
	s.Run = &sim.ActiveRun{AsteroidID: 1, Phase: sim.PhaseEscaping, Intent: sim.OutcomeBailed, BaseEscapeSecondsRequired: 2, EscapeSecondsRequired: 2, UnderAttack: true, StartHull: s.Hull, EventCooldown: 10000}
	expected := s.Clone()
	for i := 0; i < 10000 && expected.Run != nil; i++ {
		sim.TickRun(expected, c, 1/float64(c.Mining.TickHz), 1000)
	}
	if expected.Run != nil {
		t.Fatal("fixture escape did not terminate")
	}
	out := sim.EmergencyResolve(s, c, 1000)
	if out == nil || s.Run != nil {
		t.Fatal("autopilot did not resolve")
	}
	if s.Hull != expected.Hull || s.Fuel != expected.Fuel || s.RunLog[0].Outcome != expected.RunLog[0].Outcome {
		t.Fatalf("autopilot differs from normal ticks: hull %d/%d fuel %v/%v outcome %s/%s", s.Hull, expected.Hull, s.Fuel, expected.Fuel, s.RunLog[0].Outcome, expected.RunLog[0].Outcome)
	}
}

func FuzzDecodeState(f *testing.F) {
	c, err := content.Load("")
	if err != nil {
		f.Fatal(err)
	}
	b, err := sim.New(c, 42, 0).Encode()
	if err != nil {
		f.Fatal(err)
	}
	f.Add(b)
	f.Add([]byte(`{"version":7,"active_ship_id":"skiff","ships":{"skiff":{"model_id":"skiff"},"parked":null}}`))
	f.Add([]byte(`{"version":999}`))
	f.Add([]byte(`{`))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 65536 {
			t.Skip("bound fuzz input size")
		}
		s, err := sim.DecodeState(data, c)
		if err != nil {
			return
		}
		if s == nil || sim.ActiveShip(s) == nil {
			t.Fatal("accepted save has no active ship")
		}
		if _, err := s.Encode(); err != nil {
			t.Fatalf("accepted save cannot be encoded: %v", err)
		}
	})
}

func TestEmergencyResolveDoesNotRelabelOlderRun(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 0)
	s.RunLog = []sim.RunRecord{{Outcome: string(sim.OutcomeDeparted)}}
	s.Run = &sim.ActiveRun{AsteroidID: 999, Phase: sim.PhaseMining}
	if out := sim.EmergencyResolve(s, c, 1000); out != nil {
		t.Fatal("missing asteroid created an outcome")
	}
	if s.RunLog[0].Disconnected || s.DisconnectNotice != nil {
		t.Fatal("missing asteroid relabeled an unrelated historical run")
	}
}

func TestEmergencyResolveMatchesLiveEscapePhases(t *testing.T) {
	for _, phase := range []sim.RunPhase{sim.PhaseMining, sim.PhaseTribute, sim.PhaseEscaping, sim.PhaseCombat} {
		t.Run(fmt.Sprint(phase), func(t *testing.T) {
			c := testContent(t)
			s := sim.New(c, 1, 0)
			s.WorldIdx = 1
			s.Belt = []sim.Asteroid{{ID: 1, Volume: 100, Value: 100, DrillSec: 10, Scanned: true, Distance: 1}}
			if err := sim.Lock(s, c, 1, 0); err != nil {
				t.Fatal(err)
			}
			s.Run.ExtractedUnits = 50
			s.Run.HeldUnits = 50
			s.Run.CargoValue = 50
			switch phase {
			case sim.PhaseTribute:
				s.Run.Phase = phase
			case sim.PhaseEscaping:
				if err := sim.BailOrDepart(s, c, 1000); err != nil {
					t.Fatal(err)
				}
			case sim.PhaseCombat:
				s.Run.Phase = sim.PhaseTribute
				if err := sim.RefuseTribute(s, c, 1000); err != nil {
					t.Fatal(err)
				}
				s.Run.Combat.EscapeStarted = false
			}
			expected := s.Clone()
			switch phase {
			case sim.PhaseMining:
				if err := sim.BailOrDepart(expected, c, 1000); err != nil {
					t.Fatal(err)
				}
			case sim.PhaseTribute:
				if err := sim.RefuseTribute(expected, c, 1000); err != nil {
					t.Fatal(err)
				}
			case sim.PhaseCombat:
				if err := sim.CombatEscape(expected, c, 1000); err != nil {
					t.Fatal(err)
				}
			}
			for i := 0; i < 10000 && expected.Run != nil; i++ {
				sim.TickRun(expected, c, 1/float64(c.Mining.TickHz), 1000)
			}
			if expected.Run != nil || len(expected.RunLog) == 0 {
				t.Fatal("live escape fixture failed")
			}
			if out := sim.EmergencyResolve(s, c, 1000); out == nil {
				t.Fatal("missing emergency outcome")
			}
			expected.RunLog[0].Disconnected = true
			record := expected.RunLog[0]
			expected.DisconnectNotice = &record
			if !reflect.DeepEqual(s, expected) {
				t.Fatal("autopilot and live escape final states differ")
			}
		})
	}
}
