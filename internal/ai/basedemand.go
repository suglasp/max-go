// basedemand.go : the M.A.X. base demand logic for the AI.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
)

// aiDifficultyFactor is the mining yield multiplier the AI plans against (4 = the
// human/easy baseline; economy.NetProduction scales a station's output by
// factor/4). It is set per difficulty tier by applyDifficulty (AVERAGE = 4, up to
// 6 at GOD); a package var, defaulting to the AVERAGE value. See difficulty.go.
var aiDifficultyFactor = 4

// teamEconomy is a one turn snapshot of a team's base economy, summed over its
// finished stationary buildings. It drives the demand based building roster - the
// reduced port of the accounting TaskManageBuildings does across CheckNeeds /
// CheckPower / CheckWorkers / EndTurn before it decides what to build next.
type teamEconomy struct {
	netRaw, netFuel, netGold int // net per turn production (mining minus upkeep/build draw)
	power                    int // summed power draw: > 0 means consumers outstrip generators
	life                     int // summed life draw: > 0 means workers outstrip habitat support
	miningStations           int // how many mining stations the team already fields

	rawStored, rawCap   int // raw storage fill vs capacity
	fuelStored, fuelCap int // fuel storage fill vs capacity
	goldStored, goldCap int // gold storage fill vs capacity
}

// surveyTeamEconomy accumulates a team's base economy from its finished
// buildings (under construction buildings are not deployed yet, so they never
// appear here). Power and life are summed straight from the per type rates the
// way the original does, while raw/fuel/gold use economy.NetProduction so a mining
// station's yield is netted against running costs. C++ ported :
// taskmanagebuildings.cpp - the per unit sweeps in UpdateMiningNeeds / CheckPower
// / CheckWorkers / EndTurn.
func surveyTeamEconomy(g *sim.Game, team int) teamEconomy {
	var e teamEconomy

	for _, u := range g.Units.Units() {
		if u.Team != team || !u.Flags.Has(game.FlagStationary) || u.Flags.Has(game.FlagGroundCover) {
			continue
		}

		net := economy.NetProduction(u, aiDifficultyFactor)
		e.netRaw += net.Raw
		e.netFuel += net.Fuel
		e.netGold += net.Gold
		e.power += economy.PowerConsumptionRate(u.Type)
		e.life += economy.LifeConsumptionRate(u.Type)

		if u.Type == assets.MiningSt {
			e.miningStations++
		}

		inv := economy.Inventory(u)
		capacity := economy.Capacity(u)
		e.rawStored, e.rawCap = e.rawStored+inv.Raw, e.rawCap+capacity.Raw
		e.fuelStored, e.fuelCap = e.fuelStored+inv.Fuel, e.fuelCap+capacity.Fuel
		e.goldStored, e.goldCap = e.goldStored+inv.Gold, e.goldCap+capacity.Gold
	}

	return e
}

// needsMining reports whether the team should expand its resource base: it has no
// mining station yet, or its raw/fuel production cannot cover consumption. A
// station mines whatever lies under it, so a single generic check covers both raw
// and fuel shortfalls. C++ ported : taskmanagebuildings.cpp -
// TaskManageBuildings::UpdateMiningNeeds (the cargo_demand < 0 -> build MININGST).
func needsMining(e teamEconomy) bool {
	return e.miningStations == 0 || e.netRaw <= 0 || e.netFuel <= 0
}

// storageFull reports whether a cargo type has filled past three quarters of its
// capacity, the threshold at which the original adds another storage dump so mined
// resources are not wasted. C++ ported : taskmanagebuildings.cpp -
// TaskManageBuildings::EndTurn (cargo >= capacity * 3 / 4).
func storageFull(stored, capacity int) bool {
	return capacity > 0 && stored*4 >= capacity*3
}

// storageCapLimit is the AVERAGE tier's per cargo storage cap and the value the knob
// defaults to before a difficulty is applied: once a cargo type's capacity reaches it
// the AI stops adding dumps. Without a cap a base that mines faster than it spends
// builds raw dumps forever - redistribute tops every store to the brim from the
// overflowing pool, so the base stays >3/4 full and each new dump just triggers the
// next. C++ : taskmanagebuildings.cpp - EndTurn (capacity.* < 500).
const storageCapLimit = 500

// aiStorageCap is the ACTIVE per cargo storage cap the demand check reads. It is set
// per difficulty tier by applyDifficulty (difficulty.go): a harder AI keeps a LOWER
// cap so mined material flows into military production instead of idle dumps, while an easier
// one uses a higher cap and invests more heavily in economy. Defaults to the AVERAGE value, so
// a test that never applies a tier sees the pre difficulty behaviour.
var aiStorageCap = storageCapLimit

// needsStorage reports whether the team should add a storage dump for a cargo type: it
// is actually mining (so a surplus accrues to hold), the type has filled past three
// quarters, AND its total capacity is still under the active cap (aiStorageCap, which
// is difficulty scaled). The cap is what stops the runaway dump building. C++ ported :
// taskmanagebuildings.cpp - TaskManageBuildings::EndTurn (raw_mining_max > 0 && cargo >=
// capacity*3/4 && capacity < 500; the per cargo raw/fuel/gold_mining_max gate is reduced
// here to "the team fields a mining station").
func needsStorage(stored, capacity int, mining bool) bool {
	return mining && capacity < aiStorageCap && storageFull(stored, capacity)
}

// pendingBuild reports whether the team already has a build order in flight for a
// type, so the manager queues at most one of a kind per growth cycle instead of
// flooding every idle builder with the same project. C++: the tasks list scan
// TaskManageBuildings does before issuing a new CreateBuilding.
func pendingBuild(g *sim.Game, team int, t assets.UnitType) bool {
	for _, o := range g.BuildOrders() {
		if o.Builder != nil && o.Builder.Team == team && o.Product != nil && o.Product.Type == t {
			return true
		}
	}

	return false
}
