// wingoal.go : M.A.X. AI victory goals (kill an opponent or colonize a planet)
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import (
	"math/rand"

	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/winloss"
)

// winGoal is the victory objective a computer team follows: destroy its opponents or build eco spheres to win on score.
// C++ AiPlayer has no corresponding goal enum and always favors attack.
type winGoal int

const (
	goalKill     winGoal = iota // win by eliminating opponents
	goalColonize                // win on score by building eco spheres
)

// colonizeEcoSpheres is how many eco spheres a colonising team builds toward (each
// powered one adds team points every turn). Enough to build a real score lead while
// still leaving constructor time for the military backbone the team defends with.
const colonizeEcoSpheres = 4

// goalName labels a win goal for the debug trace.
func goalName(g winGoal) string {
	if g == goalColonize {
		return "colonize"
	}

	return "kill"
}

// winGoalRoll returns a percentage roll in [0,100) for the colonise chance. It is a
// package var so a test can force the branch (mirrors pickZoneSearch in task_attack).
var winGoalRoll = func() int { return rand.Intn(100) }

// chooseWinGoal selects Colonize for score victories. Other games use a difficulty-weighted roll, with easier opponents always choosing Kill.
// The game state does not distinguish elimination missions from skirmishes, so both use the same roll.
func chooseWinGoal(g *sim.Game, difficulty int) winGoal {
	if g != nil && g.VictoryType == winloss.VictoryScore && g.VictoryLimit > 0 {
		return goalColonize
	}

	if winGoalRoll() < colonizeChance(difficulty) {
		return goalColonize
	}

	return goalKill
}

// resolveWinGoal returns team's committed match objective: the goal a loaded save
// carries (g.State.AIWinGoal stores goal+1, 0 = unset) if set, else a fresh
// chooseWinGoal roll that is written back onto the game state so a save captures it.
// This mirrors the difficulty tier's persist through the game state pattern, so a
// reload keeps the AI's kill vs colonise plan instead of re-rolling it mid game.
func resolveWinGoal(g *sim.Game, team, difficulty int) winGoal {
	var st *game.GameState
	if g != nil {
		st = g.State
	}

	if st != nil && team >= 0 && team < len(st.AIWinGoal) && st.AIWinGoal[team] > 0 {
		return winGoal(st.AIWinGoal[team] - 1)
	}

	goal := chooseWinGoal(g, difficulty)

	if st != nil && team >= 0 && team < len(st.AIWinGoal) {
		st.AIWinGoal[team] = int(goal) + 1
	}

	return goal
}

// colonizeChance is the percent chance a non score game's team chooses the colonise
// route over the kill, rising with difficulty: the clueless/apprentice/average tiers
// always fight (0), only the cunning Expert+ tiers sometimes play the longer score
// game. Tuned so even God usually attacks (the more direct win) but can surprise with
// a colonisation race.
func colonizeChance(difficulty int) int {
	switch difficulty {
	case OpponentExpert:
		return 15
	case OpponentMaster:
		return 25
	case OpponentGod:
		return 40
	default:
		return 0 // Clueless / Apprentice / Average just fight
	}
}
