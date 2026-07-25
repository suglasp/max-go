// endturntimer_test.go : tests the End Turn grace countdown of simultaneous play.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/game"
)

// graceScreen is a simultaneous game mid round: a human team and a computer team, a 180
// second turn budget and a 30 second End Turn grace, with the round already planned.
func graceScreen(t *testing.T) (*Screen, *game.GameState) {
	t.Helper()

	s, g := newClickScreen(t)
	g.State.Teams[1].Type = game.TeamComputer
	s.game.State.PlayMode = game.PlaySimultaneous

	s.SetTurnTimer(180)
	s.SetEndTurnTimer(30)
	s.turnFlow.simPlanned = true

	return s, g.State
}

// TestEndTurnGraceShortensTheTurn verifies that, once opponents finish, the player keeps only the End Turn grace period.
func TestEndTurnGraceShortensTheTurn(t *testing.T) {
	s, st := graceScreen(t)

	// The computer team has NOT finished: the turn keeps its full budget.
	st.Teams[1].FinishedTurn = false
	s.armEndTurnGrace()

	if s.turnFlow.turnSeconds != 180 {
		t.Fatalf("the grace fired while an opponent was still playing: budget = %d, want 180", s.turnFlow.turnSeconds)
	}

	// It finishes: the player now has the grace period, and no more.
	st.Teams[1].FinishedTurn = true
	s.armEndTurnGrace()

	if s.turnFlow.turnSeconds != 30 {
		t.Errorf("after the opponents finished, budget = %d, want the 30s grace", s.turnFlow.turnSeconds)
	}

	// It arms once per round: a second pass must not restart the countdown.
	s.turnFlow.turnSeconds = 5
	s.armEndTurnGrace()

	if s.turnFlow.turnSeconds != 5 {
		t.Errorf("the grace re-armed inside the same round: budget = %d, want the running 5", s.turnFlow.turnSeconds)
	}

	// A new round restores the full budget and re-arms the grace.
	s.resetEndTurnGrace()

	if s.turnFlow.turnSeconds != 180 || s.turnFlow.graceArmed {
		t.Errorf("a new round did not restore the turn: budget = %d, armed = %v", s.turnFlow.turnSeconds, s.turnFlow.graceArmed)
	}
}

// TestEndTurnGraceNeverExtendsTheTurn: a grace longer than the time already left leaves
// the turn alone (the original takes the SMALLER of the two).
func TestEndTurnGraceNeverExtendsTheTurn(t *testing.T) {
	s, st := graceScreen(t)
	st.Teams[1].FinishedTurn = true

	s.SetTurnTimer(20) // only 20s left on a 30s grace
	s.armEndTurnGrace()

	if s.turnFlow.turnSeconds != 20 {
		t.Errorf("the grace EXTENDED the turn: budget = %d, want the 20 that was left", s.turnFlow.turnSeconds)
	}
}

// TestEndTurnGraceIsSimultaneousOnly: in turn based play nobody else is moving while you
// play, so the grace has no meaning and must not touch the clock.
func TestEndTurnGraceIsSimultaneousOnly(t *testing.T) {
	s, st := graceScreen(t)
	st.Teams[1].FinishedTurn = true
	s.game.State.PlayMode = game.PlayTurnBased

	s.armEndTurnGrace()

	if s.turnFlow.turnSeconds != 180 {
		t.Errorf("the grace fired in a turn based game: budget = %d, want 180", s.turnFlow.turnSeconds)
	}
}

// TestEndTurnGraceWaitsForOpponentUnits: a computer team is flagged finished the moment it
// PLANS its round, but its units are still driving. The player must not lose the turn
// while an opponent is still moving on the board.
func TestEndTurnGraceWaitsForOpponentUnits(t *testing.T) {
	s, st := graceScreen(t)
	st.Teams[1].FinishedTurn = true

	enemy := mkUnit(s.game, 0, 5, 4, 4, game.FlagMobileLandUnit|game.FlagSelectable)
	enemy.Team = 1
	enemy.Values.Set(game.AttribSpeed, 8)
	enemy.Speed = 8

	if !s.game.OrderMove(enemy, 8, 8) {
		t.Fatal("could not put the enemy unit under a move order")
	}

	if !s.opponentsActing() {
		t.Fatal("an opponent unit under a move order does not count as acting")
	}

	s.armEndTurnGrace()

	if s.turnFlow.turnSeconds != 180 {
		t.Errorf("the grace fired while an enemy was still driving: budget = %d, want 180", s.turnFlow.turnSeconds)
	}
}
