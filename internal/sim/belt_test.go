package sim_test

import (
	"testing"

	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
)

// TestSeismicSensorsRespectScannerRange covers a bug where the free
// pre-scan granted by the Seismic Sensors internal module ignored the
// ship's Scanner distance lock entirely, letting a cheap module bypass the
// paid Scanner track progression for any asteroid in the belt.
func TestSeismicSensorsRespectScannerRange(t *testing.T) {
	c := testContent(t)

	for seed := uint64(1); seed <= 50; seed++ {
		s := sim.New(c, seed, 1000)
		if err := sim.InstallSlotDevice(s, c, "skiff", sim.SlotInternal, 0, sim.ItemSeismic, 0); err != nil {
			t.Fatal(err)
		}
		if err := sim.Depart(s, c, 1); err != nil {
			t.Fatal(err)
		}
		for _, ast := range s.Belt {
			if ast.Scanned && sim.IsOutOfRange(s, c, &ast) {
				t.Fatalf("seed %d: Seismic Sensors pre-scanned %s at %.1fkm, beyond the %.1fkm scanner lock",
					seed, ast.Name, ast.Distance, sim.ScannerLockKm(s, c))
			}
		}
	}
}

// TestGenerateBeltAlwaysHasAScannableContact prevents an unlucky distance roll
// from leaving a pilot with no reachable target and therefore no way to start
// the mining loop.
func TestGenerateBeltAlwaysHasAScannableContact(t *testing.T) {
	c := testContent(t)
	for worldIdx := range c.Worlds {
		for seed := uint64(1); seed <= 100; seed++ {
			s := sim.New(c, seed, 1000)
			belt := sim.GenerateBelt(s, c, worldIdx)
			found := false
			for i := range belt {
				if !sim.IsOutOfRange(s, c, &belt[i]) {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("world %d seed %d has no asteroid within the %.1fkm scanner lock", worldIdx, seed, sim.ScannerLockKm(s, c))
			}
		}
	}
}
