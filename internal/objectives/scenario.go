// scenario.go : the stand alone scenarios that carry a bespoke objective.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : mission descriptors SC_S0009 / SC_S0018 / SC_S0024 (winloss_conditions)
package objectives

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/winloss"
)

// scenarioObjectives is keyed by the retail header's mission_index (SAVE<n>.SCE). Only
// three of the twenty four scenarios have an objective of their own; the rest are points
// races and are decided on the generic rules, in the original as much as here.
var scenarioObjectives = map[int]Objective{
	9:  scenario9,
	18: scenario18,
	24: razeTheEcoSpheres, // D-Day, the same objective as the campaign missions
}

// scenario9 is One Bullet: get ahead of the enemy on points, and the mission is yours the
// moment you are. (The map's turn limit is what makes that hard: it is a sprint.)
//
// C++ ported : SC_S0009 - winloss_conditions
func scenario9(q Query, team int) winloss.TeamStatus {
	return scripted(q, team, func(q Query) winloss.TeamStatus {
		if q.Points(teamRed) > q.Points(teamGreen) {
			return winloss.StatusWon
		}

		return winloss.StatusPending
	})
}

// scenario18 is Covert: destroy the enemy's research centres. Nothing else about their
// base matters.
//
// C++ ported : SC_S0018 - winloss_conditions
func scenario18(q Query, team int) winloss.TeamStatus {
	return scripted(q, team, func(q Query) winloss.TeamStatus {
		if q.CountReadyUnits(teamGreen, assets.Research) > 0 {
			return winloss.StatusPending
		}

		return winloss.StatusWon
	})
}
