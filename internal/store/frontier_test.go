package store_test

import (
	"context"
	"testing"

	"github.com/mynameis-nigel/ssh-moonminer/internal/content"
	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
	"github.com/mynameis-nigel/ssh-moonminer/internal/store"
)

func TestRealV7BlobMigratesAndPersistsFrontier(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, t.TempDir()+"/pilot.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	c, err := content.Load("")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.TouchAccount(ctx, "pilot", "test public key", 1); err != nil {
		t.Fatal(err)
	}
	legacy := []byte(`{"version":7,"credits":101,"system_id":"eridani","world_idx":-1,"active_ship_id":"warden","ships":{"warden":{"model_id":"warden","hull":77,"base_fuel":34,"jump_drive":{"item_id":"jump_drive","grade":0}},"skiff":{"model_id":"skiff","hull":55,"base_fuel":23}},"inventory":[{"item_id":"jump_drive","grade":3},{"item_id":"fuel_tank","grade":2,"fuel":11}],"run_log":[{"when":8,"world":"SABLE","outcome":"bailed","tier":3,"cargo_value_recovered":1000,"pirate_destroyed":"rake"}]}`)
	row, _, err := db.LoadOrCreateSave(ctx, "pilot", "default", 1, func() ([]byte, int, error) { return legacy, 7, nil })
	if err != nil {
		t.Fatal(err)
	}
	s, err := sim.DecodeState(row.State, c)
	if err != nil {
		t.Fatal(err)
	}
	if s.JumpClass != 0 || s.Credits != 23851 || s.Ships["warden"].Hull != 77 || s.Ships["skiff"].BaseFuel != 23 || s.Ships["skiff"].SystemID != "eridani" || len(s.Inventory) != 1 || s.Inventory[0].Fuel != 11 {
		t.Fatalf("migration lost pilot property: %+v", s)
	}
	f := s.Frontier["eridani"]
	if f.RunsSurvived != 1 || f.RunsByDestination["sable_halo"] != 1 || f.PiratesDestroyed != 1 || f.CargoValueSold != 0 {
		t.Fatalf("unsafe historical qualification seed: %+v", f)
	}
	s.Frontier["eridani"].CargoValueSold = 200
	s.CargoOrigins = map[string]int{"eridani": 41}
	s.CargoValue = 41
	payload, err := s.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveState(ctx, "pilot", "default", payload, sim.StateVersion, 2); err != nil {
		t.Fatal(err)
	}
	row, created, err := db.LoadOrCreateSave(ctx, "pilot", "default", 3, func() ([]byte, int, error) { t.Fatal("unexpected create"); return nil, 0, nil })
	if err != nil || created {
		t.Fatalf("reload: %v", err)
	}
	got, err := sim.DecodeState(row.State, c)
	if err != nil {
		t.Fatal(err)
	}
	if got.JumpClass != 0 || got.Credits != 23851 || got.Frontier["eridani"].CargoValueSold != 200 || got.CargoOrigins["eridani"] != 41 {
		t.Fatal("v8 save did not survive SQLite round trip")
	}
}
