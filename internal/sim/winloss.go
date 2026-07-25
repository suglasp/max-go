// winloss.go : evaluates Ecosphere score, turn limits, and team elimination.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : game_manager.cpp - GameManager_CheckEndConditions
package sim

import (
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/winloss"
)

// teamHasUnits reports whether team still has any unit on the map. This is ELIMINATION
// (the team is gone from the board), which is not the same question as TeamIsAlive below
// (the team can no longer fight back): a team can be beaten long before its last surveyor
// dies. UpdateEliminations wants this one.
func (g *Game) teamHasUnits(team int) bool {
	for _, u := range g.Units.Units() {
		if u.Team == team {
			return true
		}
	}
	return false
}

// TeamIsAlive reports whether a team can still fight, produce, or build. A team with only a surveyor and mining station is beaten even though it
// still has units on the board. Mission scripts and the default rule use this liveness test.
//
// C++ ported : scripter.cpp - the default win/loss conditions script (resource SC_L0002)
func (g *Game) TeamIsAlive(team int) bool {
	return g.HasAttackPower(team, game.ClassMobileLandSea) ||
		g.HasAttackPower(team, game.ClassMobileAir) ||
		g.HasAttackPower(team, game.ClassStationary) ||
		g.HasProductionInfrastructure(team) ||
		g.HasConstructionInfrastructure(team)
}

// defaultCondition is the rule a game with no scripted objective runs on: a team that can
// no longer fight, produce or build is LOST; everyone else is left generic and the winner
// is decided by last team standing / score / duration.
//
// C++ ported : winlosshandler.cpp - WinLossHandler::TestWinLossConditions running the
// default script (scripter.cpp LoadDefaultWinLossConditions)
func (g *Game) defaultCondition(team int) (winloss.TeamStatus, bool) {
	if team < 0 || team >= game.TeamCount || g.State.Teams[team].Type == game.TeamNone {
		return winloss.StatusGeneric, true
	}

	if !g.TeamIsAlive(team) {
		return winloss.StatusLost, true
	}

	return winloss.StatusGeneric, true
}

// conditions returns the active per team status rule: the scripted override
// when set, else the default elimination rule.
func (g *Game) conditions() winloss.ConditionFunc {
	if g.WinLossConditions != nil {
		return g.WinLossConditions
	}
	return g.defaultCondition
}

// winlossInput packs the current state into the winloss evaluator's input.
func (g *Game) winlossInput() winloss.Input {
	return winloss.Input{
		State:        g.State,
		Units:        g.Units.Units(),
		TeamPoints:   g.TeamPoints,
		VictoryType:  g.VictoryType,
		VictoryLimit: g.VictoryLimit,
		Conditions:   g.conditions(),
	}
}

// Outcome reports the local player's end of game result for the current
// state (continue / won / lost).
//
// C++: menu_check_end_game_conditions decision (menu.cpp).
func (g *Game) Outcome() winloss.Outcome {
	return winloss.Evaluate(g.winlossInput())
}

// Status returns the full per team win/loss snapshot: status, score rank and
// the in play / human / non computer counts.
//
// C++: WinLoss_EvaluateStatus (winloss.cpp).
func (g *Game) Status() winloss.Status {
	return winloss.EvaluateStatus(g.winlossInput())
}

// UpdateEliminations marks any present team that has lost all its units as
// eliminated, mirroring the menu_check_end_game_conditions side effect that
// sets team_type = TEAM_TYPE_ELIMINATED for a wiped out team. It no ops on an
// empty map (before colonies land / a bare harness) so teams that never
// deployed are not falsely eliminated.
//
// C++: the VICTORY_STATE_LOST -> TEAM_TYPE_ELIMINATED branch of
// menu_check_end_game_conditions (menu.cpp).
func (g *Game) UpdateEliminations() {
	if len(g.Units.Units()) == 0 {
		return
	}
	for team := 0; team < game.TeamCount; team++ {
		info := &g.State.Teams[team]
		if info.Type == game.TeamNone || info.Eliminated {
			continue
		}
		if !g.teamHasUnits(team) {
			info.Eliminated = true
		}
	}
}
