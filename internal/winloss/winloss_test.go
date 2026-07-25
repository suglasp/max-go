// winloss_test.go : testing unit for winloss.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package winloss

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

func state(types ...game.TeamType) *game.GameState {
	gs := &game.GameState{TurnCounter: 1}
	for i, typ := range types {
		gs.Teams[i].Type = typ
	}
	gs.PlayerTeam = 0
	return gs
}

func unit(team, turns int, flags game.Flags) *game.UnitInfo {
	u := game.NewUnitInfo(assets.Tank, team, 0, 0)
	u.Flags = flags
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribTurns, turns)
	return u
}

func conditions(statuses [game.TeamCount]TeamStatus) ConditionFunc {
	return func(team int) (TeamStatus, bool) { return statuses[team], true }
}

func TestEvaluateStatusRanksByScoreThenAssetWorth(t *testing.T) {
	gs := state(game.TeamPlayer, game.TeamComputer, game.TeamComputer)
	points := [game.TeamCount]int{10, 10, 20}
	units := []*game.UnitInfo{
		unit(0, 3, game.FlagMobileLandUnit),
		unit(1, 4, game.FlagMobileLandUnit),
	}

	status := EvaluateStatus(Input{State: gs, Units: units, TeamPoints: points})

	if status.TeamsTotal != 3 || status.TeamsInPlay != 3 {
		t.Fatalf("team counts = total %d in-play %d, want 3/3", status.TeamsTotal, status.TeamsInPlay)
	}
	if status.TeamRank[2] != 1 || status.TeamRank[1] != 2 || status.TeamRank[0] != 3 {
		t.Errorf("ranks = %v, want team2 first then asset-worth team1 before team0", status.TeamRank)
	}
	if status.TeamsHumansInPlay != 1 || status.TeamsNonComputers != 1 {
		t.Errorf("human counts = in-play %d non-computer %d, want 1/1", status.TeamsHumansInPlay, status.TeamsNonComputers)
	}
}

// TestEvaluateStatusPerfectTieKeepsLowerTeamIndex: identical points AND identical
// asset worth must rank the lower team index first, matching the original's
// stable bubble sort (winloss.cpp swaps only on a strictly better score).
func TestEvaluateStatusPerfectTieKeepsLowerTeamIndex(t *testing.T) {
	gs := state(game.TeamPlayer, game.TeamComputer, game.TeamComputer)
	points := [game.TeamCount]int{5, 5, 5}

	status := EvaluateStatus(Input{State: gs, TeamPoints: points})

	if status.TeamRank[0] != 1 || status.TeamRank[1] != 2 || status.TeamRank[2] != 3 {
		t.Errorf("ranks = %v, want ascending team index on a perfect tie", status.TeamRank)
	}
}

func TestEvaluateUsesScriptedPlayerWinLoss(t *testing.T) {
	gs := state(game.TeamPlayer, game.TeamComputer)

	if got := Evaluate(Input{State: gs, Conditions: conditions([game.TeamCount]TeamStatus{StatusWon})}); got != PlayerWon {
		t.Errorf("scripted win outcome = %d, want PlayerWon", got)
	}
	if got := Evaluate(Input{State: gs, Conditions: conditions([game.TeamCount]TeamStatus{StatusLost})}); got != PlayerLost {
		t.Errorf("scripted loss outcome = %d, want PlayerLost", got)
	}
}

func TestEvaluateLastTeamStanding(t *testing.T) {
	gs := state(game.TeamPlayer, game.TeamComputer)

	got := Evaluate(Input{
		State:      gs,
		Conditions: conditions([game.TeamCount]TeamStatus{StatusGeneric, StatusLost}),
	})
	if got != PlayerWon {
		t.Errorf("last standing outcome = %d, want PlayerWon", got)
	}
}

func TestEvaluateAllHumansDefeated(t *testing.T) {
	gs := state(game.TeamPlayer, game.TeamComputer)

	got := Evaluate(Input{
		State:      gs,
		Conditions: conditions([game.TeamCount]TeamStatus{StatusLost, StatusGeneric}),
	})
	if got != PlayerLost {
		t.Errorf("all humans defeated outcome = %d, want PlayerLost", got)
	}
}

func TestEvaluateDurationLimitUsesRankWinner(t *testing.T) {
	gs := state(game.TeamPlayer, game.TeamComputer)
	gs.TurnCounter = 12

	got := Evaluate(Input{
		State:        gs,
		TeamPoints:   [game.TeamCount]int{7, 5},
		VictoryType:  VictoryDuration,
		VictoryLimit: 10,
	})
	if got != PlayerWon {
		t.Errorf("duration rank outcome = %d, want PlayerWon", got)
	}
}

func TestEvaluateScoreLimitUsesRankWinner(t *testing.T) {
	gs := state(game.TeamPlayer, game.TeamComputer)

	got := Evaluate(Input{
		State:        gs,
		TeamPoints:   [game.TeamCount]int{5, 10},
		VictoryType:  VictoryScore,
		VictoryLimit: 10,
	})
	if got != PlayerLost {
		t.Errorf("score rank outcome = %d, want PlayerLost", got)
	}
}

func TestEvaluatePendingDoesNotEndOnLastTeamStanding(t *testing.T) {
	gs := state(game.TeamPlayer, game.TeamComputer)

	got := Evaluate(Input{
		State:      gs,
		Conditions: conditions([game.TeamCount]TeamStatus{StatusPending, StatusLost}),
	})
	if got != Continue {
		t.Errorf("pending outcome = %d, want Continue", got)
	}
}
