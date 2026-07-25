// queuemove_test.go : tests queued move order execution and route follow behavior.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package sim

import (
	"testing"

	"github.com/suglasp/max-go/internal/game"
)

// TestQueueMoveShelvesFullRoute verifies that QueueMove plans and stores a full route when the unit has no movement points.
// The screen can draw the stored route, which resumes after the next budget refill.
func TestQueueMoveShelvesFullRoute(t *testing.T) {
	g := newGame(t, 16, 4)
	tank := addTank(g, 1, 6, 1, 1) // placed at (1,1); ask to route to (7,1)

	// An off map (unreachable) destination queues nothing and reports failure.
	if g.QueueMove(tank, 99, 99) {
		t.Fatal("QueueMove to an unreachable cell should return false")
	}
	if len(g.PendingRoutes()) != 0 {
		t.Fatal("a failed QueueMove must not queue anything")
	}

	// A reachable destination shelves the FULL route without moving the unit.
	if !g.QueueMove(tank, 7, 1) {
		t.Fatal("QueueMove should plan a route on open land")
	}
	if tank.GridX != 1 || tank.GridY != 1 {
		t.Errorf("QueueMove must not advance the unit, at (%d,%d) want (1,1)", tank.GridX, tank.GridY)
	}
	if tank.Order != game.OrderMove {
		t.Errorf("queued unit order = %v, want OrderMove (reads as en route)", tank.Order)
	}
	if g.ActiveOrders() != 0 {
		t.Errorf("QueueMove must not start a live order, ActiveOrders=%d", g.ActiveOrders())
	}
	pr := g.PendingRoutes()
	if len(pr) != 1 || pr[0].Unit != tank {
		t.Fatalf("want 1 pending route for the tank, got %d", len(pr))
	}
	// The shelved route is the whole planned path (6 cells east), not a single dot,
	// so drawAllGroundPaths renders the full path with turn boundary markers.
	if n := len(pr[0].Steps); n != 6 {
		t.Errorf("pending route steps = %d, want 6 (the full planned path)", n)
	}
	if last := pr[0].Steps[len(pr[0].Steps)-1].To; last.X != 7 || last.Y != 1 {
		t.Errorf("route ends at (%d,%d), want (7,1)", last.X, last.Y)
	}

	// Next turn the queued route auto resumes (resumePending issues it again) and the
	// tank, budget refilled, drives to the destination.
	g.BeginTeamTurn(0)
	g.ResolveMovement()
	if tank.GridX != 7 {
		t.Errorf("after resume, tank x=%d, want 7 (queued route driven next turn)", tank.GridX)
	}
}
