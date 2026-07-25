// winloss.go : the M.A.X. win/loss evaluator - per team status at the end of a game. Score ranking and outcome determination.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// Note: Package winloss evaluates the headless state at the end of a game.
// per team win/loss status, score ranking, and whether the local player should see a win screen, a loss screen, or continue playing.
package winloss

import (
	"sort"

	"github.com/suglasp/max-go/internal/build"
	"github.com/suglasp/max-go/internal/game"
)

// TeamStatus is the mission script result per team.
//
// C++: WinLossState (enums.hpp)
type TeamStatus int

const (
	StatusGeneric TeamStatus = iota
	StatusPending
	StatusWon
	StatusLost
)

// VictoryType is the win condition selected in settings for the whole game.
//
// C++: VICTORY_TYPE_* (enums.hpp)
type VictoryType int

const (
	VictoryDuration VictoryType = iota
	VictoryScore
)

// Outcome is the local player's result at the end of the game.
type Outcome int

const (
	Continue Outcome = iota
	PlayerWon
	PlayerLost
)

// ConditionFunc yields the mission/script status for team. A false return
// mirrors the C++ assert fallback by treating the team as generic.
//
// C++: WinLossHandler::TestWinLossConditions (winlosshandler.cpp)
type ConditionFunc func(team int) (TeamStatus, bool)

// Input is the pure state needed to evaluate win/loss. TeamPoints is kept
// outside game.TeamInfo until the save/state model grows that field.
type Input struct {
	State        *game.GameState
	Units        []*game.UnitInfo
	TeamPoints   [game.TeamCount]int
	VictoryType  VictoryType
	VictoryLimit int
	Conditions   ConditionFunc
}

// Status is the Go equivalent of WinLoss_Status without the alien team slot.
//
// C++: WinLoss_Status (winloss.hpp)
type Status struct {
	TeamType          [game.TeamCount]game.TeamType
	TeamStatus        [game.TeamCount]TeamStatus
	TeamRank          [game.TeamCount]int
	TeamsTotal        int
	TeamsInPlay       int
	TeamsHumansInPlay int
	TeamsNonComputers int
}

// AssetWorth returns the build cost tiebreaker for a team's live units.
//
// C++: WinLoss_GetTotalWorthOfAssets (winloss.cpp)
func AssetWorth(team int, units []*game.UnitInfo) int {
	total := 0
	for _, u := range units {
		if u != nil && u.Team == team {
			total += build.Cost(u)
		}
	}
	return total
}

// EvaluateStatus collects win/loss state per team and ranks active teams by
// score, using live asset worth as the tiebreaker when the score is the same.
//
// C++: WinLoss_EvaluateStatus (winloss.cpp)
func EvaluateStatus(in Input) Status {
	var status Status
	if in.State == nil {
		return status
	}

	inPlay := make([]int, 0, game.TeamCount)
	for team := 0; team < game.TeamCount; team++ {
		info := in.State.Teams[team]
		status.TeamType[team] = info.Type
		if info.Type == game.TeamNone {
			continue
		}

		teamStatus := StatusGeneric
		if in.Conditions != nil {
			if s, ok := in.Conditions(team); ok {
				teamStatus = s
			}
		}
		status.TeamStatus[team] = teamStatus
		status.TeamsTotal++

		if info.Type != game.TeamComputer {
			status.TeamsNonComputers++
		}
		if teamStatus != StatusLost && !info.Eliminated {
			if info.Type != game.TeamComputer {
				status.TeamsHumansInPlay++
			}
			inPlay = append(inPlay, team)
		}
	}

	sort.Slice(inPlay, func(i, j int) bool {
		a, b := inPlay[i], inPlay[j]
		if in.TeamPoints[a] != in.TeamPoints[b] {
			return in.TeamPoints[a] > in.TeamPoints[b]
		}
		if wa, wb := AssetWorth(a, in.Units), AssetWorth(b, in.Units); wa != wb {
			return wa > wb
		}
		// A perfect tie keeps the lower team index in front, matching the
		// original's bubble sort that swaps only on a strictly better score.
		return a < b
	})
	status.TeamsInPlay = len(inPlay)
	for i, team := range inPlay {
		status.TeamRank[team] = i + 1
	}
	return status
}

// Evaluate returns the local player's outcome at the end of the game.
//
// C++: menu_check_end_game_conditions + menu_wrap_up_game winner test
// (menu.cpp)
func Evaluate(in Input) Outcome {
	status := EvaluateStatus(in)
	if in.State == nil || in.State.PlayerTeam < 0 || in.State.PlayerTeam >= game.TeamCount {
		return Continue
	}
	player := in.State.PlayerTeam
	playerStatus := status.TeamStatus[player]

	switch playerStatus {
	case StatusWon:
		return PlayerWon
	case StatusLost:
		return PlayerLost
	}

	ended := false
	switch {
	case playerStatus != StatusPending && status.TeamsInPlay == 1 && status.TeamsTotal > 1:
		ended = true
	case status.TeamsInPlay == 0:
		ended = true
	case status.TeamsHumansInPlay == 0 && status.TeamsNonComputers > 0:
		ended = true
	case in.VictoryType == VictoryDuration && in.VictoryLimit > 0 && in.VictoryLimit <= in.State.TurnCounter:
		ended = true
	case in.VictoryType == VictoryScore && scoreLimitReached(in):
		ended = true
	}
	if !ended {
		return Continue
	}
	if playerStatus == StatusGeneric && status.TeamRank[player] == 1 {
		return PlayerWon
	}
	return PlayerLost
}

// scoreLimitReached reports whether any team has reached the configured score limit.
func scoreLimitReached(in Input) bool {
	if in.VictoryLimit <= 0 {
		return false
	}
	for team := 0; team < game.TeamCount; team++ {
		if in.State.Teams[team].Type != game.TeamNone && in.TeamPoints[team] >= in.VictoryLimit {
			return true
		}
	}
	return false
}
