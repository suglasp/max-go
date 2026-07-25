// buildexit_cellhash_test.go : tests cell hash consistency after builder exits a build site.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package sim

import (
	"testing"

	"github.com/suglasp/max-go/internal/economy"
)

// TestBuildExitClearsStaleCellHash verifies that a constructor leaving a completed 2x2 site is removed from all four footprint cells.
// The hash must remove the posed 2x2 footprint before the builder returns to its 1x1 mobile footprint.
func TestBuildExitClearsStaleCellHash(t *testing.T) {
	g := newGame(t, 24, 24)
	g.Resources = economy.NewResourceMap(24, 24)

	c := constructorAt(g, 0, 7, 8) // adjacent to the site's west edge
	siteX, siteY := 8, 8           // 2x2 footprint (8,8)..(9,9)
	g.StartBuild(c, miningProduct(), siteX, siteY, 1)
	g.BeginTeamTurn(0) // one refilled turn finishes the one turn build

	if !g.IsBuildReady(c) {
		t.Fatal("constructor is not build-ready after finishing the structure")
	}
	markers := g.ExitMarkers(c)
	if len(markers) == 0 {
		t.Fatal("no exit markers offered around the finished footprint")
	}
	m := markers[0]
	if !g.ExitBuildSite(c, m[0], m[1]) {
		t.Fatalf("ExitBuildSite rejected a valid marker (%d,%d)", m[0], m[1])
	}
	g.ResolveMovement() // animate the drive off to completion

	// The builder has driven off: it must be hashed ONLY at its new cell, never in any
	// of the four old build site footprint cells.
	for dy := 0; dy < 2; dy++ {
		for dx := 0; dx < 2; dx++ {
			cx, cy := siteX+dx, siteY+dy
			for _, u := range g.Units.Cells.At(cx, cy) {
				if u == c {
					t.Errorf("constructor still stale-hashed at old footprint cell (%d,%d)", cx, cy)
				}
			}
		}
	}
	// Sanity: it IS hashed at its actual resting cell.
	atNew := false
	for _, u := range g.Units.Cells.At(c.GridX, c.GridY) {
		if u == c {
			atNew = true
		}
	}
	if !atNew {
		t.Errorf("constructor not hashed at its new cell (%d,%d)", c.GridX, c.GridY)
	}
}
