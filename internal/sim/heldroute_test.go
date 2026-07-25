// heldroute_test.go : pins the route a unit keeps when it halts on spotting an enemy.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package sim

import "testing"

// TestHaltForEnemyHoldsTheRoute checks that a halted unit stops where it stands but keeps
// the remainder of its drive, and that nothing but the player sets it going again.
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
		t.Error("a held route must not be a pending one, or the next turn start drives the unit back into the enemy")
	}

	// The trap: the unit must sit still through the turn boundary. Only the player
	// decides whether to press on.
	g.BeginTeamTurn(0)
	g.ResolveMovement()
	if tank.GridX != 1 || tank.GridY != 1 {
		t.Errorf("tank moved to (%d,%d) on its own; a halted unit waits at (1,1)", tank.GridX, tank.GridY)
	}
	if len(g.HeldRoute(tank)) == 0 {
		t.Error("the turn start dropped the held route; it must survive until the player orders something")
	}

	// The player presses on: the route is re-laid from where the unit stands.
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
// movement points holds the SHELVED remainder, and the shelf is emptied so the turn start
// cannot drive it.
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
	if tank.GridX != 3 {
		t.Errorf("tank drove on to x=%d by itself; a halted unit waits at x=3", tank.GridX)
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
