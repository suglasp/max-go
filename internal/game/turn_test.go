// turn_test.go : tests turn-based and simultaneous sequencing.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package game

import "testing"

func twoPlayerParams() *StartParams {
	p := &StartParams{}
	p.Teams[0].Type = TeamPlayer
	p.Teams[2].Type = TeamComputer // leave team 1 and 3 as None to test skipping
	return p
}

func TestNewGameStateSeedsActiveTeams(t *testing.T) {
	gs := NewGameState(twoPlayerParams())
	if gs.TurnCounter != 1 {
		t.Errorf("turn counter = %d, want 1", gs.TurnCounter)
	}
	if gs.Phase != PhaseInGame {
		t.Errorf("phase = %d, want PhaseInGame", gs.Phase)
	}
	if gs.ActiveTeam != 0 || gs.PlayerTeam != 0 {
		t.Errorf("active/player = %d/%d, want 0/0", gs.ActiveTeam, gs.PlayerTeam)
	}
	if !gs.IsActive(0) || gs.IsActive(1) || !gs.IsActive(2) || gs.IsActive(3) {
		t.Error("IsActive does not match the configured teams")
	}
}

func TestFinishTurnRotatesActiveTeamSkippingNone(t *testing.T) {
	gs := NewGameState(twoPlayerParams())
	gs.FinishTurn(0)
	if gs.ActiveTeam != 2 {
		t.Errorf("after team 0 finished, active = %d, want 2 (1 is None)", gs.ActiveTeam)
	}
	if gs.AllFinished() {
		t.Error("not all teams finished yet (team 2 outstanding)")
	}
	gs.FinishTurn(2)
	if !gs.AllFinished() {
		t.Error("all active teams should be finished")
	}
}

func TestAdvanceTurnResets(t *testing.T) {
	gs := NewGameState(twoPlayerParams())
	gs.FinishTurn(0)
	gs.FinishTurn(2)
	gs.AdvanceTurn()
	if gs.TurnCounter != 2 {
		t.Errorf("turn counter = %d, want 2", gs.TurnCounter)
	}
	if gs.AllFinished() {
		t.Error("finished flags should be cleared after AdvanceTurn")
	}
	if gs.ActiveTeam != 0 {
		t.Errorf("active team = %d, want 0 after rollover", gs.ActiveTeam)
	}
}

func TestEliminatedTeamSkipped(t *testing.T) {
	gs := NewGameState(twoPlayerParams())
	gs.Teams[0].Eliminated = true
	if gs.IsActive(0) {
		t.Error("eliminated team should not be active")
	}
	if gs.FirstActiveTeam() != 2 {
		t.Errorf("first active = %d, want 2", gs.FirstActiveTeam())
	}
}

func TestNewGameStateDefaultsTurnBased(t *testing.T) {
	// An unset Options.PlayMode (0) yields the turn based loop.
	gs := NewGameState(twoPlayerParams())
	if gs.PlayMode != PlayTurnBased {
		t.Errorf("default play mode = %d, want PlayTurnBased", gs.PlayMode)
	}
}

func TestNewGameStateReadsSimultaneous(t *testing.T) {
	p := twoPlayerParams()
	p.Options.PlayMode = int(PlaySimultaneous)
	gs := NewGameState(p)
	if gs.PlayMode != PlaySimultaneous {
		t.Errorf("play mode = %d, want PlaySimultaneous", gs.PlayMode)
	}
}

func TestNewGameStateClampsUnknownPlayMode(t *testing.T) {
	p := twoPlayerParams()
	p.Options.PlayMode = 99 // out of range -> turn based
	if gs := NewGameState(p); gs.PlayMode != PlayTurnBased {
		t.Errorf("unknown play mode -> %d, want PlayTurnBased", gs.PlayMode)
	}
}

func TestFinishTurnSimultaneousKeepsActiveTeam(t *testing.T) {
	// In simultaneous mode FinishTurn marks a team done without rotating the active slot; the round advances after every team finishes.
	p := twoPlayerParams()
	p.Options.PlayMode = int(PlaySimultaneous)
	gs := NewGameState(p)
	gs.FinishTurn(0)
	if gs.ActiveTeam != 0 {
		t.Errorf("simultaneous FinishTurn rotated active to %d, want it to stay 0", gs.ActiveTeam)
	}
	if !gs.Teams[0].FinishedTurn {
		t.Error("team 0 should be flagged finished")
	}
	if gs.AllFinished() {
		t.Error("team 2 has not finished yet")
	}
	gs.FinishTurn(2)
	if !gs.AllFinished() {
		t.Error("all active teams should now be finished")
	}
}
