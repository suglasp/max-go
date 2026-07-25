// build_placement_test.go : tests the constructor build site clamp.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import "testing"

// TestClampBuildSiteAnchorsOnBuilder verifies the vanilla rule that a constructor's 2x2
// build site is clamped to overlap the builder's own cell (the quadrant toward the
// cursor), never placed far away. This is shared build code, so it holds in every game
// mode: random, custom and missions all place buildings through the same path.
// C++: game_manager.cpp:5743 ORDER_STATE_SELECT_SITE target_grid_x/y.
func TestClampBuildSiteAnchorsOnBuilder(t *testing.T) {
	const bx, by = 10, 10

	// A 2x2: whichever way the cursor points, the footprint anchor must land so the
	// 2x2 still contains the builder cell (10,10).
	cases := []struct {
		name         string
		cx, cy       int
		wantX, wantY int
	}{
		{"lower right quadrant", 11, 11, 10, 10},
		{"upper left quadrant", 9, 9, 9, 9},
		{"upper right quadrant", 11, 9, 10, 9},
		{"lower left quadrant", 9, 11, 9, 10},
		{"on the builder", 10, 10, 10, 10},
		{"far lower right still clamps", 50, 50, 10, 10},
		{"far upper left nudges one cell only", -5, -5, 9, 9},
	}

	for _, c := range cases {
		gotX, gotY := clampBuildSite(bx, by, c.cx, c.cy, 2)
		if gotX != c.wantX || gotY != c.wantY {
			t.Errorf("%s: clampBuildSite cursor (%d,%d) = (%d,%d), want (%d,%d)", c.name, c.cx, c.cy, gotX, gotY, c.wantX, c.wantY)
		}

		// The invariant that makes it faithful: the 2x2 footprint contains the builder.
		if !(gotX <= bx && bx <= gotX+1 && gotY <= by && by <= gotY+1) {
			t.Errorf("%s: footprint (%d,%d)+2 does not overlap the builder (%d,%d)", c.name, gotX, gotY, bx, by)
		}
	}

	// A 1x1 (engineer) always sits on the builder regardless of the cursor.
	if x, y := clampBuildSite(bx, by, 50, 50, 1); x != bx || y != by {
		t.Errorf("1x1 clamp = (%d,%d), want the builder cell (%d,%d)", x, y, bx, by)
	}
}
