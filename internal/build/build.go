// build.go : handles placement checks and construction progress.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// Package build validates sites, turns raw material into finished units, and
// forms complexes from adjacent structures.
//
// Completed builds return an unplaced *game.UnitInfo for internal/sim to deploy.
// Avoiding an import of units keeps the dependency graph acyclic.
package build

import (
	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
)

// Footprint is the grid edge length a unit occupies when placed: buildings
// take a 2x2 block, everything else a single cell.
//
// C++: the BUILDING flag selecting the 2x2 vs 1x1 placement checks
// (UnitsManager_IsAccessible / Access_IsAccessible).
func Footprint(flags game.Flags) int {
	if flags.Has(game.FlagBuilding) {
		return 2
	}

	return 1
}

// SiteValid reports whether a unit described by d may be placed with its
// top left corner at (x, y): every cell of its footprint must be accessible to
// the team. The caller handles slab requirements and clears constructor sites
// during the landing sequence.
//
// C++: UnitsManager_IsAccessible (units_manager.cpp): the Access_IsAccessible
// check applied to every footprint cell.
func SiteValid(g *access.Grid, d access.UnitDesc, team, x, y int) bool {
	return SiteValidMod(g, d, team, x, y, access.SameClassBlocks)
}

// SiteValidMod is SiteValid with an explicit accessibility modifier. The engineer
// Path build arms with access.SameClassBlocks|access.IgnoreMobileUnits so a cell a
// mobile unit is merely parked on stays in the run (only terrain and permanent
// structures drop a cell); the run's build step later waits for the unit to clear.
func SiteValidMod(g *access.Grid, d access.UnitDesc, team, x, y int, mod access.Modifier) bool {
	n := Footprint(d.Flags)

	for dy := 0; dy < n; dy++ {
		for dx := 0; dx < n; dx++ {
			if !g.IsAccessible(d, team, x+dx, y+dy, mod).Passable() {
				return false
			}
		}
	}

	return true
}

// repType is the builder rate category for product: a building bills the
// constructor rate, infantry/commando the training hall rate, and any other
// mobile unit the factory rate. The raw draw each turn scales from this category
// via RawConsumptionRate. In normal play this equals the actual builder's type
// (a constructor builds buildings, a factory builds vehicles, a training hall
// builds infantry), which is what the draw each turn keys on.
//
// C++: the category split shared by UnitInfo::GetNormalRateBuildCost and the
// Cargo_GetRawConsumptionRate(unit_type, rate) draw in UnitInfo::UpdateProduction.
func repType(product *game.UnitInfo) assets.UnitType {
	switch {
	case product.Flags.Has(game.FlagStationary):
		return assets.Constrct

	case product.Type == assets.Commando || product.Type == assets.Infantry:
		return assets.TrainHal

	default:
		return assets.LandPlt
	}
}

// baseRate is the raw material a build consumes each build turn at rate 1,
// selected by the product's category: a building costs the constructor rate
// (2), infantry/commando the training hall rate (1), and other mobile units
// the factory rate (3).
//
// C++: UnitInfo::GetNormalRateBuildCost's category split (unitinfo.cpp).
func baseRate(product *game.UnitInfo) int {
	return economy.RawConsumptionRate(repType(product), 1)
}

// Cost is the total raw material to build product: its base rate each turn times
// its build time (ATTRIB_TURNS).
//
// C++: UnitInfo::GetNormalRateBuildCost (unitinfo.cpp).
func Cost(product *game.UnitInfo) int {
	if product.Values == nil {
		return 0
	}

	return baseRate(product) * product.Values.Get(game.AttribTurns)
}

// BuildOrder drives one construction: a builder (factory or constructor)
// turning raw material into product over several turns. BuildRate is the speed
// multiplier (1 normal, 2 double, 4 quad); Progress counts build turns
// accumulated toward the product's TotalTurns.
//
// C++: the ORDER_BUILD state machine on UnitInfo (unitinfo.cpp / units_
// manager.cpp), reduced to the material and progress core each turn.
type BuildOrder struct {
	Builder      *game.UnitInfo
	Product      *game.UnitInfo
	GridX, GridY int
	BuildRate    int
	Progress     int
	TotalTurns   int
	Started      bool // Started latches true once a mobile constructor has reached the site and begun work.
}

// NewBuildOrder starts building product at (gx, gy) at the given rate. The
// product is already built but not placed (its type, flags and base
// values already wired); it is deployed by the caller when the order finishes.
func NewBuildOrder(builder, product *game.UnitInfo, gx, gy, rate int) *BuildOrder {
	if rate < 1 {
		rate = 1
	}

	turns := 0
	if product.Values != nil {
		turns = product.Values.Get(game.AttribTurns)
	}

	if turns < 1 {
		turns = 1
	}

	return &BuildOrder{
		Builder:    builder,
		Product:    product,
		GridX:      gx,
		GridY:      gy,
		BuildRate:  rate,
		TotalTurns: turns,
	}
}

// CostPerTurn is the raw material this order draws this turn at its build rate.
// Building faster grows faster than linear: rate 1/2/4 draws the category rate
// times 1/4/12, not a flat multiple, so a rushed build costs more total material.
//
// C++ ported : unitinfo.cpp - Cargo_GetRawConsumptionRate(unit_type, build_rate)
// (the draw each turn in UnitInfo::UpdateProduction / the factory StartBuilding).
func (o *BuildOrder) CostPerTurn() int {
	return economy.RawConsumptionRate(repType(o.Product), o.BuildRate)
}

// Done reports whether the product is finished.
func (o *BuildOrder) Done() bool {
	return o.Progress >= o.TotalTurns
}

// TurnsLeft is how many more build turns until the product finishes at the current
// build rate, rounded up (a rate 2 order with 3 turns of work left finishes in 2);
// 0 once done. Used for the "N turns left" status readout when a builder is selected.
func (o *BuildOrder) TurnsLeft() int {
	remaining := o.TotalTurns - o.Progress
	if remaining <= 0 {
		return 0
	}
	rate := o.BuildRate
	if rate < 1 {
		rate = 1
	}
	return (remaining + rate - 1) / rate
}

// Advance applies one turn of construction, drawing this turn's material from
// *pool. It first throttles the build rate down to what *pool can sustain for the
// work still remaining (the rate is clamped again every turn), so a build that can
// no longer afford its chosen speed slows instead of stalling outright and finishes
// near empty. It reports whether progress was made: if the order is
// finished or the pool cannot cover even one turn at rate 1, nothing is consumed
// and it returns false (the build stalls until material is available).
//
// C++ ported : unitinfo.cpp - UnitInfo::UpdateProduction (build_rate =
// min(build_rate, BuildMenu_GetMaxPossibleBuildRate(storage)); storage -=
// Cargo_GetRawConsumptionRate(unit_type, build_rate)).
func (o *BuildOrder) Advance(pool *int) bool {
	if o.Done() {
		return false
	}

	// Slow the rate to what the pool sustains for the remaining work (never below
	// 1). This ratchets down: a build that hit a shortage keeps the slower speed.
	remaining := o.TotalTurns - o.Progress
	if lim := MaxBuildRate(repType(o.Product), remaining, *pool); o.BuildRate > lim {
		o.BuildRate = lim
	}
	if o.BuildRate < 1 {
		o.BuildRate = 1
	}

	cost := o.CostPerTurn()
	if *pool < cost {
		return false
	}

	*pool -= cost
	o.Progress += o.BuildRate

	if o.Progress > o.TotalTurns {
		o.Progress = o.TotalTurns
	}

	return true
}

// MaxBuildRate is the fastest build speed (1, 2 or 4) a builder holding storage
// raw can sustain over a job of buildTime turns. Because a faster rate draws raw
// faster than linearly (rate 1/2/4 -> the builder rate times 1/4/12), a small
// stockpile caps the speed. Shared by the sim (the throttle each turn) and the
// build menu (which rate buttons are live + the turns/cost preview), so both agree.
//
// C++ ported : buildmenu.cpp - BuildMenu_GetMaxPossibleBuildRate.
func MaxBuildRate(builder assets.UnitType, buildTime, storage int) int {
	if buildTime <= 1 {
		return 1
	}
	cost1 := economy.RawConsumptionRate(builder, 1)
	cost2 := economy.RawConsumptionRate(builder, 2)
	cost4 := economy.RawConsumptionRate(builder, 4)
	turns1 := buildTime
	turns2, turns4 := 0, 0
	change2 := cost2 - cost1*2
	for cost := turns1*cost1 + change2; cost <= storage; cost += change2 {
		if turns1 < 2 {
			break
		}
		turns2++
		turns1 -= 2
	}
	change4 := cost4 - cost2*2
	for cost := turns1*cost1 + turns2*cost2 + change4; cost <= storage; cost += change4 {
		if turns2 < 2 {
			if turns2 > 0 && turns1 == 1 && cost-cost2-cost1+cost4 <= storage {
				turns4++
			}
			break
		}
		turns4++
		turns2 -= 2
	}
	switch {
	case turns4 > 0:
		return 4
	case turns2 > 0:
		return 2
	default:
		return 1
	}
}
