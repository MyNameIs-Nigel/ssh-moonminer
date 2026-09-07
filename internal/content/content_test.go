package content

import (
	"math"
	"testing"
	"testing/fstest"
)

func TestEmbeddedContentValid(t *testing.T) {
	if _, err := Load(""); err != nil {
		t.Fatal(err)
	}
}

func TestValidationRejectsUnsafeGenerationInputs(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Content)
	}{
		{"empty names", func(c *Content) { c.Belt.NamePrefixes = nil }},
		{"zero volume step", func(c *Content) { c.Belt.VolumeStep = 0 }},
		{"zero value step", func(c *Content) { c.Belt.ValueStep = 0 }},
		{"zero drill divisor", func(c *Content) { c.Belt.DrillSecPerVol = 0 }},
		{"reversed volumes", func(c *Content) { c.Belt.VolumeMax = c.Belt.VolumeMin - 1 }},
		{"negative scan cost", func(c *Content) { c.Belt.ScanFuelCost = -1 }},
		{"subnanosecond tick", func(c *Content) { c.Mining.TickHz = 1000000001 }},
		{"NaN probability", func(c *Content) { c.Mining.TributeChance = math.NaN() }},
		{"infinite pirate hull", func(c *Content) { c.Pirates[0].Hull = math.Inf(1) }},
		{"NaN tier multiplier", func(c *Content) { c.Tiers.Mults[0] = math.NaN() }},
	}
	for _, tc := range tests {
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

func TestDecodeTOMLRejectsUnknownKeys(t *testing.T) {
	fs := fstest.MapFS{"test.toml": {Data: []byte("start_credtis = 100\n")}}
	var p PilotStart
	if err := decodeTOML(fs, "test.toml", &p); err == nil {
		t.Fatal("misspelled field silently accepted")
	}
}
