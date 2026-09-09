package content

import "testing"

func TestJumpValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Content)
	}{
		{"stranding", func(c *Content) { c.Gates[2].JumpFuelBase = 200 }},
		{"unknown gate", func(c *Content) { c.Gates[0].To = "missing" }},
		{"duplicate gate", func(c *Content) { c.Gates = append(c.Gates, c.Gates[0]) }},
		{"unreachable qualification", func(c *Content) { c.Ratings[1].ReqRunsDestination = "vesta_local" }},
		{"invalid drift", func(c *Content) { c.Gates[0].DriftChance = 1.1 }},
		{"unavailable hull", func(c *Content) { c.Ships[0].SoldIn = []string{"missing"} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, err := Load("")
			if err != nil {
				t.Fatal(err)
			}
			tc.change(c)
			if err := c.validate(); err == nil {
				t.Fatal("unsafe content accepted")
			}
		})
	}
}

func TestFrontierHullPricesAndSpecializations(t *testing.T) {
	c, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"lantern", "halberd", "vesper"} {
		m := c.ShipByID(id)
		if len(m.SoldIn) != 1 {
			t.Fatal("frontier hull has multiple markets")
		}
		r := c.Ratings[c.SystemByID(m.SoldIn[0]).RequiredRating]
		if m.Price <= r.Price {
			t.Fatalf("%s costs less than its home certification", id)
		}
	}
	lantern, halberd, vesper := c.ShipByID("lantern"), c.ShipByID("halberd"), c.ShipByID("vesper")
	if lantern.InternalSlots != 2 || lantern.WeaponSlots != 0 || lantern.HullCap != 1 || lantern.ScannerCap != 4 {
		t.Fatal("Lantern lost its fragile logistics tradeoff")
	}
	if halberd.PowerGenCap != 5 || halberd.FuelEffCap != 1 {
		t.Fatal("Halberd lost its power/fuel tradeoff")
	}
	if !vesper.NoShield || !vesper.NoBuyback || vesper.DockRepairPct <= 0 || vesper.WeaponSlots != 2 {
		t.Fatal("Vesper lost its high-stakes tradeoff")
	}
}
