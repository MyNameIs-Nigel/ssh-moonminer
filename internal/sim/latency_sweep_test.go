package sim_test

import (
	"testing"
	"time"

	"github.com/mynameis-nigel/ssh-moonminer/internal/sim"
)

// The player spends 2s deciding after seeing the light. Advance the real
// mining simulation one actor tick at a time until their key arrives.
func TestPressurePointLatencySweep(t *testing.T) {
	c := testContent(t)
	const start = 1000
	base := sim.New(c, 99, start)
	if err := sim.Depart(base, c, 1); err != nil {
		t.Fatal(err)
	}
	base.Belt[0].Scanned = true
	if err := sim.Lock(base, c, base.Belt[0].ID, start); err != nil {
		t.Fatal(err)
	}
	base.Run.NextSkillCheckIn = 0
	base.Run.PirateDistance = 1e9
	sim.TickRun(base, c, 0, start)
	check := base.Run.SkillCheck
	if check == nil {
		t.Fatal("fixture has no active pressure point")
	}
	tick := sim.TickInterval(c)
	for _, rtt := range []time.Duration{0, 150 * time.Millisecond, 300 * time.Millisecond, time.Second, 1500 * time.Millisecond} {
		t.Run(rtt.String(), func(t *testing.T) {
			state := base.Clone()
			arrival := 2*time.Second + rtt
			for elapsed := tick; elapsed <= arrival; elapsed += tick {
				sim.TickRun(state, c, tick.Seconds(), start+int64(elapsed/time.Second))
			}
			if err := sim.AttemptSkillCheck(state, c, start+int64(arrival/time.Second)); err != nil {
				t.Fatal(err)
			}
			status := state.Run.PressurePoints[check.PressurePoint].Status
			want := sim.PressurePointHit
			if arrival.Truncate(tick).Seconds() >= check.Window {
				want = sim.PressurePointMissed
			}
			t.Logf("RTT %v, reaction 2s, window %.2fs, outcome %s", rtt, check.Window, status)
			if status != want {
				t.Fatalf("want %s", want)
			}
		})
	}
}
