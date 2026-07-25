// scrolledgecursor_test.go : tests edge scroll cursor shape and scroll velocity on the game screen.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/cursors"
)

// TestScrollEdgeCursorID verifies the edge scroll cursor turns into a NO cursor only
// when the view is fully clamped in the pointed direction, and keeps the arrow for a
// component that can still scroll (e.g. a corner clamped on one axis).
func TestScrollEdgeCursorID(t *testing.T) {
	const maxX, maxY = 100, 100

	// Mid map: every direction still scrolls - the arrow stands.
	if got := scrollEdgeCursorID(50, 50, maxX, maxY, -1, 0); got != cursors.ArrowW {
		t.Errorf("mid-map west = %v, want ArrowW", got)
	}

	// Fully scrolled left, pointing left: clamped on X, no vertical component -> NO.
	if got := scrollEdgeCursorID(0, 50, maxX, maxY, -1, 0); got != cursors.UnitNoGo {
		t.Errorf("clamped-left west = %v, want UnitNoGo", got)
	}

	// Fully scrolled right, pointing right: clamped on X -> NO.
	if got := scrollEdgeCursorID(maxX, 50, maxX, maxY, 1, 0); got != cursors.UnitNoGo {
		t.Errorf("clamped-right east = %v, want UnitNoGo", got)
	}

	// Bottom left corner pointing SW: clamped left (dx dropped) but can still scroll
	// down -> keep the S arrow, not NO.
	if got := scrollEdgeCursorID(0, 50, maxX, maxY, -1, 1); got != cursors.ArrowS {
		t.Errorf("clamped-left, can-go-down = %v, want ArrowS", got)
	}

	// Top left corner pointing NW with both axes clamped -> NO.
	if got := scrollEdgeCursorID(0, 0, maxX, maxY, -1, -1); got != cursors.UnitNoGo {
		t.Errorf("top-left corner NW = %v, want UnitNoGo", got)
	}
}
