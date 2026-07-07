package sim

import "math/rand"

// newRNG creates a deterministic RNG from seed and belt count discriminator.
func newRNG(seed uint64, beltCount uint64) *rand.Rand {
	return rand.New(rand.NewSource(int64(seed ^ beltCount*0x9E3779B97F4A7C15)))
}
