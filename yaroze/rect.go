package yaroze

import "go.chrisrx.dev/psx/geom"

// Clamp returns x and y moved as little as they need to be for a w by h box
// at them to lie inside the rectangle.
func (r RECT) Clamp(x, y int16, w, h uint16) (int16, int16) {
	return min(max(x, r.X), r.X+r.W-int16(w)), min(max(y, r.Y), r.Y+r.H-int16(h))
}

// ToRECT is the rectangle r as the library takes it, for its own calls.
func ToRECT(r geom.Rect) RECT {
	return RECT{
		X: int16(r.X),
		Y: int16(r.Y),
		W: int16(r.W),
		H: int16(r.H),
	}
}
