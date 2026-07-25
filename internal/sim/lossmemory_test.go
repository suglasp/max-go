// lossmemory_test.go : tests the per team loss memory (recordLoss / RecentLosses).
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package sim

import "testing"

// TestRecordLossMobileOnly: destroying a mobile unit records a loss site at its cell
// for its team, but destroying a building records nothing (a downed structure must not
// mark its own ground as a kill zone for the team's movers).
func TestRecordLossMobileOnly(t *testing.T) {
	g := newGame(t, 16, 16)
	g.State.TurnCounter = 5

	tank := addTank(g, 1, 4, 6, 6)
	g.DestroyUnit(tank)

	losses := g.RecentLosses(0)
	if len(losses) != 1 {
		t.Fatalf("after a mobile-unit death: RecentLosses(0) = %d sites, want 1", len(losses))
	}
	if losses[0].X != 6 || losses[0].Y != 6 || losses[0].Turn != 5 {
		t.Errorf("loss site = (%d,%d turn %d), want (6,6 turn 5)", losses[0].X, losses[0].Y, losses[0].Turn)
	}

	bld := addBuilding(g, 2, 10, 10)
	g.DestroyUnit(bld)
	if n := len(g.RecentLosses(0)); n != 1 {
		t.Errorf("a downed building was recorded as a loss site (%d sites, want 1)", n)
	}
}

// TestRecentLossesPruneStale: a loss older than the retention window is pruned when a
// fresh loss is recorded, so the per team slice stays bounded.
func TestRecentLossesPruneStale(t *testing.T) {
	g := newGame(t, 16, 16)

	g.State.TurnCounter = 1
	g.DestroyUnit(addTank(g, 1, 4, 3, 3)) // old loss at turn 1

	g.State.TurnCounter = 1 + lossSiteRetention + 2 // well past retention
	g.DestroyUnit(addTank(g, 2, 4, 12, 12))         // recording this prunes the stale one

	losses := g.RecentLosses(0)
	if len(losses) != 1 || losses[0].X != 12 || losses[0].Y != 12 {
		t.Fatalf("prune: RecentLosses(0) = %v, want only the fresh (12,12) site", losses)
	}
}
