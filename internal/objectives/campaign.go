// campaign.go : the nine campaign missions' win and loss conditions.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : mission descriptors SC_C0001 .. SC_C0009 (winloss_conditions)
package objectives

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/winloss"
)

// campaignObjectives is keyed by the retail header's mission_index (SAVE<n>.CAM).
// Missions 6 and 7 are absent on purpose: Repel Boarders and Bastion of Rebellion carry
// no bespoke objective in the original either and are won on the generic rules.
var campaignObjectives = map[int]Objective{
	1: campaign1,
	2: campaign2,
	3: campaign3,
	4: razeTheEcoSpheres, // Stone Cold Deadly
	5: razeTheEcoSpheres, // Slaughter Shore
	8: campaign8,
	9: razeTheEcoSpheres, // Price of Freedom
}

// campaign1 is Islands in the Sun: destroy the enemy's eco spheres, but leave their
// research centre standing. Blowing the lab up loses the mission on the spot - the whole
// point of the raid is to take it intact.
//
// C++ ported : SC_C0001 - winloss_conditions
func campaign1(q Query, team int) winloss.TeamStatus {
	return scripted(q, team, func(q Query) winloss.TeamStatus {
		if q.CountReadyUnits(teamGreen, assets.Research) == 0 {
			return winloss.StatusLost // the lab was the objective, and it is rubble
		}

		if q.CountReadyUnits(teamGreen, assets.Greenhse) > 0 {
			return winloss.StatusPending
		}

		return winloss.StatusWon
	})
}

// campaign2 is Heart of the Matter: hold out to the turn limit without losing a research
// centre of your own.
//
// C++ ported : SC_C0002 - winloss_conditions
func campaign2(q Query, team int) winloss.TeamStatus {
	return scripted(q, team, func(q Query) winloss.TeamStatus {
		if q.CountCasualties(teamRed, assets.Research) > 0 {
			return winloss.StatusLost
		}

		if q.TurnValue() < q.TurnVictoryLimit() {
			return winloss.StatusPending
		}

		return winloss.StatusWon
	})
}

// campaign3 is Element of Import: come away with two of the supply trucks. Destroying
// the trucks you were sent to take loses it (fewer than two left between both sides), and
// so does losing the commando before the job is done.
//
// C++ ported : SC_C0003 - winloss_conditions
func campaign3(q Query, team int) winloss.TeamStatus {
	return scripted(q, team, func(q Query) winloss.TeamStatus {
		mine := q.CountReadyUnits(teamRed, assets.SplyTrck)

		if mine+q.CountReadyUnits(teamGreen, assets.SplyTrck) < 2 {
			return winloss.StatusLost // not enough trucks left in the world to take two
		}

		if mine > 1 {
			return winloss.StatusWon
		}

		if q.CountReadyUnits(teamRed, assets.Commando) > 0 {
			return winloss.StatusPending
		}

		return winloss.StatusLost // no commando left to seize them with
	})
}

// razeTheEcoSpheres is the plain "destroy every enemy eco sphere" objective, shared by
// Stone Cold Deadly, Slaughter Shore, Price of Freedom and the D-Day scenario.
//
// C++ ported : SC_C0004 / SC_C0005 / SC_C0009 / SC_S0024 - winloss_conditions
func razeTheEcoSpheres(q Query, team int) winloss.TeamStatus {
	return scripted(q, team, func(q Query) winloss.TeamStatus {
		if q.CountReadyUnits(teamGreen, assets.Greenhse) > 0 {
			return winloss.StatusPending
		}

		return winloss.StatusWon
	})
}

// alienUnits are the four types Bright Hope counts: the derelict alien war machines and
// the Juggernaut.
var alienUnits = []assets.UnitType{assets.AlnPlane, assets.AlnTank, assets.AlnAsGun, assets.Juggrnt}

// countAliens totals the alien war machines a team holds.
func countAliens(q Query, team int) int {
	n := 0

	for _, t := range alienUnits {
		n += q.CountReadyUnits(team, t)
	}

	return n
}

// campaign8 is Bright Hope: capture four alien units before the clock runs out. Destroy
// too many of them and the mission is unwinnable at once (fewer than four left standing
// anywhere), and letting the turn limit pass with fewer than four in hand loses it.
//
// Once the four are captured the mission goes GENERIC rather than WON: the objective is
// satisfied, and the game is then decided on the generic rules like any other.
//
// C++ ported : SC_C0008 - winloss_conditions
func campaign8(q Query, team int) winloss.TeamStatus {
	return scripted(q, team, func(q Query) winloss.TeamStatus {
		mine := countAliens(q, teamRed)

		if mine+countAliens(q, teamGreen) < 4 {
			return winloss.StatusLost // too few alien units left in the world to take four
		}

		if mine > 3 {
			return winloss.StatusGeneric
		}

		if q.TurnValue() >= q.TurnVictoryLimit() {
			return winloss.StatusLost // out of time with the aliens still out there
		}

		return winloss.StatusPending
	})
}
