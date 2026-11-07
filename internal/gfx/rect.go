// rect.go : defines inclusive GNW rectangles and geometry helpers.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : rect.c - GNW rect helpers
package gfx

// Rect is a GNW rectangle. Coordinates name the upper left (ULX, ULY)
// and lower right (LRX, LRY) corners. Most GNW code treats the
// lower right corner as inclusive (a window covering pixels 0..9 has
// ulx=0, lrx=9); Width/Height follow rect.c and return LRX-ULX, so
// callers that need the inclusive pixel size add 1 like the original
// code does.
type Rect struct {
	ULX, ULY, LRX, LRY int
}

// NewRect builds a Rect from its four corners.
//
// C++: rect_init (rect.c)
func NewRect(ulx, uly, lrx, lry int) Rect {
	return Rect{ulx, uly, lrx, lry}
}

// Width returns LRX-ULX.
//
// C++: rect_get_width (rect.c)
func (r Rect) Width() int {
	return r.LRX - r.ULX
}

// Height returns LRY-ULY.
//
// C++: rect_get_height (rect.c)
func (r Rect) Height() int {
	return r.LRY - r.ULY
}

/* // note: ported but unused code in Go port
// no caller joins two rects; the reference has no caller either.
// Union returns the smallest rectangle containing both r and o.
//
// C++: rect_min_bound (rect.c)
func (r Rect) Union(o Rect) Rect {
	u := r
	if o.ULX < u.ULX {
		u.ULX = o.ULX
	}
	if o.ULY < u.ULY {
		u.ULY = o.ULY
	}
	if o.LRX > u.LRX {
		u.LRX = o.LRX
	}
	if o.LRY > u.LRY {
		u.LRY = o.LRY
	}
	return u
}
*/

// Intersect clips r against bound, returning ok=false when the two do
// not overlap (corners inclusive, as in GNW).
//
// C++: rect_inside_bound (rect.c)
func (r Rect) Intersect(bound Rect) (clipped Rect, ok bool) {
	if r.ULX > bound.LRX || r.LRX < bound.ULX || r.ULY > bound.LRY || r.LRY < bound.ULY {
		return r, false
	}
	clipped = r
	if bound.ULX > clipped.ULX {
		clipped.ULX = bound.ULX
	}
	if clipped.LRX > bound.LRX {
		clipped.LRX = bound.LRX
	}
	if bound.ULY > clipped.ULY {
		clipped.ULY = bound.ULY
	}
	if clipped.LRY > bound.LRY {
		clipped.LRY = bound.LRY
	}
	return clipped, true
}

// Contains reports whether the point (x, y) lies in r, corners
// inclusive.
func (r Rect) Contains(x, y int) bool {
	return x >= r.ULX && x <= r.LRX && y >= r.ULY && y <= r.LRY
}
