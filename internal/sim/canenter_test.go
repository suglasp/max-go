// canenter_test.go : tests cell enter passability checks in sim.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package sim

import "testing"

// TestCanEnter checks the move cursor passability helper: a land unit can
// enter empty land cells inside the map bounds, but not cells outside the map
// nor its own cell (already occupied by the same class).
func TestCanEnter(t *testing.T) {
	g := newGame(t, 8, 8)       // all land
	u := addTank(g, 1, 2, 1, 1) // the unit occupies (1,1)

	for _, c := range [][2]int{{0, 0}, {7, 7}, {4, 4}, {0, 7}} {
		if !g.CanEnter(u, c[0], c[1]) {
			t.Errorf("CanEnter%v = false on empty land, want true", c)
		}
	}
	if g.CanEnter(u, 1, 1) {
		t.Error("CanEnter on the unit's own occupied cell should be false")
	}
	for _, c := range [][2]int{{-1, 0}, {0, -1}, {8, 0}, {0, 8}, {99, 99}} {
		if g.CanEnter(u, c[0], c[1]) {
			t.Errorf("CanEnter%v = true out of bounds, want false", c)
		}
	}
}
