// training.go : the fifteen training missions' goals.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : mission descriptors SC_T0001 .. SC_T0015 (winloss_conditions)
package objectives

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/winloss"
)

// A training mission IS its objective: each one teaches one thing and is won the moment
// the player has done it. Most also carry a loss rule that fires when the player has
// destroyed the very thing the lesson needs (bulldozing the mining station in a mining
// lesson, for instance), so the mission ends rather than dragging on unwinnable.
//
// trainingObjectives is keyed by the retail header's mission_index (SAVE<n>.TRA).
var trainingObjectives = map[int]Objective{
	1:  training1,
	2:  training2,
	3:  training3,
	4:  training4,
	5:  training5,
	6:  training6,
	7:  training7,
	8:  training8,
	9:  training9,
	10: training10,
	11: training11,
	12: training12,
	13: training13,
	14: training14,
	15: training15,
}

// training1 is Mining and Storage: get raw and fuel into store, with somewhere to put
// them. Lose the mining station or the engineer and the lesson cannot be finished.
//
// This is the one script with no liveness preamble, and it ignores the team it is asked
// about - the original evaluates the player's goal whichever team it is handed. Ported as
// it stands: a training mission has one real player, and an opponent that reads WON here
// changes nothing (only LOST takes a team out of play).
//
// C++ ported : SC_T0001 - winloss_conditions
func training1(q Query, _ int) winloss.TeamStatus {
	stocked := q.StoredCargo(teamRed, economy.CargoRaw) >= 1 &&
		q.StoredCargo(teamRed, economy.CargoFuel) >= 1 &&
		q.CountReadyUnits(teamRed, assets.ADump) > 0

	if stocked {
		return winloss.StatusWon
	}

	if q.CountReadyUnits(teamRed, assets.MiningSt) > 0 && q.CountReadyUnits(teamRed, assets.Engineer) > 0 {
		return winloss.StatusPending
	}

	return winloss.StatusLost
}

// training2 is Factory & Structures: raise a light plant and a gun turret. The lesson is
// lost once the player has neither a builder nor the thing that builder was to make.
//
// C++ ported : SC_T0002 - winloss_conditions
func training2(q Query, team int) winloss.TeamStatus {
	return scripted(q, team, func(q Query) winloss.TeamStatus {
		if q.CountReadyUnits(teamRed, assets.LightPlt) > 0 && q.CountReadyUnits(teamRed, assets.GunTurrt) > 0 {
			return winloss.StatusWon
		}

		engineer := q.CountReadyUnits(teamRed, assets.Engineer) > 0 || q.CountReadyUnits(teamRed, assets.GunTurrt) > 0
		builder := q.CountReadyUnits(teamRed, assets.Constrct) > 0 || q.CountReadyUnits(teamRed, assets.LightPlt) > 0

		if engineer && builder {
			return winloss.StatusPending
		}

		return winloss.StatusLost
	})
}

// training3 is Power & Construction: raise a heavy plant and turn out three scouts.
//
// C++ ported : SC_T0003 - winloss_conditions
func training3(q Query, team int) winloss.TeamStatus {
	return scripted(q, team, func(q Query) winloss.TeamStatus {
		scouts := q.CountReadyUnits(teamRed, assets.Scout)

		if q.CountReadyUnits(teamRed, assets.LandPlt) > 0 && scouts > 2 {
			return winloss.StatusWon
		}

		progress := q.CountReadyUnits(teamRed, assets.LightPlt) > 0 || scouts > 2
		builder := q.CountReadyUnits(teamRed, assets.Constrct) > 0 || q.CountReadyUnits(teamRed, assets.LandPlt) > 0

		if progress && q.CountReadyUnits(teamRed, assets.MiningSt) > 0 && builder {
			return winloss.StatusPending
		}

		return winloss.StatusLost
	})
}

// trainingMiningTarget is the RAW mining capacity Surveying asks the player to reach by
// planting a second station on a deposit the surveyor found (the briefing: "the second
// mining station must produce at least 10 Raw Materials a turn").
const trainingMiningTarget = 22

// training4 is Surveying: survey out the ground and raise the team's RAW mining capacity.
//
// C++ ported : SC_T0004 - winloss_conditions
func training4(q Query, team int) winloss.TeamStatus {
	return scripted(q, team, func(q Query) winloss.TeamStatus {
		if q.RawMiningCapacity(teamRed) >= trainingMiningTarget {
			return winloss.StatusWon
		}

		if q.CountReadyUnits(teamRed, assets.Constrct) > 0 && q.CountReadyUnits(teamRed, assets.MiningSt) > 0 {
			return winloss.StatusPending
		}

		return winloss.StatusLost
	})
}

// training5 is Seek & Destroy: find the enemy's mining station and blow it up.
//
// C++ ported : SC_T0005 - winloss_conditions
func training5(q Query, team int) winloss.TeamStatus {
	return scripted(q, team, func(q Query) winloss.TeamStatus {
		if q.CountReadyUnits(teamGreen, assets.MiningSt) > 0 {
			return winloss.StatusPending
		}

		return winloss.StatusWon
	})
}

// training6 is Material Transfer: move material to a builder and put a gun turret up. It
// is won the moment the turret is UNDER construction, not when it stands - the lesson is
// the transfer, not the wait.
//
// C++ ported : SC_T0006 - winloss_conditions
func training6(q Query, team int) winloss.TeamStatus {
	return scripted(q, team, func(q Query) winloss.TeamStatus {
		if q.UnitsBeingConstructed(teamRed, assets.GunTurrt) > 0 {
			return winloss.StatusWon
		}

		return winloss.StatusPending
	})
}

// training7 is Mining Allocation: run two plants at once and still mine more fuel than
// the base burns.
//
// C++ ported : SC_T0007 - winloss_conditions
func training7(q Query, team int) winloss.TeamStatus {
	return scripted(q, team, func(q Query) winloss.TeamStatus {
		running := q.RunningBuildings(teamRed, assets.LightPlt) + q.RunningBuildings(teamRed, assets.LandPlt)
		surplus := q.TotalMining(teamRed, economy.CargoFuel) - q.TotalConsumption(teamRed, economy.CargoFuel)

		if running > 1 && surplus > 0 {
			return winloss.StatusWon
		}

		plants := q.CountReadyUnits(teamRed, assets.LightPlt) + q.CountReadyUnits(teamRed, assets.LandPlt)

		if plants > 1 && q.CountReadyUnits(teamRed, assets.MiningSt) > 0 {
			return winloss.StatusPending
		}

		return winloss.StatusLost
	})
}

// training8 is Mining for Gold: set a station to mine gold and get some into store.
//
// C++ ported : SC_T0008 - winloss_conditions
func training8(q Query, team int) winloss.TeamStatus {
	return scripted(q, team, func(q Query) winloss.TeamStatus {
		if q.StoredCargo(teamRed, economy.CargoGold) > 0 {
			return winloss.StatusWon
		}

		return winloss.StatusPending
	})
}

// training9 is Upgrade your Tanks: spend gold on the Upgrade screen until a tank rolls
// out above Mk I.
//
// C++ ported : SC_T0009 - winloss_conditions
func training9(q Query, team int) winloss.TeamStatus {
	return scripted(q, team, func(q Query) winloss.TeamStatus {
		if q.HasUnitAboveMarkLevel(teamRed, assets.Tank, 1) {
			return winloss.StatusWon
		}

		return winloss.StatusPending
	})
}

// training10 is Victory Points: get an eco sphere running and score a single point.
//
// C++ ported : SC_T0010 - winloss_conditions
func training10(q Query, team int) winloss.TeamStatus {
	return scripted(q, team, func(q Query) winloss.TeamStatus {
		if q.Points(teamRed) > 0 {
			return winloss.StatusWon
		}

		return winloss.StatusPending
	})
}

// training11 is Infiltrator: train a commando and blood it on an enemy unit. Lose the
// buildings that make and house commandos and the lesson is over.
//
// C++ ported : SC_T0011 - winloss_conditions
func training11(q Query, team int) winloss.TeamStatus {
	return scripted(q, team, func(q Query) winloss.TeamStatus {
		if q.HasUnitExperience(teamRed, assets.Commando) {
			return winloss.StatusWon
		}

		hall := q.CountReadyUnits(teamRed, assets.TrainHal) > 0
		housing := q.CountReadyUnits(teamRed, assets.Habitat) > 0
		mining := q.CountReadyUnits(teamRed, assets.MiningSt) > 0

		if hall && housing && mining {
			return winloss.StatusPending
		}

		return winloss.StatusLost
	})
}

// training12 is Mines: lay a mine of your own AND clear one of the enemy's.
//
// C++ ported : SC_T0012 - winloss_conditions
func training12(q Query, team int) winloss.TeamStatus {
	return scripted(q, team, func(q Query) winloss.TeamStatus {
		if q.CountReadyUnits(teamRed, assets.LandMine) > 0 && q.CountCasualties(teamGreen, assets.LandMine) > 0 {
			return winloss.StatusWon
		}

		return winloss.StatusPending
	})
}

// trainingTankSquad is the three tanks the Repairing and Reloading lessons hand the
// player and expect back in one piece.
const trainingTankSquad = 3

// training13 is Repairing: get all three tanks back to full health. Let one be destroyed
// instead of repaired and the lesson is lost.
//
// C++ ported : SC_T0013 - winloss_conditions
func training13(q Query, team int) winloss.TeamStatus {
	return scripted(q, team, func(q Query) winloss.TeamStatus {
		short := q.CountReadyUnits(teamRed, assets.Tank) < trainingTankSquad ||
			q.CountDamagedUnits(teamRed, assets.Tank) > 0

		if !short {
			return winloss.StatusWon
		}

		if q.CountCasualties(teamRed, assets.Tank) > 0 {
			return winloss.StatusLost
		}

		return winloss.StatusPending
	})
}

// training14 is Reloading: get all three tanks back to full ammo. Same shape as the
// repair lesson, on a different gauge.
//
// C++ ported : SC_T0014 - winloss_conditions
func training14(q Query, team int) winloss.TeamStatus {
	return scripted(q, team, func(q Query) winloss.TeamStatus {
		short := q.CountReadyUnits(teamRed, assets.Tank) < trainingTankSquad ||
			q.CountUnitsWithReducedAttrib(teamRed, assets.Tank, game.AttribAmmo) > 0

		if !short {
			return winloss.StatusWon
		}

		if q.CountCasualties(teamRed, assets.Tank) > 0 {
			return winloss.StatusLost
		}

		return winloss.StatusPending
	})
}

// training15 is Research: put a research centre on Armor and see the level rise.
//
// C++ ported : SC_T0015 - winloss_conditions
func training15(q Query, team int) winloss.TeamStatus {
	return scripted(q, team, func(q Query) winloss.TeamStatus {
		if q.ResearchLevel(teamRed, game.ResearchArmor) > 0 {
			return winloss.StatusWon
		}

		if q.CountReadyUnits(teamRed, assets.Research) > 0 {
			return winloss.StatusPending
		}

		return winloss.StatusLost
	})
}
