// endturntimer_test.go : tests the End Turn grace countdown of simultaneous play.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/platform"
)

// graceScreen is a simultaneous game mid round: a human team and a computer team, a 180
// second turn budget and a 30 second End Turn grace, with the round already planned.
func graceScreen(t *testing.T) (*Screen, *game.GameState) {
	t.Helper()

	s, g := newTestScreen(t, 16)
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

	enemy := mkUnit(s.game, 0, 0, 5, 4, 4, game.FlagMobileLandUnit|game.FlagSelectable)
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

// TestOptionsCloseKeepsTheRunningClock checks the L1 rule: closing the in game Options never restarts the running
// turn clock. An End Turn Time change applies to the next countdown, a turn timer change to the next round, and an
// unchanged setting leaves the game's own timers (a mission's or a custom game's) alone.
func TestOptionsCloseKeepsTheRunningClock(t *testing.T) {
	s, _ := graceScreen(t)
	s.ApplyPrefs(LivePrefs{TurnTimer: 180, EndTurn: 40}) // game creation
	s.SetTurnTimer(240)                                  // the custom game's own turn timer
	s.SetEndTurnTimer(30)                                // and its own End Turn Time
	s.turnFlow.turnStart -= 100 * 1000                   // 100 seconds into the turn
	start, secs := s.turnFlow.turnStart, s.turnFlow.turnSeconds

	// Options closed with only End Turn Time changed (40 -> 20).
	s.ApplyPrefs(LivePrefs{TurnTimer: 180, EndTurn: 20})
	if s.turnFlow.turnStart != start || s.turnFlow.turnSeconds != secs {
		t.Errorf("the running clock restarted: start %d -> %d, seconds %d -> %d", start, s.turnFlow.turnStart, secs, s.turnFlow.turnSeconds)
	}
	if s.turnFlow.fullTurnSeconds != 240 {
		t.Errorf("an unchanged turn timer overwrote the game's own one: full %d, want 240", s.turnFlow.fullTurnSeconds)
	}
	if s.turnFlow.endTurnSeconds != 20 {
		t.Errorf("End Turn Time = %d, want the new 20 for the next countdown", s.turnFlow.endTurnSeconds)
	}

	// Options closed again with nothing changed: the End Turn value stays as set.
	s.SetEndTurnTimer(25)
	s.ApplyPrefs(LivePrefs{TurnTimer: 180, EndTurn: 20})
	if s.turnFlow.endTurnSeconds != 25 {
		t.Errorf("an unchanged End Turn Time overwrote the game's value: %d, want 25", s.turnFlow.endTurnSeconds)
	}

	// The turn timer changed (180 -> 120): the next round uses it, the running clock keeps counting.
	s.ApplyPrefs(LivePrefs{TurnTimer: 120, EndTurn: 20})
	if s.turnFlow.turnStart != start || s.turnFlow.turnSeconds != secs {
		t.Error("a turn timer change restarted the running clock")
	}
	if s.turnFlow.fullTurnSeconds != 120 {
		t.Errorf("next round budget = %d, want the new 120", s.turnFlow.fullTurnSeconds)
	}
}

// TestEndTurnTimeChangeAppliesToRunningCountdown: a new End Turn Time switches a running countdown at once, shorter
// or longer (floored at 15 seconds). A countdown that has not started, or a turn that already ended sooner than the
// grace, keeps its clock.
func TestEndTurnTimeChangeAppliesToRunningCountdown(t *testing.T) {
	s, st := graceScreen(t)
	s.ApplyPrefs(LivePrefs{TurnTimer: 180, EndTurn: 30}) // game creation
	st.Teams[1].FinishedTurn = true
	s.armEndTurnGrace()
	if !s.turnFlow.graceOnClock || s.turnFlow.turnSeconds != 30 {
		t.Fatalf("the countdown did not arm: on clock %v, budget %d", s.turnFlow.graceOnClock, s.turnFlow.turnSeconds)
	}
	s.turnFlow.turnStart -= 15 * 1000 // 15 seconds into the countdown

	for _, tc := range []struct{ endTurn, want int }{{20, 20}, {60, 60}, {5, 15}} {
		s.ApplyPrefs(LivePrefs{TurnTimer: 180, EndTurn: tc.endTurn})
		if s.turnFlow.turnSeconds != tc.want || s.turnRemaining() < tc.want-1 {
			t.Errorf("End Turn Time %d: budget %d, left %d, want a fresh %d", tc.endTurn, s.turnFlow.turnSeconds, s.turnRemaining(), tc.want)
		}
	}

	// Not armed yet: the running turn keeps its clock (the L1 case: 3:00 never jumps to the new End Turn Time).
	s2, _ := graceScreen(t)
	s2.ApplyPrefs(LivePrefs{TurnTimer: 180, EndTurn: 30})
	s2.turnFlow.turnStart -= 100 * 1000
	start, secs := s2.turnFlow.turnStart, s2.turnFlow.turnSeconds
	s2.ApplyPrefs(LivePrefs{TurnTimer: 180, EndTurn: 20})
	if s2.turnFlow.turnStart != start || s2.turnFlow.turnSeconds != secs {
		t.Errorf("an unarmed turn clock changed: start %d -> %d, budget %d -> %d", start, s2.turnFlow.turnStart, secs, s2.turnFlow.turnSeconds)
	}

	// Armed on a turn that already ended sooner (10 seconds left, under the 30 grace): the clock stays.
	s3, st3 := graceScreen(t)
	s3.ApplyPrefs(LivePrefs{TurnTimer: 180, EndTurn: 30})
	s3.turnFlow.turnSeconds = 10 // 10 seconds left on the running turn (test ticks start near zero, so no rewind)
	s3.turnFlow.turnStart = platform.Ticks()
	st3.Teams[1].FinishedTurn = true
	s3.armEndTurnGrace()
	start, secs = s3.turnFlow.turnStart, s3.turnFlow.turnSeconds
	s3.ApplyPrefs(LivePrefs{TurnTimer: 180, EndTurn: 60})
	if !s3.turnFlow.graceArmed || s3.turnFlow.graceOnClock {
		t.Fatalf("armed %v, on clock %v; want armed, not on the clock", s3.turnFlow.graceArmed, s3.turnFlow.graceOnClock)
	}
	if s3.turnFlow.turnStart != start || s3.turnFlow.turnSeconds != secs {
		t.Error("a turn that already ended sooner than the grace was lengthened by an End Turn Time change")
	}
}

// TestEndTurnTimeChangeNextRoundFlow runs the live simultaneous rounds: a new End Turn Time set before the countdown
// arms leaves the full turn on the clock, the countdown then arms with the new value, and the next round opens on the
// full turn time again until the opponents are done.
func TestEndTurnTimeChangeNextRoundFlow(t *testing.T) {
	s, g := newLiveAIScreen(t, 16, 1)
	g.State.PlayMode = game.PlaySimultaneous
	s.ApplyPrefs(LivePrefs{TurnTimer: 180, EndTurn: 30})

	s.beginSimRound()
	if s.turnFlow.turnSeconds != 180 {
		t.Fatalf("round start budget %d, want the full 180", s.turnFlow.turnSeconds)
	}
	s.ApplyPrefs(LivePrefs{TurnTimer: 180, EndTurn: 20})
	if s.turnFlow.turnSeconds != 180 {
		t.Fatalf("an End Turn Time change before the countdown changed the clock to %d", s.turnFlow.turnSeconds)
	}
	s.armEndTurnGrace()
	if s.turnFlow.turnSeconds != 20 {
		t.Fatalf("the countdown armed at %d, want the new End Turn Time 20", s.turnFlow.turnSeconds)
	}

	s.commitSimTurn()
	s.beginSimRound()
	if s.turnFlow.turnSeconds != 180 || s.turnFlow.graceArmed || s.turnFlow.graceOnClock {
		t.Fatalf("next round: budget %d, armed %v, on clock %v; want the full 180, not armed", s.turnFlow.turnSeconds, s.turnFlow.graceArmed, s.turnFlow.graceOnClock)
	}
	s.armEndTurnGrace()
	if s.turnFlow.turnSeconds != 20 {
		t.Errorf("next round countdown %d, want 20", s.turnFlow.turnSeconds)
	}
}

// TestLoadClearsTheRunningCountdown: a load drops the old game's countdown, so an End Turn Time change right after a
// load cannot switch a clock the loaded game never armed.
func TestLoadClearsTheRunningCountdown(t *testing.T) {
	dir := t.TempDir()
	saver, _ := graceScreen(t)
	saver.SetSaveInfo(dir, "GREEN_1.WRL")
	if err := saver.saveToSlot(1, "grace", false); err != nil {
		t.Fatalf("save: %v", err)
	}

	loader, st := graceScreen(t)
	loader.SetSaveInfo(dir, "GREEN_1.WRL")
	loader.ApplyPrefs(LivePrefs{TurnTimer: 180, EndTurn: 30})
	st.Teams[1].FinishedTurn = true
	loader.armEndTurnGrace()
	if err := loader.Load(1); err != nil {
		t.Fatalf("load: %v", err)
	}
	if loader.turnFlow.graceArmed || loader.turnFlow.graceOnClock {
		t.Fatalf("after a load: armed %v, on clock %v; want neither", loader.turnFlow.graceArmed, loader.turnFlow.graceOnClock)
	}
	start, secs := loader.turnFlow.turnStart, loader.turnFlow.turnSeconds
	loader.ApplyPrefs(LivePrefs{TurnTimer: 180, EndTurn: 20})
	if loader.turnFlow.turnStart != start || loader.turnFlow.turnSeconds != secs {
		t.Error("an End Turn Time change after a load switched a countdown the loaded game had not armed")
	}
}
