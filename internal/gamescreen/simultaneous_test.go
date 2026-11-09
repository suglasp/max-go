// simultaneous_test.go : tests simultaneous.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
)

// simTestScreen builds a minimal headless screen running a 1-human + 1-computer
// simultaneous game. endless skips the win check so the round machinery can be
// exercised without deploying a full base.
func simTestScreen() (*Screen, *sim.Game) {
	m := flatWorld(16, 16)
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	p.Options.PlayMode = int(game.PlaySimultaneous)
	g := sim.New(p, m)
	s := &Screen{game: g, wmap: m, lang: assets.Strings{}, turnFlow: turnFlowState{handover: -1}, debugMenu: debugMenuState{endless: true}}
	return s, g
}

// TestSimModeReadsPlayMode confirms the screen runs the simultaneous loop when the
// configured play mode is simultaneous.
func TestSimModeReadsPlayMode(t *testing.T) {
	s, _ := simTestScreen()
	if !s.simMode() {
		t.Fatal("simMode should be true for a PlaySimultaneous game")
	}
	s.SetPlayMode(int(game.PlayTurnBased))
	if s.simMode() {
		t.Fatal("SetPlayMode(turn based) should leave simMode false")
	}
}

// TestBeginSimRoundFlagsComputerTeamsFinished checks that opening a round runs the
// AI planning for the computer teams (flagging them finished) while the human keeps
// the turn open, and does not advance the turn counter.
func TestBeginSimRoundFlagsComputerTeamsFinished(t *testing.T) {
	s, g := simTestScreen()
	s.beginSimRound()

	if !s.turnFlow.simPlanned {
		t.Error("beginSimRound should arm simPlanned")
	}
	if g.State.Teams[0].FinishedTurn {
		t.Error("the human team should NOT be finished at round start (it still plays live)")
	}
	if !g.State.Teams[1].FinishedTurn {
		t.Error("a computer team should be flagged finished once it has planned")
	}
	if g.State.ActiveTeam != 0 || g.State.PlayerTeam != 0 {
		t.Errorf("active/player = %d/%d, want 0/0 (the single human view)", g.State.ActiveTeam, g.State.PlayerTeam)
	}
	if g.State.TurnCounter != 1 {
		t.Errorf("turn counter = %d, want 1 (opening a round must not advance the turn)", g.State.TurnCounter)
	}
}

// TestBeginSimRoundResumeFromLoadSkipsTurnStart: a loaded mid round game reopens
// the round WITHOUT re-running the per team turn start - the economy, builds and
// budget refresh all ran before the save and their results ride it. Only the AI
// plans are rebuilt; the restored FinishedTurn flags stay (the
// GAME_STATE_10_LOAD_GAME guard).
func TestBeginSimRoundResumeFromLoadSkipsTurnStart(t *testing.T) {
	s, g := simTestScreen()

	// A restored mid round board: the human already spent most of a unit's move
	// budget and the computer team finished when it planned before the save.
	tank := game.NewUnitInfo(assets.Tank, 0, 1, 0)
	tank.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	tank.Values = &game.UnitValues{}
	tank.Values.Set(game.AttribSpeed, 8)
	tank.InitState()
	g.Units.Deploy(tank, 5, 5)
	tank.Speed = 2
	g.State.Teams[1].FinishedTurn = true
	s.turnFlow.simLoadResume = true

	s.beginSimRound()

	if !s.turnFlow.simPlanned {
		t.Error("the resume must still arm simPlanned")
	}
	if s.turnFlow.simLoadResume {
		t.Error("the resume flag must clear after one round open")
	}
	if tank.Speed != 2 {
		t.Errorf("tank Speed = %d, want the restored 2 (a resume must NOT refresh budgets)", tank.Speed)
	}
	if !g.State.Teams[1].FinishedTurn {
		t.Error("the restored FinishedTurn flags must stay as saved")
	}
	if g.State.TurnCounter != 1 {
		t.Errorf("turn counter = %d, want 1 (a resume must not advance the turn)", g.State.TurnCounter)
	}
}

// TestCommitSimTurnAdvancesAndReopensRound checks that the human's End Turn commits
// the whole round at once: the turn rolls over and the next round is opened with the
// human live again and the computer reflagged finished.
func TestCommitSimTurnAdvancesAndReopensRound(t *testing.T) {
	s, g := simTestScreen()
	s.beginSimRound()
	s.commitSimTurn()

	if g.State.TurnCounter != 2 {
		t.Errorf("turn counter = %d, want 2 after the commit rolls the round", g.State.TurnCounter)
	}
	if !s.turnFlow.simPlanned {
		t.Error("the next round's plan phase should be armed after commit")
	}
	if g.State.Teams[0].FinishedTurn {
		t.Error("the human should be live (not finished) in the new round")
	}
	if !g.State.Teams[1].FinishedTurn {
		t.Error("the computer should be finished (planned) in the new round")
	}
	if g.State.ActiveTeam != 0 {
		t.Errorf("active team = %d, want 0 after the rollover", g.State.ActiveTeam)
	}
}
