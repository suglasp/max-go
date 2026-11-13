// enemyhalt_test.go : pins halt on contact applying to the computer's units, not just yours.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"strings"
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
)

// newHaltScreen builds a screen with a human team 0 and a computer team 1 on open land.
func newHaltScreen(t *testing.T) (*Screen, *sim.Game) {
	t.Helper()

	s, g := newTestScreen(t, 32, 1)
	g.State.ActiveTeam = 1
	s.enemyHalt = true

	return s, g
}

// TestComputerMoversHaltOnContact verifies that halt on contact applies to computer teams as well as the player's team (access.cpp:827).
func TestComputerMoversHaltOnContact(t *testing.T) {
	s, g := newHaltScreen(t)

	ai := mkTank(g.Units, 1, 1, 2, 2, tankStats{angle: 2, speed: 20, scan: 4, hits: 30}) // the computer's tank, scan 4
	mkTank(g.Units, 0, 2, 14, 2, tankStats{angle: 2, speed: 8, scan: 4, hits: 30})       // the player's tank waiting up the road
	g.BeginTeamTurn(1)

	if !g.OrderMove(ai, 24, 2) { // a drive right past the ambush
		t.Fatal("the computer's move order was rejected")
	}

	for i := 0; i < 400 && g.ActiveOrders() > 0; i++ {
		g.Tick()
		s.haltMoversOnEnemy()
	}

	if ai.GridX >= 14 {
		t.Errorf("the computer's tank drove to x=%d without stopping: it should halt when the player's tank comes into scan range", ai.GridX)
	}
}

// TestHaltingTeamCoversPlayersAndComputers: every team that actually plays halts; an empty
// slot never does.
func TestHaltingTeamCoversPlayersAndComputers(t *testing.T) {
	s, g := newHaltScreen(t)

	if !s.haltingTeam(0) || !s.haltingTeam(1) {
		t.Error("a human team and a computer team must both halt on contact")
	}

	if s.haltingTeam(2) {
		t.Error("an empty team slot has nothing to halt")
	}

	if s.haltingTeam(-1) || s.haltingTeam(game.TeamCount) {
		t.Error("an out of range team must not halt (it would index out of bounds)")
	}

	_ = g
}

// TestHaltTellsThePlayer: a unit that stops short of where it was sent says why. Without the
// message a column halting in open ground reads as the move having failed.
func TestHaltTellsThePlayer(t *testing.T) {
	s, g := newHaltScreen(t)
	g.State.ActiveTeam = 0

	mover := mkTank(g.Units, 0, 1, 2, 2, tankStats{angle: 2, speed: 12, scan: 4, hits: 30}) // the player's
	_ = mkTank(g.Units, 1, 2, 12, 2, tankStats{angle: 2, speed: 0, scan: 0, hits: 30})      // a computer unit waiting down the road

	g.BeginTeamTurn(0)
	if !g.OrderMove(mover, 20, 2) {
		t.Fatal("the move order was rejected")
	}

	for i := 0; i < 400 && g.ActiveOrders() > 0; i++ {
		g.Tick()
		s.haltMoversOnEnemy()
	}

	if len(g.HeldRoute(mover)) == 0 {
		t.Fatal("the unit should have halted and held its route")
	}

	found := false
	for _, e := range s.banner.msgLog {
		if strings.Contains(e.text, "halted") {
			found = true
		}
	}
	if !found {
		t.Error("the player was not told why their unit stopped")
	}
}

// TestPopupOffersContinueOnAHeldRoute: the halted unit's popup leads with Continue, and taking
// it sends the unit on toward the destination it was holding.
func TestPopupOffersContinueOnAHeldRoute(t *testing.T) {
	s, g := newHaltScreen(t)
	g.State.ActiveTeam = 0

	mover := mkTank(g.Units, 0, 1, 2, 2, tankStats{angle: 2, speed: 12, scan: 4, hits: 30})

	g.BeginTeamTurn(0)
	g.OrderMove(mover, 12, 2)
	g.HaltForEnemy(mover)

	items := s.popupItemsFor(mover)
	if len(items) == 0 || items[0].label != "Continue" {
		t.Fatalf("the halted unit's popup should lead with Continue, got %v", items)
	}

	items[0].action()

	if len(g.HeldRoute(mover)) != 0 {
		t.Error("Continue should consume the hold")
	}
	g.ResolveMovement()
	if mover.GridX != 12 || mover.GridY != 2 {
		t.Errorf("the unit is at (%d,%d) after Continue, want the held destination (12,2)", mover.GridX, mover.GridY)
	}
}

// driveWithHaltChecks ticks every move to its end, running the halt check after each tick.
func driveWithHaltChecks(s *Screen, g *sim.Game) {
	for i := 0; i < 400 && g.ActiveOrders() > 0; i++ {
		g.Tick()
		s.haltMoversOnEnemy()
	}
}

// TestNoHaltForAnEnemyTheTeamAlreadySees is the re-halt crawl: a friendly spotter already sees
// the enemy Tank, so the mover driving into its scan range reveals nothing and must not halt.
func TestNoHaltForAnEnemyTheTeamAlreadySees(t *testing.T) {
	s, g := newHaltScreen(t)
	g.State.ActiveTeam = 0

	mover := mkTank(g.Units, 0, 1, 2, 2, tankStats{angle: 2, speed: 20, scan: 4, hits: 30})
	_ = mkTank(g.Units, 0, 2, 12, 6, tankStats{angle: 2, speed: 0, scan: 8, hits: 30}) // a parked friendly spotter that sees the enemy
	_ = mkTank(g.Units, 1, 3, 12, 2, tankStats{angle: 2, speed: 0, scan: 0, hits: 30}) // the enemy Tank, in plain sight of the spotter

	g.BeginTeamTurn(0)
	if !g.OrderMove(mover, 20, 2) {
		t.Fatal("the move order was rejected")
	}
	driveWithHaltChecks(s, g)

	if len(g.HeldRoute(mover)) != 0 || mover.GridX != 20 {
		t.Errorf("the mover halted at x=%d for an enemy its team already saw", mover.GridX)
	}
}

// TestNoHaltForAnEnemyThatCannotShootTheMover: a newly revealed enemy with no weapon (a
// Surveyor) does not stop a Tank.
func TestNoHaltForAnEnemyThatCannotShootTheMover(t *testing.T) {
	s, g := newHaltScreen(t)
	g.State.ActiveTeam = 0

	mover := mkTank(g.Units, 0, 1, 2, 2, tankStats{angle: 2, speed: 20, scan: 4, hits: 30})
	surveyor := mkTank(g.Units, 1, 2, 12, 2, tankStats{angle: 2, speed: 0, scan: 0, hits: 30})
	surveyor.Type = assets.Surveyor

	g.BeginTeamTurn(0)
	if !g.OrderMove(mover, 20, 2) {
		t.Fatal("the move order was rejected")
	}
	driveWithHaltChecks(s, g)

	if len(g.HeldRoute(mover)) != 0 || mover.GridX != 20 {
		t.Errorf("the mover halted at x=%d for an unarmed enemy", mover.GridX)
	}
}

// TestResumedUnitDrivesOnPastAKnownEnemy: the mover halts once on a newly revealed Tank, drives
// on at the next turn start, and does not halt again for a second Tank its team already sees.
func TestResumedUnitDrivesOnPastAKnownEnemy(t *testing.T) {
	s, g := newHaltScreen(t)
	g.State.ActiveTeam = 0

	mover := mkTank(g.Units, 0, 1, 2, 2, tankStats{angle: 2, speed: 12, scan: 4, hits: 30})
	_ = mkTank(g.Units, 1, 2, 9, 5, tankStats{angle: 2, speed: 0, scan: 0, hits: 30})  // first enemy, unseen until the mover nears it
	_ = mkTank(g.Units, 0, 3, 18, 9, tankStats{angle: 2, speed: 0, scan: 6, hits: 30}) // a friendly spotter that sees only the second enemy
	_ = mkTank(g.Units, 1, 4, 16, 4, tankStats{angle: 2, speed: 0, scan: 0, hits: 30}) // second enemy beside the road, in plain sight of the spotter

	g.BeginTeamTurn(0)
	if !g.OrderMove(mover, 24, 2) {
		t.Fatal("the move order was rejected")
	}
	driveWithHaltChecks(s, g)

	if len(g.HeldRoute(mover)) == 0 {
		t.Fatalf("the mover did not halt on the first, newly revealed enemy (x=%d)", mover.GridX)
	}

	for turn := 0; turn < 3 && mover.GridX != 24; turn++ {
		g.BeginTeamTurn(0)
		driveWithHaltChecks(s, g)
		if len(g.HeldRoute(mover)) != 0 {
			t.Fatalf("turn %d: the resumed mover halted again at x=%d for an enemy its team already saw", turn+1, mover.GridX)
		}
	}

	if mover.GridX != 24 {
		t.Errorf("the resumed mover ended at x=%d, want 24", mover.GridX)
	}
}
