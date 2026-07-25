// buildexit_exterior_test.go : tests build-ready exit selection outside the completed footprint.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package sim

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// wall1x1 deploys a solid 1x1 building the movement query treats as impassable.
func wall1x1(g *Game, id, gx, gy int) *game.UnitInfo {
	u := game.NewUnitInfo(assets.Research, 0, id, 0)
	u.Flags = game.FlagStationary | game.FlagSelectable
	u.Values = &game.UnitValues{}
	g.Units.Deploy(u, gx, gy)
	return u
}

// TestExitLeavesClosedGroup verifies that the AI skips a nearby enclosed pocket and sends the builder to a reachable exterior cell.
func TestExitLeavesClosedGroup(t *testing.T) {
	g := newGame(t, 20, 20)
	con := constructorAt(g, 0, 2, 2) // only its type/flags matter to the exit query

	const siteX, siteY = 10, 10 // a 1x1 finished footprint the builder leaves behind
	// Wall every cell around the site except the north cell (10,9), then seal that cell
	// too, so (10,9) is an empty pocket with no way out.
	id := 100
	for _, d := range [][2]int{
		{-1, -1}, {1, -1}, {-1, 0}, {1, 0}, {-1, 1}, {0, 1}, {1, 1}, // the ring, minus (10,9)
		{-1, -2}, {0, -2}, {1, -2}, // seal the pocket's far side
	} {
		wall1x1(g, id, siteX+d[0], siteY+d[1])
		id++
	}

	blocked := footprintCells(siteX, siteY, 1)
	if g.connectsToOpen(con, siteX, siteY-1, blocked) {
		t.Fatal("test setup: (10,9) should be a sealed pocket, not open")
	}

	x, y, ok := g.findReachableSpot(con, siteX, siteY, 1)
	if !ok {
		t.Fatal("no exit spot found at all")
	}
	if x == siteX && y == siteY-1 {
		t.Error("exit chose the interior pocket (10,9); it must exit to the outside")
	}
	if !g.connectsToOpen(con, x, y, blocked) {
		t.Errorf("exit cell (%d,%d) does not connect to open ground", x, y)
	}
}
