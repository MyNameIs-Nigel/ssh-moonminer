package sim_test

import (
	"reflect"
	"testing"

	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
)

func TestFrontierCargoOriginSurvivalAndLoss(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 78, 10)
	s.Credits = 100000
	s.JumpClass = 0
	if err := sim.InstallSlotDevice(s, c, "skiff", sim.SlotUtility, 0, sim.ItemFuelTank, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := sim.Jump(s, c, "eridani", 20); err != nil {
		t.Fatal(err)
	}
	if err := sim.Depart(s, c, 4); err != nil {
		t.Fatal(err)
	}
	s.Belt[0].Scanned = true
	s.Belt[0].Tier = 3
	if err := sim.Lock(s, c, s.Belt[0].ID, 30); err != nil {
		t.Fatal(err)
	}
	s.Run.HeldUnits = 10
	s.Run.ExtractedUnits = 10
	s.Run.CargoValue = 500
	s.Run.PirateDistance = 100000
	sim.EmergencyResolve(s, c, 40)
	if s.Frontier["eridani"].RunsSurvived != 1 || s.Frontier["eridani"].RunsByDestination["eris_veil"] != 1 {
		t.Fatal("disconnect survival not recorded")
	}
	sim.Dock(s, c)
	if _, err := sim.Jump(s, c, "sol", 50); err != nil {
		t.Fatal(err)
	}
	if _, err := sim.SellCargo(s, c, 60); err != nil {
		t.Fatal(err)
	}
	if s.Frontier["eridani"].CargoValueSold != 500 || s.Frontier["sol"].CargoValueSold != 0 {
		t.Fatal("sale attributed to wrong origin")
	}
	if err := sim.Refuel(s, c); err != nil {
		t.Fatal(err)
	}
	if err := sim.Depart(s, c, 1); err != nil {
		t.Fatal(err)
	}
	s.Belt[0].Scanned = true
	if err := sim.Lock(s, c, s.Belt[0].ID, 70); err != nil {
		t.Fatal(err)
	}
	s.Hull = 0
	sim.TickRun(s, c, 0, 80)
	if s.JumpClass != 0 || s.Frontier["sol"].ShipsLost != 1 {
		t.Fatal("death erased rating or missed loss")
	}
}
func TestFrontierHullsAndLocalRecovery(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 0)
	s.Credits = 1000000
	before := s.Clone()
	if err := sim.AcquireShip(s, c, "lantern"); err == nil {
		t.Fatal("frontier hull sold in Sol")
	}
	if !reflect.DeepEqual(s, before) {
		t.Fatal("failed purchase mutated")
	}
	s.SystemID = "kepler"
	sim.ActiveShip(s).SystemID = "kepler"
	if err := sim.AcquireShip(s, c, "lantern"); err != nil {
		t.Fatal(err)
	}
	if err := sim.InstallSlotDevice(s, c, "lantern", sim.SlotInternal, 0, sim.ItemFuelMiner, 0); err != nil {
		t.Fatal(err)
	}
	if err := sim.InstallSlotDevice(s, c, "lantern", sim.SlotInternal, 1, sim.ItemSeismic, 0); err != nil {
		t.Fatal(err)
	}
	if !sim.ShipHasSlotItem(s, "lantern", sim.ItemFuelMiner) || !sim.ShipHasSlotItem(s, "lantern", sim.ItemSeismic) {
		t.Fatal("two internals not active")
	}
	s.SystemID = "redline"
	sim.ActiveShip(s).SystemID = "redline"
	if err := sim.AcquireShip(s, c, "vesper"); err != nil {
		t.Fatal(err)
	}
	if err := sim.SwitchActiveShip(s, c, "vesper"); err != nil {
		t.Fatal(err)
	}
	if err := sim.InstallSlotDevice(s, c, "vesper", sim.SlotUtility, 0, sim.ItemShield, 0); err == nil {
		t.Fatal("Vesper accepted shield")
	}
	s.Hull = 50
	s.Ships["vesper"].Hull = 50
	s.WorldIdx = 9
	sim.Dock(s, c)
	healed := s.Hull
	if healed <= 50 {
		t.Fatal("Vesper did not heal on dock")
	}
	sim.Dock(s, c)
	if s.Hull != healed {
		t.Fatal("repeated Dock farms repairs")
	}
	s.Ships["skiff"].SystemID = "sol"
	s.WorldIdx = 9
	s.Belt = sim.GenerateBelt(s, c, 9)
	s.Belt[0].Scanned = true
	if err := sim.Lock(s, c, s.Belt[0].ID, 1); err != nil {
		t.Fatal(err)
	}
	s.Hull = 0
	sim.TickRun(s, c, 0, 2)
	if s.SystemID != "redline" || sim.ActiveShip(s).SystemID != "redline" || s.ActiveShipID != "skiff" {
		t.Fatal("recovery teleported pilot or stranded them")
	}
	if sim.IsBuyback(s, c, "vesper") || sim.ShipAcquirePrice(s, c, "vesper") != 600000 {
		t.Fatal("Vesper discounted after death")
	}
}

func TestTributeDoesNotCreditLostCargoToFrontier(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 0)
	s.CargoValue = 600
	s.CargoUnits = 600
	s.CargoOrigins = map[string]int{"sol": 300, "eridani": 300}
	if err := sim.Depart(s, c, 1); err != nil {
		t.Fatal(err)
	}
	s.Belt[0].Scanned = true
	if err := sim.Lock(s, c, s.Belt[0].ID, 1); err != nil {
		t.Fatal(err)
	}
	s.Run.Phase = sim.PhaseTribute
	s.Run.CargoValue = 400
	s.Run.HeldUnits = 400
	s.Run.ExtractedUnits = 400
	if err := sim.AcceptTribute(s, c, 2); err != nil {
		t.Fatal(err)
	}
	if s.CargoOrigins["sol"]+s.CargoOrigins["eridani"] != s.CargoValue {
		t.Fatal("tribute left phantom sale provenance")
	}
}
func TestRedlineInstabilityIsADeterministicClock(t *testing.T) {
	c := testContent(t)
	s := sim.New(c, 1, 0)
	s.SystemID = "redline"
	sim.ActiveShip(s).SystemID = "redline"
	s.JumpClass = 2
	if err := sim.Depart(s, c, 9); err != nil {
		t.Fatal(err)
	}
	s.Belt[0].Scanned = true
	if err := sim.Lock(s, c, s.Belt[0].ID, 1); err != nil {
		t.Fatal(err)
	}
	s.Run.PirateDistance = 100000
	a, b := s.Clone(), s.Clone()
	sim.TickRun(a, c, 10, 11)
	sim.TickRun(b, c, 10, 11)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("instability replay differs")
	}
	if a.Run == nil || a.Run.BeltStability >= 1 || a.Run.BeltStability <= 0 {
		t.Fatal("Redline stability did not decay")
	}
}

func TestEverySystemHasALocalRecoveryRun(t *testing.T) {
	c := testContent(t)
	for _, sys := range c.Systems {
		t.Run(sys.ID, func(t *testing.T) {
			s := sim.New(c, 15, 0)
			s.SystemID = sys.ID
			sim.ActiveShip(s).SystemID = sys.ID
			s.JumpClass = 2
			s.Credits = 0
			sim.DevSetFuel(s, c, 0)
			if !sim.InsuranceEligible(s, c) {
				t.Fatal("stranded local Skiff denied salvage advance")
			}
			if err := sim.InsuranceAdvance(s, c); err != nil {
				t.Fatal(err)
			}
			if err := sim.Refuel(s, c); err != nil {
				t.Fatal(err)
			}
			idx := -1
			for i, w := range c.Worlds {
				if w.SystemID == sys.ID && sim.CanDepart(s, c, i) {
					idx = i
					break
				}
			}
			if idx < 0 {
				t.Fatal("recovery cannot reach a local belt")
			}
			if err := sim.Depart(s, c, idx); err != nil {
				t.Fatal(err)
			}
			s.Belt[0].Scanned = true
			if err := sim.Lock(s, c, s.Belt[0].ID, 1); err != nil {
				t.Fatalf("advance did not fund a local run: %v", err)
			}
		})
	}
}

func TestFreeLocalSkiffPreservesRemoteProperty(t *testing.T){
 c:=testContent(t);s:=sim.New(c,1,0);s.Ships["skiff"].Internal=&sim.SlotDevice{ItemID:sim.ItemFuelMiner,Grade:2};remote:=s.Ships["skiff"]
 s.SystemID="eridani";s.Credits=0
 if err:=sim.AcquireShip(s,c,"skiff");err!=nil{t.Fatalf("no-local-hull recovery denied: %v",err)}
 if s.ActiveShipID!="skiff"||sim.ActiveShip(s).SystemID!="eridani"||s.Credits!=0{t.Fatal("recovery not free and local")}
 if s.Ships["skiff@sol"]!=remote||remote.SystemID!="sol"||remote.Internal.Grade!=2{t.Fatal("recovery lost or teleported remote property")}
 b,err:=s.Encode();if err!=nil{t.Fatal(err)};got,err:=sim.DecodeState(b,c);if err!=nil{t.Fatal(err)};if got.Ships["skiff@sol"].Internal.Grade!=2{t.Fatal("remote Skiff did not persist")}
}
