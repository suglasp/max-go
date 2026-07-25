// buildorders.go : the M.A.X. build order logic for the AI.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
)

// buildSpec is one entry of a strategy's desired base: build the type until the
// team owns `want` of them (counting those under construction). `large` is the
// footprint (2x2 building vs 1x1). C++: the BuildOrder entries that
// AiPlayer::ChooseStrategicBuildOrders emits.
type buildSpec struct {
	t     assets.UnitType
	large bool
	want  int
}

// strategicBuildOrders returns the team's desired buildings in priority order,
// biased by its strategy, win goal and terrain. This is the tractable stand in for
// the original's continent- and difficulty-scored ChooseStrategicBuildOrders: a fixed
// growth plan (power + production first, then the strategy's signature plant, then
// support) that the build manager walks top to bottom. A coastal team also fields a
// shipyard (and so a navy); a colonising team races eco spheres for the score win. The richer
// C++: AiPlayer::ChooseStrategicBuildOrders / ProcessBuildOrders.
func strategicBuildOrders(strategy Strategy, goal winGoal, coastal bool) []buildSpec {
	// Common economic backbone every team grows toward. Power is no longer listed
	// here: it is demand driven (a constructor raises a 2x2 Power Station, an
	// engineer a 1x1 Power Generator), so the backbone stays all 2x2 - which is what
	// a constructor can actually build.
	orders := []buildSpec{
		{assets.LandPlt, true, 1}, // a heavy vehicle plant (ground units)
	}

	// The strategy's signature production building leads the rest. A naval (Sea) plan
	// only makes sense on the coast; landlocked, it builds an air plant instead.
	switch strategy {
	case StrategyAir:
		orders = append(orders, buildSpec{assets.AirPlt, true, 1})
	case StrategySea:
		if !coastal {
			orders = append(orders, buildSpec{assets.AirPlt, true, 1})
		}
	default:
		orders = append(orders, buildSpec{assets.AirPlt, true, 1})
	}

	orders = append(orders, buildSpec{assets.Depot, true, 1})

	// A coastal team fields a navy: one shipyard (this is the SINGLE place the dock is
	// listed). A landlocked team never builds one - an unusable dock - and a landlocked
	// Sea strategy team fell back to an air plant above.
	if coastal {
		orders = append(orders, buildSpec{assets.Shipyard, true, 1})
	}

	// A colonising team races to raise eco spheres: each powered Eco Sphere adds team
	// points every turn, so several of them out score a purely military rival (the
	// score / "colonise the planet" win). They sit above the research lab in priority -
	// points matter more than research to a coloniser. The power and habitat the
	// eco spheres consume are added reactively by the build manager's demand tiers.
	if goal == goalColonize {
		orders = append(orders, buildSpec{assets.Greenhse, true, colonizeEcoSpheres})
	}

	// The LIGHT plant, which the AI never used to raise at all - so it could field no
	// scout, no supply truck, no mine layer and, worst of all, no anti air gun. A team
	// with nothing that shoots upward is free lunch for a single bomber.
	orders = append(orders, buildSpec{assets.LightPlt, true, 1})

	// Research lab.
	orders = append(orders, buildSpec{assets.Research, true, 1})

	// The Training Hall, for infantry and commandos. It sits last because a vehicle army
	// comes first, but a team that never builds one can never take a building intact.
	orders = append(orders, buildSpec{assets.TrainHal, true, 1})

	// SECOND heavy and air plants, once the base above stands. One plant turns out one
	// unit at a time, so a single line was the ceiling on the whole army: the AI fielded
	// a handful of units by the time a human had a column. These come LAST, so a second
	// line is only started after the economy, the storage and the first of everything are
	// in place - a poor team never reaches them.
	orders = append(orders, buildSpec{assets.LandPlt, true, 2})

	if strategy != StrategySea || !coastal {
		orders = append(orders, buildSpec{assets.AirPlt, true, 2})
	}

	if coastal {
		orders = append(orders, buildSpec{assets.Shipyard, true, 2})
	}

	return orders
}

// nextBuilding picks the highest priority building the team still needs, or
// false if every desired count is met. It counts both finished structures and
// orders already in progress so the manager does not double queue. C++: the
// "is this unit type needed" check in TaskCreateBuilding / TaskManager.
func nextBuilding(g *sim.Game, team int, orders []buildSpec) (buildSpec, bool) {
	for _, spec := range orders {
		if buildingCount(g, team, spec.t) < spec.want {
			return spec, true
		}
	}

	return buildSpec{}, false
}

// buildingCount tallies a team's structures of a type, including ones still
// under construction (so an in flight order counts against the desired total).
func buildingCount(g *sim.Game, team int, t assets.UnitType) int {
	n := 0
	for _, u := range g.Units.Units() {
		if u.Team == team && u.Type == t {
			n++
		}
	}

	for _, o := range g.BuildOrders() {
		if o.Builder != nil && o.Builder.Team == team && o.Product != nil && o.Product.Type == t {
			n++
		}
	}

	return n
}

// buildingFlags is the flag set every constructed structure carries: stationary,
// needs a slab, selectable, plus the 2x2 building bit for large footprints.
// C++: the REQUIRES_SLAB / BUILDING flag bits in the unit base values.
func buildingFlags(large bool) game.Flags {
	f := game.FlagStationary | game.FlagRequiresSlab | game.FlagSelectable

	if large {
		f |= game.FlagBuilding
	}

	return f
}

// buildFlagsFor is buildingFlags specialised by unit type: a terrain special
// (connector / bridge / water platform) lays NO slab - a visible slab would be a
// road - so it drops FlagRequiresSlab, exactly as the player's build flow does.
// Every other type keeps the normal building flags.
// C++ ported : gamescreen buildDesc - CNCT_4W/WTRPLTFM/BRIDGE excluded from the
// REQUIRES_SLAB slab deploy (game_manager.cpp).
func buildFlagsFor(t assets.UnitType, large bool) game.Flags {
	f := buildingFlags(large)

	if t == assets.Cnct4W || t == assets.WtrPltfm || t == assets.Bridge {
		f &^= game.FlagRequiresSlab
	}

	if game.CanHeadRotateType(t) {
		f |= game.FlagTurretSprite | game.FlagCanHeadRotate
	}

	return f
}
