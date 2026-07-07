package hitbox_test

import (
	"testing"

	"github.com/mynameis-nigel/ssh-moonminer/internal/tui/hitbox"
)

func TestAtOverlap(t *testing.T) {
	r := hitbox.New()
	r.Add(hitbox.Box{X: 0, Y: 0, W: 10, H: 1, ID: "first"})
	r.Add(hitbox.Box{X: 5, Y: 0, W: 10, H: 1, ID: "second"})
	b, ok := r.At(6, 0)
	if !ok || b.ID != "second" {
		t.Fatalf("got %v ok=%v", b.ID, ok)
	}
}
