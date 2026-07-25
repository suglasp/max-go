// move_reservation_test.go : two units entering the same cell from opposite sides must not stack on it.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package sim

import (
	"testing"

	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/paths"
	"github.com/suglasp/max-go/internal/units"
)

// TestChokepointDoesNotStackUnits checks that only one same-team unit can reserve and enter a shared chokepoint on the same tick; the other waits.
//
// Steps are hand built (bypassing pathfinding) so both orders start their
// entry check in lockstep on tick one - the exact race window: a unit is
// invisible to another's occupancy check for the whole multi-frame slide
// across a tile, not just registered on arrival. Fails without the
// reservation fix (AddAt/Arrive/ReleaseReservation) - both commits pass, both
// units end up in the destination cell's bucket.
func TestChokepointDoesNotStackUnits(t *testing.T) {
	g := newGame(t, 10, 5)

	// A approaches the gap from the northwest, B from the southwest - same
	// diagonal distance, same speed, no turning delay, so both reach their
	// u.Moved==0 entry commit for the SHARED gap cell (5,2) on the identical
	// tick. Past the gap they diverge back to their own lanes, so this is a
	// merge-then-continue chokepoint (like the reported bridge), not a head
	// on swap - only the gap cell is ever actually contested.
	unitA := game.NewUnitInfo(assets.Tank, 0, 1, 3) // 3 = SE, matches its first step
	unitA.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	unitA.Values = &game.UnitValues{}
	unitA.Values.Set(game.AttribSpeed, 20)
	g.Units.Deploy(unitA, 4, 1)
	unitA.ResetMovement()

	unitB := game.NewUnitInfo(assets.Tank, 0, 2, 1) // 1 = NE, matches its first step
	unitB.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	unitB.Values = &game.UnitValues{}
	unitB.Values.Set(game.AttribSpeed, 20)
	g.Units.Deploy(unitB, 4, 3)
	unitB.ResetMovement()

	d := g.unitDesc(unitA) // both are plain land Tanks: one descriptor serves both
	cost := paths.GridCost(g.Grid, d, 0, access.SameClassBlocks)

	stepsA := []paths.Step{{To: game.Point{X: 5, Y: 2}, Dir: 3}, {To: game.Point{X: 6, Y: 1}, Dir: 1}}
	stepsB := []paths.Step{{To: game.Point{X: 5, Y: 2}, Dir: 1}, {To: game.Point{X: 6, Y: 3}, Dir: 3}}
	g.orders = append(g.orders,
		units.NewMoveOrder(g.Units, unitA, stepsA, cost, false),
		units.NewMoveOrder(g.Units, unitB, stepsB, cost, false),
	)

	// Tick by hand (not ResolveMovement) so the shared gap cell's occupant
	// count can be checked EVERY frame - a transient mid-route collision at
	// (5,2) would otherwise be invisible once both units move on afterward
	// and the final-position check alone would not catch it.
	for i := 0; i < 500 && g.ActiveOrders() > 0; i++ {
		g.Tick()
		if n := len(g.Units.Cells.At(5, 2)); n > 1 {
			t.Fatalf("tick %d: %d units stacked on the gap cell (5,2)", i, n)
		}
	}
	if g.ActiveOrders() > 0 {
		t.Fatalf("orders never settled within 500 ticks")
	}

	if unitA.GridX == unitB.GridX && unitA.GridY == unitB.GridY {
		t.Fatalf("both units share cell (%d,%d)", unitA.GridX, unitA.GridY)
	}
	if unitA.GridX != 6 || unitA.GridY != 1 {
		t.Errorf("unit A stopped at (%d,%d), want (6,1)", unitA.GridX, unitA.GridY)
	}
	if unitB.GridX != 6 || unitB.GridY != 3 {
		t.Errorf("unit B stopped at (%d,%d), want (6,3)", unitB.GridX, unitB.GridY)
	}
}
