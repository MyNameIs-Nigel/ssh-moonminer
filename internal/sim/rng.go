package sim

import "math/rand"

// newRNG creates a deterministic RNG from seed and belt count discriminator.
func newRNG(seed uint64, beltCount uint64) *rand.Rand {
	return rand.New(rand.NewSource(int64(seed ^ beltCount*0x9E3779B97F4A7C15)))
}

// runRNG derives a deterministic, per-tick RNG for in-progress-run dice
// rolls (pirate action, event rolls) from state + run fields already
// present, so the sim never stores a live *rand.Rand and stays a pure
// function of its inputs (same seed + same tick sequence ⇒ identical rolls).
func runRNG(s *State, run *ActiveRun, salt uint64) *rand.Rand {
	discriminator := uint64(run.AsteroidID)*1000003 + run.TickCount*97 + salt
	return newRNG(s.Seed, discriminator)
}
