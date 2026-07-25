// missionquery.go : the board questions a scripted mission objective asks.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : scripter.cpp - the max_* functions registered for the win/loss Lua context
package sim

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/winloss"
)

// A mission objective is a question about the board: how many of that unit does the
// enemy still have, is my commando blooded yet, am I mining enough. The original asks
// them through twenty Lua functions the engine registers for the mission script; the
// port asks them through the methods below, and internal/objectives spells the missions
// out in terms of them.
//
// These are ports, not paraphrases: several of the original's functions do something
// other than their name suggests, and the missions are built on what they DO. The
// surprises are called out at each one.
//
// C++ ported : scripter.cpp - RegisterHasMaterials .. RegisterGetResearchLevel

// teamUnits calls fn for each of the team's units. The alien slot is a team like any
// other here: a mission may well ask about it (Bright Hope counts alien units).
func (g *Game) teamUnits(team int, fn func(u *game.UnitInfo)) {
	if team < 0 || team >= game.TeamCount {
		return
	}

	for _, u := range g.Units.Units() {
		if u.Team == team {
			fn(u)
		}
	}
}

// HasAttackPower reports whether the team still has something of that family that can
// shoot. GOTCHA, and it is the original's rule, not a simplification: it tests AMMO,
// not the attack attribute, and it deliberately ignores the Submarine and the Commando
// - a team left with nothing but stealth units counts as beaten. Every mission's
// liveness preamble leans on this.
//
// C++ ported : scripter.cpp - HasAttackPower
func (g *Game) HasAttackPower(team int, c game.UnitClass) bool {
	found := false

	g.teamUnits(team, func(u *game.UnitInfo) {
		if found || !u.InClass(c) || u.Ammo <= 0 {
			return
		}

		if u.Type == assets.Submarne || u.Type == assets.Commando {
			return
		}

		found = true
	})

	return found
}

// productionBuildings are the five the original counts as production infrastructure
// (the Training Hall included, unlike the score screen's factory tally).
var productionBuildings = []assets.UnitType{
	assets.Shipyard, assets.LightPlt, assets.LandPlt, assets.AirPlt, assets.TrainHal,
}

// HasProductionInfrastructure reports whether the team can still turn out units: it
// owns a plant AND generates the power to run it. The power demand is 1, or 2 when the
// team is also short of materials (under 8 fuel or under 24 raw in store), and in that
// short case a team with no mining station is written off at once.
//
// C++ ported : scripter.cpp - HasProductionInfrastructure
func (g *Game) HasProductionInfrastructure(team int) bool {
	plant := false

	for _, t := range productionBuildings {
		if g.CountReadyUnits(team, t) > 0 {
			plant = true

			break
		}
	}

	if !plant {
		return false
	}

	demand := 1

	if g.StoredCargo(team, economy.CargoFuel) < 8 || g.StoredCargo(team, economy.CargoRaw) < 24 {
		if g.CountReadyUnits(team, assets.MiningSt) == 0 {
			return false // short of materials with no way to mine more
		}

		demand = 2
	}

	power := 0

	g.teamUnits(team, func(u *game.UnitInfo) {
		if !u.Flags.Has(game.FlagStationary) {
			return
		}

		if rate := economy.PowerConsumptionRate(u.Type); rate < 0 {
			power -= rate // a generator's consumption is negative: it makes power
		}
	})

	return power >= demand
}

// HasConstructionInfrastructure reports whether the team can still build: it owns a
// Constructor AND an Engineer AND has the raw material to put them to work. The raw it
// needs is 38, dropping to 8 with a mining station and by a further 8 with a power
// source (so a team with both needs none at all).
//
// C++ ported : scripter.cpp - HasConstructionInfrastructure
func (g *Game) HasConstructionInfrastructure(team int) bool {
	if g.CountReadyUnits(team, assets.Constrct) == 0 || g.CountReadyUnits(team, assets.Engineer) == 0 {
		return false
	}

	needed := 38
	if g.CountReadyUnits(team, assets.MiningSt) > 0 {
		needed = 8
	}

	powered := false

	g.teamUnits(team, func(u *game.UnitInfo) {
		if u.Flags.Has(game.FlagStationary) && economy.PowerConsumptionRate(u.Type) < 0 {
			powered = true
		}
	})

	if powered {
		needed -= 8
	}

	return needed <= 0 || needed <= g.StoredCargo(team, economy.CargoRaw)
}

// CountReadyUnits counts the team's units of one type that are in service. A unit still
// inside the factory that built it (OrderStateBuildingReady) does not count yet.
//
// C++ ported : scripter.cpp - CountReadyUnits
func (g *Game) CountReadyUnits(team int, t assets.UnitType) int {
	n := 0

	g.teamUnits(team, func(u *game.UnitInfo) {
		if u.Type == t && u.State != game.OrderStateBuildingReady {
			n++
		}
	})

	return n
}

// CountCasualties is how many units of one type the team has LOST, the same per type
// tally the Reports screen's Casualties tab lists.
//
// C++ ported : scripter.cpp - CountCasualties (CTInfo.casualties[])
func (g *Game) CountCasualties(team int, t assets.UnitType) int {
	if team < 0 || team >= game.TeamCount {
		return 0
	}

	return g.TeamStats[team].Casualties[t]
}

// StoredCargo is how much of one cargo the team is holding. GOTCHA: the original sums
// the STORAGE OF THE UNITS whose type carries that cargo (a mining station and a
// storage unit hold raw, a fuel tank holds fuel, a gold vault holds gold), NOT the
// complex pool - so a mission asking "have I mined any gold yet" is really asking
// whether anything of the team's is holding gold.
//
// C++ ported : scripter.cpp - HasMaterials
func (g *Game) StoredCargo(team int, cargo economy.CargoType) int {
	sum := 0

	g.teamUnits(team, func(u *game.UnitInfo) {
		if !u.Flags.Has(game.FlagStationary) && !u.InClass(game.ClassMobileLandSea) {
			return
		}

		if economy.CargoTypeOf(u.Type) == cargo {
			sum += u.Storage
		}
	})

	return sum
}

// RawMiningCapacity is the RAW capacity of the team's powered mining stations: what they
// could pull from the deposits under them, whatever they are set to mine right now. The
// Surveying mission is won by raising it, which is why it is raw and not the sum of all
// three ores - the original's function hands back the three separately and the mission
// reads only the first (klei's save format notes agree: "Total Raw Materials Mining > 21",
// and the retail briefing asks for a second station pulling at least 10 raw a turn).
//
// C++ ported : scripter.cpp - CountTotalMining (its raw_mining_max return)
func (g *Game) RawMiningCapacity(team int) int {
	sum := 0

	g.teamUnits(team, func(u *game.UnitInfo) {
		if u.Type == assets.MiningSt && u.Order == game.OrderPowerOn {
			sum += u.RawMiningMax
		}
	})

	return sum
}

// TotalMining is what the team's powered stations actually mine of one cargo per turn
// (the allocation the player set), as against the capacity above.
//
// C++ ported : scripter.cpp - GetTotalMining
func (g *Game) TotalMining(team int, cargo economy.CargoType) int {
	sum := 0

	g.teamUnits(team, func(u *game.UnitInfo) {
		if u.Type != assets.MiningSt || u.Order != game.OrderPowerOn {
			return
		}

		switch cargo {
		case economy.CargoRaw:
			sum += u.RawMining
		case economy.CargoFuel:
			sum += u.FuelMining
		case economy.CargoGold:
			sum += u.GoldMining
		}
	})

	return sum
}

// TotalConsumption is what the team's buildings burn of one cargo per turn: the
// negative half of each building's net production, summed and returned positive.
//
// C++ ported : scripter.cpp - GetTotalConsumption (Cargo_GetNetProduction)
func (g *Game) TotalConsumption(team int, cargo economy.CargoType) int {
	sum := 0

	g.teamUnits(team, func(u *game.UnitInfo) {
		if !u.Flags.Has(game.FlagStationary) {
			return
		}

		net := economy.NetProduction(u, g.MiningDifficulty)

		var draw int

		switch cargo {
		case economy.CargoRaw:
			draw = net.Raw
		case economy.CargoFuel:
			draw = net.Fuel
		case economy.CargoGold:
			draw = net.Gold
		}

		if draw < 0 {
			sum -= draw
		}
	})

	return sum
}

// RunningBuildings counts the team's buildings of one type that are switched on (or
// still going up). GOTCHA: the original calls this one "get_total_power_consumption",
// but it consumes nothing and returns no power - it COUNTS BUILDINGS, and the Mining
// Allocation mission uses it to ask "do I have two plants running".
//
// C++ ported : scripter.cpp - GetTotalPowerConsumption
func (g *Game) RunningBuildings(team int, t assets.UnitType) int {
	n := 0

	g.teamUnits(team, func(u *game.UnitInfo) {
		if u.Type != t || !u.Flags.Has(game.FlagStationary) {
			return
		}

		if u.Order == game.OrderPowerOn || u.Order == game.OrderBuild {
			n++
		}
	})

	return n
}

// UnitsBeingConstructed counts what the team has ON THE WAY of one type: a builder part
// way through raising that building, or a factory turning that unit out. The Material
// Transfer mission is won the moment the turret is UNDER construction, not when it stands.
//
// C++ ported : scripter.cpp - GetTotalUnitsBeingConstructed
func (g *Game) UnitsBeingConstructed(team int, t assets.UnitType) int {
	n := 0

	for _, o := range g.builds {
		if o.Builder != nil && o.Builder.Team == team && o.Product != nil && o.Product.Type == t {
			n++
		}
	}

	g.teamUnits(team, func(u *game.UnitInfo) {
		if !u.Flags.Has(game.FlagStationary) || !g.FactoryProducing(u) {
			return
		}

		if q, _ := g.FactoryQueue(u); len(q) > 0 && q[0] == t {
			n++
		}
	})

	return n
}

// HasUnitAboveMarkLevel reports whether the team fields a land or sea unit of that type
// upgraded past the given Mk level (Mk I is version 1), which is how the Upgrade
// mission knows the player has been to the Upgrade screen.
//
// C++ ported : scripter.cpp - HasUnitAboveMarkLevel
func (g *Game) HasUnitAboveMarkLevel(team int, t assets.UnitType, mark int) bool {
	found := false

	g.teamUnits(team, func(u *game.UnitInfo) {
		if found || u.Type != t || !u.InClass(game.ClassMobileLandSea) || u.Values == nil {
			return
		}

		if u.Values.Version() > mark {
			found = true
		}
	})

	return found
}

// HasUnitExperience reports whether the team has an experienced unit of that type. The Infiltrator mission requires a commando with experience.
//
// C++ ported : scripter.cpp - HasUnitExperience
func (g *Game) HasUnitExperience(team int, t assets.UnitType) bool {
	found := false

	g.teamUnits(team, func(u *game.UnitInfo) {
		if u.Type == t && u.Experience > 0 {
			found = true
		}
	})

	return found
}

// CountDamagedUnits counts the team's units of one type that are not at full health.
// The Repairing mission is won by getting the number back to zero.
//
// C++ ported : scripter.cpp - CountDamangedUnits [sic]
func (g *Game) CountDamagedUnits(team int, t assets.UnitType) int {
	n := 0

	g.teamUnits(team, func(u *game.UnitInfo) {
		if u.Type != t || u.Values == nil {
			return
		}

		if u.Hits != u.Values.Get(game.AttribHits) {
			n++
		}
	})

	return n
}

// CountUnitsWithReducedAttrib counts the team's units of one type that are short of
// their full ammo, speed, shots, hits or storage. The Reloading mission is won by
// getting the ammo count back to zero.
//
// C++ ported : scripter.cpp - CountUnitsWithReducedAttrib
func (g *Game) CountUnitsWithReducedAttrib(team int, t assets.UnitType, a game.Attrib) int {
	n := 0

	g.teamUnits(team, func(u *game.UnitInfo) {
		if u.Type != t || u.Values == nil {
			return
		}

		full := u.Values.Get(a)
		short := false

		switch a {
		case game.AttribAmmo:
			short = u.Ammo != full
		case game.AttribSpeed:
			short = u.Speed != full
		case game.AttribRounds:
			short = u.Shots != full
		case game.AttribHits:
			short = u.Hits != full
		case game.AttribStorage:
			short = u.Storage != full
		}

		if short {
			n++
		}
	})

	return n
}

// ResearchLevel is the team's achieved level in one research topic.
//
// C++ ported : scripter.cpp - GetResearchLevel
func (g *Game) ResearchLevel(team, topic int) int {
	if team < 0 || team >= game.TeamCount || topic < 0 || topic >= game.ResearchTopicCount {
		return 0
	}

	return g.State.Teams[team].Research[topic].Level
}

// Points returns the team's Eco Sphere colonisation score from the live game state.
//
// C++ ported : maxregistryhandler.cpp - MAX_REGISTRY.<colour>TeamPoints
func (g *Game) Points(team int) int {
	if team < 0 || team >= game.TeamCount {
		return 0
	}

	return g.TeamPoints[team]
}

// TurnValue is the turn the game is on.
//
// C++ ported : maxregistryhandler.cpp - MAX_REGISTRY.TeamTurnValue
func (g *Game) TurnValue() int { return g.State.TurnCounter }

// TurnVictoryLimit is the turn cap a Duration game ends on (0 when the game has none).
//
// C++ ported : maxregistryhandler.cpp - MAX_REGISTRY.TeamTurnVictoryLimit
func (g *Game) TurnVictoryLimit() int {
	if g.VictoryType != winloss.VictoryDuration {
		return 0
	}

	return g.VictoryLimit
}
