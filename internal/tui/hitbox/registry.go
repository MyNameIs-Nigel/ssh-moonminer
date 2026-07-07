package hitbox

// Box is a clickable region.
type Box struct {
	X, Y, W, H int
	ID         string
	Data       any
}

// Registry tracks interactive regions for the current frame.
type Registry struct {
	boxes []Box
}

// New creates an empty registry.
func New() *Registry { return &Registry{} }

// Add registers a box (later boxes win on overlap).
func (r *Registry) Add(b Box) { r.boxes = append(r.boxes, b) }

// At returns the topmost box at x,y.
func (r *Registry) At(x, y int) (Box, bool) {
	for i := len(r.boxes) - 1; i >= 0; i-- {
		b := r.boxes[i]
		if x >= b.X && x < b.X+b.W && y >= b.Y && y < b.Y+b.H {
			return b, true
		}
	}
	return Box{}, false
}

// Clear resets for next frame.
func (r *Registry) Clear() { r.boxes = r.boxes[:0] }
