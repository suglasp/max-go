// buildunits.go : the M.A.X. unit building logic for the AI.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// producible is one unit a factory can turn out, with the movement/render flags
// it deploys with. C++: the per factory build list derived from the unit data.
type producible struct {
	t     assets.UnitType
	flags game.Flags
}

// Flag shorthands for the producible tables.
const (
	flagLandUnit = game.FlagMobileLandUnit | game.FlagSelectable
	flagAirUnit  = game.FlagMobileAirUnit | game.FlagHovering | game.FlagSelectable
	flagSeaUnit  = game.FlagMobileSeaUnit | game.FlagSelectable
)

// factoryProducts maps a factory type to the units it can build, in rough
// priority order (the production manager picks the first one the strategy
// favours). This is the tractable stand in for the original's data driven factory
// build lists. C++: the unit data's buildable by factory tables.
// The factory->unit assignments follow the manual authoritative player roster
// (menus/factorybuild.go factoryBuild, manual p.28-30): the Scanner is a HEAVY unit
// (LandPlt), SP Flak a LIGHT unit (LightPlt), and infantry trains at the TRAINING
// HALL (the Barracks is a service building, not a factory). The AI keeps a
// combat focused SUBSET of each plant's full player list.
var factoryProducts = map[assets.UnitType][]producible{
	assets.LandPlt: { // heavy ground: armour + artillery + the mobile radar (Scanner)
		{assets.Tank, flagLandUnit | game.FlagTurretSprite | game.FlagCanHeadRotate},
		{assets.Artillry, flagLandUnit},
		{assets.RocktLch, flagLandUnit},
		{assets.Scanner, flagLandUnit | game.FlagSpinningTurret},
	},
	assets.LightPlt: { // light ground: scouts + support + light AA (SP Flak)
		{assets.Scout, flagLandUnit},
		{assets.SplyTrck, flagLandUnit},
		{assets.MineLayr, flagLandUnit},
		{assets.SpFlak, flagLandUnit},
	},
	assets.AirPlt: { // air wing: fighter + bomber + radar plane
		{assets.Fighter, flagAirUnit},
		{assets.Bomber, flagAirUnit},
		{assets.Awac, flagAirUnit | game.FlagSpinningTurret},
	},
	assets.Shipyard: { // naval
		{assets.Corvette, flagSeaUnit},
		{assets.Submarne, flagSeaUnit},
		{assets.MsslBoat, flagSeaUnit},
		{assets.FastBoat, flagSeaUnit},
	},
	assets.TrainHal: { // infantry (the Training Hall, NOT the Barracks)
		{assets.Infantry, flagLandUnit},
		{assets.Commando, flagLandUnit},
	},
}

// reconDemand lists the recon units the team should keep a minimum of, with the
// factory that builds each: a Surveyor for resource recon (it feeds taskSurvey ->
// depositScore, and is in NO balanced factoryProducts list, so nothing else
// rebuilds it once the starting one dies) and a Scanner for enemy recon (a mobile
// radar). taskProduceUnits.ensureRecon produces one when the team is short, before
// generic production, so the AI never goes blind. C++ ported : aiplayer.cpp -
// CheckReconnaissanceNeeds (Scanner) + taskfindmines.cpp - TaskObtainUnits(SURVEYOR)
// (reduced to a fixed minimum count instead of the scan vs enemy range calc).
var reconDemand = []reconNeed{
	{assets.Surveyor, assets.LightPlt, flagLandUnit, 1},                         // resource recon (Light plant)
	{assets.Scanner, assets.LandPlt, flagLandUnit | game.FlagSpinningTurret, 1}, // enemy recon (Heavy plant: Scanner is a heavy unit)
}

// reconNeed is one recon unit the team must keep fielded: the unit, the plant
// that builds it, its deploy flags and the minimum count.
type reconNeed struct {
	unit    assets.UnitType
	factory assets.UnitType
	flags   game.Flags
	min     int
}

// isFactory reports whether a unit type produces mobile units (has a product
// list). C++: the FACTORY class building check.
func isFactory(t assets.UnitType) bool {
	_, ok := factoryProducts[t]

	return ok
}

// strategyWeights biases each strategy's production mix: a unit's weight is roughly how
// many of it the team fields per one unweighted (weight 1) unit, so a higher weight
// means that type accumulates faster. Types absent from a strategy's row keep the
// baseline weight 1, so a strategy never STARVES a factory's other products - it only
// SKEWS the mix toward its signature units (an Air team flies more fighters/bombers, a
// Tank Horde rolls more armour, a Sea team launches more boats). CombinedArms / Random /
// Defensive carry no entries, so they keep the even spread the base balance already gave
// (Defensive leans on its turrets/buildings, not a skewed mobile roster). Recon units
// (Surveyor / Scanner) are produced on demand by ensureRecon, not weighted here. C++:
// the per strategy unit build weighting AiPlayer applies when filling its factories.
var strategyWeights = map[Strategy]map[assets.UnitType]int{
	StrategyTankHorde:  {assets.Tank: 4, assets.Scout: 2},
	StrategyAir:        {assets.Fighter: 3, assets.Bomber: 3},
	StrategySea:        {assets.Corvette: 3, assets.MsslBoat: 3, assets.Submarne: 2, assets.FastBoat: 2},
	StrategyScoutHorde: {assets.Scout: 4, assets.Fighter: 2},
	StrategyFastAttack: {assets.Scout: 3, assets.Fighter: 2, assets.Tank: 2, assets.FastBoat: 2},
	StrategyMissiles:   {assets.RocktLch: 3, assets.MsslBoat: 3, assets.Artillry: 2},
	StrategyEspionage:  {assets.Commando: 3},
}

// productWeight is the production weight of unit type t under strategy s (baseline 1).
func productWeight(s Strategy, t assets.UnitType) int {
	if row, ok := strategyWeights[s]; ok {
		if w := row[t]; w > 0 {
			return w
		}
	}

	return 1
}

// chooseProduct picks which unit a factory should build this turn, biased by the team's
// strategy: an air strategy favours the air wing, sea favours naval, a tank horde
// favours armour, etc. It picks the product with the smallest count to weight ratio, so
// a strategy's favoured (heavier weighted) units are allowed to accumulate
// proportionally more before another type is chosen - the army composition then tracks
// the strategy weights instead of an even split, while every type still gets built (none
// is starved). Ties keep list (priority) order. needed, when set, rules a type out
// (the "is this unit needed" check); no product left = nothing to build. C++: the
// AiPlayer build order weighting that drives TaskCreateUnit, gated by
// TaskManager::IsUnitNeeded.
func chooseProduct(factory assets.UnitType, strategy Strategy, mix func(assets.UnitType) int, needed func(assets.UnitType) bool) (producible, bool) {
	var list []producible
	for _, p := range factoryProducts[factory] {
		// A type with an idle unit is not built again.
		if needed != nil && !needed(p.t) {
			continue
		}
		list = append(list, p)
	}

	if len(list) == 0 {
		return producible{}, false
	}

	best := list[0]
	bestCount, bestWeight := mix(best.t), productWeight(strategy, best.t)

	for _, p := range list[1:] {
		c, w := mix(p.t), productWeight(strategy, p.t)

		// Smaller ratio wins: c/w < bestCount/bestWeight, compared by cross multiplication
		// to stay in integers. Strict < keeps the earlier (higher priority) entry on a tie.
		if c*bestWeight < bestCount*w {
			best, bestCount, bestWeight = p, c, w
		}
	}

	return best, true
}
