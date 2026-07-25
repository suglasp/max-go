// v24_test.go : save format v24 (held routes) round trip tests for the saveload package.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package saveload

import (
	"bytes"
	"testing"
)

// TestSaveLoadV24HeldRoute checks that a route held by an enemy halt survives a save and a
// load, and that it comes back HELD rather than pending: a reloaded game must not drive the
// unit back into the enemy it stopped for at the next turn start.
func TestSaveLoadV24HeldRoute(t *testing.T) {
	g := newTestGame(t)
	tank := g.Units.ByID.Get(1) // the bench tank at (3,4)

	g.BeginTeamTurn(0)
	if !g.OrderMove(tank, 11, 4) {
		t.Fatal("OrderMove should route across the test map")
	}
	g.HaltForEnemy(tank)
	if len(g.HeldRoute(tank)) == 0 {
		t.Fatal("the halt should have held a route to save")
	}

	var buf bytes.Buffer
	if err := Save(&buf, Capture(g, "GREEN_1.WRL")); err != nil {
		t.Fatalf("save: %v", err)
	}

	snap, err := Load(&buf)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(snap.HeldMoves) != 1 {
		t.Fatalf("snapshot HeldMoves = %d, want the tank's held route", len(snap.HeldMoves))
	}
	if h := snap.HeldMoves[0]; h.DestX != 11 || h.DestY != 4 {
		t.Errorf("held destination = (%d,%d), want (11,4)", h.DestX, h.DestY)
	}

	g2 := snap.Restore(landMap(16, 16))
	tank2 := g2.Units.ByID.Get(tank.ID)
	if tank2 == nil {
		t.Fatal("the tank did not come back from the save")
	}

	if len(g2.HeldRoute(tank2)) == 0 {
		t.Error("the restored tank lost its held route; the destination must survive a reload")
	}
	if len(g2.PendingRoutes()) != 0 {
		t.Error("a held route must not come back as a pending one, or the turn start resumes it")
	}

	g2.BeginTeamTurn(0)
	g2.ResolveMovement()
	if tank2.GridX != 3 || tank2.GridY != 4 {
		t.Errorf("the restored tank drove to (%d,%d) on its own; it should still wait at (3,4)", tank2.GridX, tank2.GridY)
	}

	// And the player can still press on with it.
	if !g2.ContinueHeldRoute(tank2) {
		t.Fatal("ContinueHeldRoute should re-path to the restored destination")
	}
	g2.ResolveMovement()
	if tank2.GridX != 11 || tank2.GridY != 4 {
		t.Errorf("tank at (%d,%d) after continuing, want the held destination (11,4)", tank2.GridX, tank2.GridY)
	}
}
