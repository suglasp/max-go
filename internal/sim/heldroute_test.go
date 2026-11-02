// heldroute_test.go : pins the route a unit keeps when it halts on spotting an enemy.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package sim

import (
	"testing"

	"github.com/suglasp/max-go/internal/game"
)

// TestHaltForEnemyHoldsTheRoute checks that a halted unit stops where it stands but keeps
// the remainder of its drive, and that the next turn start drives it on by itself.
func TestHaltForEnemyHoldsTheRoute(t *testing.T) {
	g := newGame(t, 16, 4)
	tank := addTank(g, 1, 6, 1, 1)

	g.BeginTeamTurn(0)
	if !g.OrderMove(tank, 7, 1) {
		t.Fatal("OrderMove should route across open land")
	}

	g.HaltForEnemy(tank)

	if g.ActiveOrders() != 0 {
		t.Errorf("active orders = %d, want 0: a halted unit stops moving", g.ActiveOrders())
	}
	held := g.HeldRoute(tank)
	if len(held) != 6 {
		t.Fatalf("held route = %d steps, want the 6 it had left to drive", len(held))
	}
	if dest := held[len(held)-1].To; dest.X != 7 || dest.Y != 1 {
		t.Errorf("held route ends at (%d,%d), want the original (7,1)", dest.X, dest.Y)
	}
	if len(g.PendingRoutes()) != 0 {
		t.Error("a held route must not also be a pending one, or the turn start drives it twice")
	}

	// The rest of this turn it stands; the next turn start drives it on by itself.
	g.ResolveMovement()
	if tank.GridX != 1 || tank.GridY != 1 {
		t.Errorf("tank moved to (%d,%d) in the halt turn; it waits at (1,1) until the next turn", tank.GridX, tank.GridY)
	}
	g.BeginTeamTurn(0)
	g.ResolveMovement()
	if tank.GridX != 7 || tank.GridY != 1 {
		t.Errorf("tank at (%d,%d) after the next turn start, want it to drive on to (7,1)", tank.GridX, tank.GridY)
	}
	if len(g.HeldRoute(tank)) != 0 {
		t.Error("the turn start must consume the hold")
	}
}

// TestContinueHeldRouteInTheHaltTurn checks that Continue presses on at once, in the same
// turn as the halt, re-laying the route from where the unit stands.
func TestContinueHeldRouteInTheHaltTurn(t *testing.T) {
	g := newGame(t, 16, 4)
	tank := addTank(g, 1, 6, 1, 1)

	g.BeginTeamTurn(0)
	if !g.OrderMove(tank, 7, 1) {
		t.Fatal("OrderMove should route across open land")
	}
	g.HaltForEnemy(tank)

	if !g.ContinueHeldRoute(tank) {
		t.Fatal("ContinueHeldRoute should re-path to the held destination")
	}
	if len(g.HeldRoute(tank)) != 0 {
		t.Error("continuing must consume the hold")
	}
	g.ResolveMovement()
	if tank.GridX != 7 || tank.GridY != 1 {
		t.Errorf("tank at (%d,%d) after continuing, want (7,1)", tank.GridX, tank.GridY)
	}
	// Nothing left to continue: a second ask reports so rather than re-driving.
	if g.ContinueHeldRoute(tank) {
		t.Error("ContinueHeldRoute should report false when the unit holds no route")
	}
}

// TestHaltForEnemyHoldsAShelvedRoute checks the between turns case: a unit that ran out of
// movement points holds the SHELVED remainder, the shelf is emptied, and the turn start
// drives the held route on once.
func TestHaltForEnemyHoldsAShelvedRoute(t *testing.T) {
	g := newGame(t, 16, 4)
	tank := addTank(g, 1, 2, 1, 1) // two cells a turn, so a six cell drive spills over

	g.BeginTeamTurn(0)
	g.OrderMove(tank, 7, 1)
	g.ResolveMovement()

	if tank.GridX != 3 {
		t.Fatalf("tank at x=%d, want 3 (two cells on its budget)", tank.GridX)
	}
	if len(g.PendingRoutes()) != 1 {
		t.Fatalf("want the remainder shelved as a pending route, got %d", len(g.PendingRoutes()))
	}

	g.HaltForEnemy(tank)

	held := g.HeldRoute(tank)
	if len(held) != 4 {
		t.Fatalf("held route = %d steps, want the 4 cells still to drive", len(held))
	}
	if len(g.PendingRoutes()) != 0 {
		t.Fatal("the shelved route must move to the hold, not stay pending")
	}

	g.BeginTeamTurn(0)
	g.ResolveMovement()
	if tank.GridX != 5 {
		t.Errorf("tank at x=%d after the turn start, want x=5 (two more cells toward the held destination)", tank.GridX)
	}
	if len(g.HeldRoute(tank)) != 0 {
		t.Error("the turn start must consume the hold")
	}
}

// TestHeldRouteOfABuilderAtWorkIsDropped checks that the turn start never drives a unit
// parked build ready off its building: the stale held route is dropped instead.
func TestHeldRouteOfABuilderAtWorkIsDropped(t *testing.T) {
	g := newGame(t, 16, 4)
	tank := addTank(g, 1, 6, 1, 1)

	g.BeginTeamTurn(0)
	g.OrderMove(tank, 7, 1)
	g.HaltForEnemy(tank)
	g.buildReady = map[*game.UnitInfo]*BuildReady{tank: {}}

	g.BeginTeamTurn(0)
	g.ResolveMovement()
	if tank.GridX != 1 {
		t.Errorf("a unit parked build ready drove to x=%d; it must stay at x=1", tank.GridX)
	}
	if len(g.HeldRoute(tank)) != 0 {
		t.Error("the stale held route should be dropped")
	}
}

// TestHeldRouteYieldsToANewOrder checks that any fresh order supersedes the hold, so a
// stale route can never resurface after the player has moved on.
func TestHeldRouteYieldsToANewOrder(t *testing.T) {
	g := newGame(t, 16, 6)

	// A new move order takes over.
	tank := addTank(g, 1, 6, 1, 1)
	g.BeginTeamTurn(0)
	g.OrderMove(tank, 7, 1)
	g.HaltForEnemy(tank)
	g.OrderMove(tank, 1, 4)
	if len(g.HeldRoute(tank)) != 0 {
		t.Error("a new move order must clear the held route")
	}
	g.ResolveMovement()
	if tank.GridX != 1 || tank.GridY != 4 {
		t.Errorf("tank at (%d,%d), want the new destination (1,4)", tank.GridX, tank.GridY)
	}

	// So does a plain cancel: the player told it to stand down.
	other := addTank(g, 2, 6, 10, 1)
	g.OrderMove(other, 14, 1)
	g.HaltForEnemy(other)
	if len(g.HeldRoute(other)) == 0 {
		t.Fatal("the halt should have held a route")
	}
	g.CancelOrder(other)
	if len(g.HeldRoute(other)) != 0 {
		t.Error("CancelOrder must clear the held route")
	}
	g.BeginTeamTurn(0)
	g.ResolveMovement()
	if other.GridX != 10 {
		t.Errorf("a cancelled unit drove on to x=%d; it should stand at x=10", other.GridX)
	}
}

// TestHaltMidCellFinishesTheCrossing checks that a unit halted halfway into a cell drives
// on to that cell's centre and holds the rest, instead of freezing between two cells.
func TestHaltMidCellFinishesTheCrossing(t *testing.T) {
	g := newGame(t, 16, 4)
	tank := addTank(g, 1, 6, 1, 1)

	g.BeginTeamTurn(0)
	if !g.OrderMove(tank, 7, 1) {
		t.Fatal("OrderMove should route across open land")
	}
	for i := 0; i < 200 && tank.Moved == 0; i++ {
		g.Tick()
	}
	if tank.Moved == 0 {
		t.Fatal("the tank never started crossing into the next cell")
	}

	g.HaltForEnemy(tank)
	g.ResolveMovement()

	if tank.GridX != 2 || tank.GridY != 1 {
		t.Errorf("tank parked at (%d,%d), want the cell it was crossing into (2,1)", tank.GridX, tank.GridY)
	}
	if tank.Moved != 0 || tank.X != 2*64+32 || tank.Y != 1*64+32 {
		t.Errorf("tank left between cells: Moved=%d pixel=(%d,%d), want the (2,1) centre", tank.Moved, tank.X, tank.Y)
	}
	held := g.HeldRoute(tank)
	if len(held) != 5 {
		t.Fatalf("held route = %d steps, want the 5 after the crossing", len(held))
	}
	if dest := held[len(held)-1].To; dest.X != 7 || dest.Y != 1 {
		t.Errorf("held route ends at (%d,%d), want (7,1)", dest.X, dest.Y)
	}
}

// TestAnyNewRouteDropsTheHeldOne checks that the routes Auto Survey, the AI and a click
// with no moves left lay also drop a held route, so no stale arrows stay behind and the
// turn start never drives the old one.
func TestAnyNewRouteDropsTheHeldOne(t *testing.T) {
	g := newGame(t, 16, 6)
	tank := addTank(g, 1, 6, 1, 1)

	g.BeginTeamTurn(0)
	g.OrderMove(tank, 7, 1)
	g.HaltForEnemy(tank)
	if !g.OrderMoveAvoiding(tank, 1, 4, nil) {
		t.Fatal("OrderMoveAvoiding should route to (1,4)")
	}
	if len(g.HeldRoute(tank)) != 0 {
		t.Error("OrderMoveAvoiding must drop the held route")
	}

	other := addTank(g, 2, 6, 10, 1)
	g.OrderMove(other, 14, 1)
	g.HaltForEnemy(other)
	if !g.QueueMove(other, 10, 4) {
		t.Fatal("QueueMove should route to (10,4)")
	}
	if len(g.HeldRoute(other)) != 0 {
		t.Error("QueueMove must drop the held route")
	}

	g.BeginTeamTurn(0)
	g.ResolveMovement()
	if other.GridX != 10 || other.GridY != 4 {
		t.Errorf("queued tank at (%d,%d), want its new destination (10,4), not the old held one", other.GridX, other.GridY)
	}
}
