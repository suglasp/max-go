// objectives.go : what each retail mission actually asks you to do.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : the winloss_conditions scripts of mission descriptors (SC_C0001..SC_T0015)
package objectives

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/winloss"
)

// A mission is not won by wiping the enemy out. Campaign 1 wants the enemy's eco
// spheres gone and their research centre INTACT; Training 13 wants three tanks at full
// health; Bright Hope wants four alien units captured before the clock runs out. Vanilla
// held these rules in the executable, keyed on the mission_index in the save file's
// header ("0 means generic rules apply"). This project spells them out in Go, so it needs neither a script engine nor an additional data file.
//
// A mission the table does not know keeps the generic rules (elimination, score, turn
// limit), which is also how most retail mission descriptors behave.

// Team indices the mission scripts are written against: the player always lands as RED,
// the scripted opponent is GREEN. Every retail script hardcodes the pair.
const (
	teamRed   = 0
	teamGreen = 1
)

// Query is the board a mission objective asks its questions of. *sim.Game answers it;
// the interface keeps this package out of the sim's import graph and lets a test drive
// an objective against a board it made up.
//
// C++ ported : scripter.cpp - the max_* functions the win/loss Lua context registers
type Query interface {
	HasAttackPower(team int, c game.UnitClass) bool
	HasProductionInfrastructure(team int) bool
	HasConstructionInfrastructure(team int) bool
	CountReadyUnits(team int, t assets.UnitType) int
	CountCasualties(team int, t assets.UnitType) int
	StoredCargo(team int, cargo economy.CargoType) int
	RawMiningCapacity(team int) int
	TotalMining(team int, cargo economy.CargoType) int
	TotalConsumption(team int, cargo economy.CargoType) int
	RunningBuildings(team int, t assets.UnitType) int
	UnitsBeingConstructed(team int, t assets.UnitType) int
	HasUnitAboveMarkLevel(team int, t assets.UnitType, mark int) bool
	HasUnitExperience(team int, t assets.UnitType) bool
	CountDamagedUnits(team int, t assets.UnitType) int
	CountUnitsWithReducedAttrib(team int, t assets.UnitType, a game.Attrib) int
	ResearchLevel(team, topic int) int
	Points(team int) int
	TurnValue() int
	TurnVictoryLimit() int
}

// Objective is one mission's rule, answering the four state question for a team.
//
// C++ ported : the winloss_conditions Lua chunk, called with the team as its argument
type Objective func(q Query, team int) winloss.TeamStatus

// For returns the objective of the mission the save header names, or nil when it has
// none and the generic rules apply. index is the retail header's mission_index (1 based
// per category; a 0 means a game with no scripted objective at all).
//
// C++ ported : missionregistry.cpp - the descriptor looked up for the loaded mission
func For(category game.MissionCategory, index int) Objective {
	switch category {
	case game.MissionCampaign:
		return campaignObjectives[index]
	case game.MissionScenario:
		return scenarioObjectives[index]
	case game.MissionTraining:
		return trainingObjectives[index]
	}

	return nil // multiplayer scenarios and demos carry no scripted objective
}

// Bind hands an objective the board to ask, giving the sim the condition hook it takes.
func Bind(o Objective, q Query) winloss.ConditionFunc {
	if o == nil || q == nil {
		return nil
	}

	return func(team int) (winloss.TeamStatus, bool) {
		return o(q, team), true
	}
}

// alive is the liveness test every retail script opens with: a team is still in the
// fight while it can shoot, produce or build. "Can shoot" means a unit with ammunition; submarines and commandos do not count, so a team with
// only stealth units is beaten.
//
// C++ ported : the shared preamble of every winloss_conditions script (and SC_L0002,
// the default script a mission without its own inherits)
func alive(q Query, team int) bool {
	return q.HasAttackPower(team, game.ClassMobileLandSea) ||
		q.HasAttackPower(team, game.ClassMobileAir) ||
		q.HasAttackPower(team, game.ClassStationary) ||
		q.HasProductionInfrastructure(team) ||
		q.HasConstructionInfrastructure(team)
}

// scripted wraps the shape every bespoke mission shares: a dead team is LOST, a
// surviving OPPONENT is generic (it plays for points like anyone else), and the player's
// own verdict is the mission's to give.
//
// The player's alive state that goal falls back on is PENDING, not GENERIC, and that is
// load bearing: PENDING vetoes the "last team standing" ending, so wiping the enemy out
// does NOT win a mission whose objective is unmet - and it turns the expiry of a turn
// limit into a defeat.
func scripted(q Query, team int, goal func(q Query) winloss.TeamStatus) winloss.TeamStatus {
	if !alive(q, team) {
		return winloss.StatusLost
	}

	if team != teamRed {
		return winloss.StatusGeneric
	}

	return goal(q)
}
