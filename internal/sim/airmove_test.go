// airmove_test.go : tests the straight line air move order of movement.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package sim

import (
	"testing"

	"github.com/suglasp/max-go/internal/game"
)

// TestAirOrderFliesStraightNotTheGrid: a plane ordered 10 cells away flies the line in about 4
// ticks a cell, where the old grid route took 8 or more a cell.
func TestAirOrderFliesStraightNotTheGrid(t *testing.T) {
	g := newGame(t, 32, 8)
	plane := addAirUnit(g, 1, 1, 3, true)
	plane.Values.Set(game.AttribSpeed, 30)
	plane.Speed = 30
	plane.SetAngle(2)
	if !g.OrderMove(plane, 11, 3) {
		t.Fatal("the air order was refused")
	}
	ticks := 0
	for g.HasOrder(plane) && ticks < 1000 {
		g.Tick()
		ticks++
	}
	if plane.GridX != 11 || plane.GridY != 3 {
		t.Fatalf("plane at (%d,%d), want (11,3)", plane.GridX, plane.GridY)
	}
	if ticks != 4*10+4 {
		t.Errorf("flight took %d ticks, want %d", ticks, 4*10+4)
	}
}

// TestAirOrderRefusesDestinationHeldByAPlane: a destination another plane holds is refused, as the
// ground route search refuses an occupied cell.
func TestAirOrderRefusesDestinationHeldByAPlane(t *testing.T) {
	g := newGame(t, 16, 8)
	plane := addAirUnit(g, 1, 1, 3, true)
	addAirUnit(g, 2, 6, 3, true)
	if g.OrderMove(plane, 6, 3) {
		t.Error("an air order onto a cell another plane holds must be refused")
	}
	if g.HasOrder(plane) {
		t.Error("a refused order left an order behind")
	}
}

// TestAirHaltMidFlightParksOnACentre: a halt on spotting an enemy parks a flying plane on the cell centre
// ahead and holds the rest of the route.
func TestAirHaltMidFlightParksOnACentre(t *testing.T) {
	g := newGame(t, 32, 8)
	plane := addAirUnit(g, 1, 1, 3, true)
	plane.Values.Set(game.AttribSpeed, 30)
	plane.Speed = 30
	plane.SetAngle(2)
	if !g.OrderMove(plane, 20, 3) {
		t.Fatal("the air order was refused")
	}
	for i := 0; i < 6; i++ {
		g.Tick()
	}
	g.HaltForEnemy(plane)
	for g.HasOrder(plane) {
		g.Tick()
	}
	if plane.X%64 != 32 {
		t.Errorf("the plane parked off a cell centre (x %d)", plane.X)
	}
	held := g.HeldRoute(plane)
	if len(held) != 1 || held[0].To.X != 20 || held[0].To.Y != 3 {
		t.Errorf("held route = %v, want the destination (20,3)", held)
	}
	if plane.GridX >= 20 {
		t.Errorf("the plane flew on to cell %d", plane.GridX)
	}
}

// TestAirOrderOverBudgetQueuesTheRoute: a plane with a budget of 3 flying 10 cells stops after 3 and
// queues the destination for the next turn.
func TestAirOrderOverBudgetQueuesTheRoute(t *testing.T) {
	g := newGame(t, 32, 8)
	plane := addAirUnit(g, 1, 1, 3, true)
	plane.Values.Set(game.AttribSpeed, 3)
	plane.Speed = 3
	plane.SetAngle(2)
	if !g.OrderMove(plane, 11, 3) {
		t.Fatal("the air order was refused")
	}
	g.ResolveMovement()
	if plane.GridX != 4 {
		t.Errorf("plane at cell x %d, want 4 (3 points of flight)", plane.GridX)
	}
	if !g.HasPendingRoute(plane) {
		t.Error("the unfinished flight should be queued for the next turn")
	}
}

// TestHiddenPlaneSettlesInOneTick: a plane the player cannot see resolves its whole flight in one Tick,
// like a hidden ground mover.
func TestHiddenPlaneSettlesInOneTick(t *testing.T) {
	g := newGame(t, 32, 8)
	g.RefreshVisibility()
	plane := addAirUnit(g, 1, 1, 3, true)
	plane.Team = 1
	plane.Values.Set(game.AttribSpeed, 30)
	plane.Speed = 30
	plane.SetAngle(2)
	if !g.OrderMove(plane, 20, 3) {
		t.Fatal("the air order was refused")
	}
	g.Tick()
	if g.HasOrder(plane) || plane.GridX != 20 {
		t.Errorf("the hidden plane should have landed on (20,3) in one Tick: order=%v at (%d,%d)", g.HasOrder(plane), plane.GridX, plane.GridY)
	}
}
