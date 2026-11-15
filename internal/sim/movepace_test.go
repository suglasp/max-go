// movepace_test.go : tests the move order pacing rules (velocity carry, cancel) in movement.go.
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

// TestCancelZeroesVelocity: a move that is cancelled mid route is over, so the unit's ramp
// velocity is zero and the next order pulls out from rest.
func TestCancelZeroesVelocity(t *testing.T) {
	g := newGame(t, 24, 8)
	tank := addConstructor(g, assets.Tank, 0, 2, 4, 12, 30, 0)
	tank.SetAngle(2)
	if !g.OrderMove(tank, 10, 4) {
		t.Fatal("the move order was refused")
	}
	for i := 0; i < 5; i++ {
		g.Tick()
	}
	if tank.Velocity == 0 {
		t.Fatal("the tank should be at speed after 5 ticks")
	}
	g.CancelOrder(tank)
	if tank.Velocity != 0 {
		t.Errorf("velocity after the cancel = %d, want 0", tank.Velocity)
	}
}

// addMover deploys a tank of the team with a speed budget, facing east.
func addMover(g *Game, team, id, speed, gx, gy int) *game.UnitInfo {
	u := game.NewUnitInfo(assets.Tank, team, id, 2)
	u.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribSpeed, speed)
	u.Values.Set(game.AttribHits, 30)
	u.InitState()
	g.Units.Deploy(u, gx, gy)
	return u
}

// TestHiddenMoverSettlesInOneTick: a mover the player cannot see resolves at once, as in the
// reference, while the player's own mover (always in view) takes one order tick at a time.
func TestHiddenMoverSettlesInOneTick(t *testing.T) {
	g := newGame(t, 32, 8)
	g.RefreshVisibility()
	enemy := addMover(g, 1, 2, 60, 2, 4)
	own := addMover(g, 0, 1, 60, 2, 1)
	if !g.OrderMove(enemy, 12, 4) || !g.OrderMove(own, 12, 1) {
		t.Fatal("a move order was refused")
	}
	g.Tick()
	if g.HasOrder(enemy) || enemy.GridX != 12 {
		t.Errorf("the hidden mover should have settled at (12,4) in one Tick: order=%v at (%d,%d)", g.HasOrder(enemy), enemy.GridX, enemy.GridY)
	}
	if !g.HasOrder(own) || own.GridX == 12 {
		t.Errorf("the player's own mover must not skip ahead: order=%v at (%d,%d)", g.HasOrder(own), own.GridX, own.GridY)
	}
}

// TestHiddenMoverStopsWhenSeen: a hidden mover keeps going one cell per pass but stops
// resolving at once the moment it comes into the player's view; from there it moves one
// order tick at a time.
func TestHiddenMoverStopsWhenSeen(t *testing.T) {
	g := newGame(t, 32, 8)
	watcher := addMover(g, 0, 1, 0, 20, 4)
	watcher.Values.Set(game.AttribScan, 3)
	g.RefreshVisibility()
	enemy := addMover(g, 1, 2, 60, 2, 4)
	if !g.OrderMove(enemy, 30, 4) {
		t.Fatal("the move order was refused")
	}
	g.Tick()
	if !g.HasOrder(enemy) {
		t.Fatal("the mover should still be on its way: it came into view")
	}
	if enemy.GridX >= 20 || enemy.GridX < 15 {
		t.Errorf("the mover stopped resolving at x=%d, want it just inside the scan edge (about x=17)", enemy.GridX)
	}
	before := enemy.GridX
	g.Tick()
	if enemy.GridX-before > 1 {
		t.Errorf("in view the mover advanced %d cells in one more Tick, want at most 1", enemy.GridX-before)
	}
}
