// access.go : determines whether a unit may occupy a cell and its movement cost.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : access.cpp - Access_Is* / surface type queries
package access

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/world"
)

// Cost is a passability result: 0 blocked, else a movement cost class.
//
// C++ ported : access.cpp - Access_IsAccessible (return value)
type Cost int

const (
	Blocked Cost = 0  // impassable
	Reduced Cost = 2  // passable at reduced cost (road / slab / paved bridge)
	Clear   Cost = 4  // normally passable
	Water   Cost = 12 // a land unit crossing open water (amphibious only; 4|8)
)

// Passable reports whether the cost is not Blocked.
func (c Cost) Passable() bool {
	return c != Blocked
}

// Modifier flags tune a passability query.
//
// C++ ported : enums.hpp - AccessModifier_*
type Modifier uint32

const (
	NoModifiers          Modifier = 0x00
	EnemySameClassBlocks Modifier = 0x01 // an enemy unit of the same movement class blocks
	SameClassBlocks      Modifier = 0x02 // any unit of the same movement class blocks
	IgnoreMobileUnits    Modifier = 0x04 // skip mobile units: only terrain and permanent structures decide the cell
	MovesUnderBridge     Modifier = 0x08 // the mover passes under (not over) bridges
	IgnoreVisibility     Modifier = 0x10 // consider units the team cannot see
	IgnoreSurfaceClass   Modifier = 0x20 // skip the surface/unit checks entirely
)

// UnitDesc describes a moving unit for a passability query: its type, capability flags, and the surface mask it may occupy.
//
// C++ ported : access.cpp - ResourceManager_GetUnit flags + land type
type UnitDesc struct {
	Type     assets.UnitType
	Flags    game.Flags
	LandType uint8 // mask of world.Surface* the unit may occupy
}

// LandTypeFor returns the surface mask a unit may occupy from its movement flags.
// COAST is the shoreline transition band. Only amphibious units carry both land and sea flags, so boats stay in deep water and tanks stay off the beach.
// The amphibious case comes before the sea-only case so it keeps land access.
//
// C++ ported : the per-unit land_type attribute (units.cpp - ParseLandType),
// reconstructed from the movement class.
func LandTypeFor(flags game.Flags) uint8 {
	switch {
	case flags.Has(game.FlagMobileAirUnit):
		return world.SurfaceLand | world.SurfaceWater | world.SurfaceCoast | world.SurfaceAir

	case flags.Has(game.FlagMobileLandUnit) && flags.Has(game.FlagMobileSeaUnit):
		return world.SurfaceLand | world.SurfaceWater | world.SurfaceCoast // amphibious crosses the shore

	case flags.Has(game.FlagMobileSeaUnit):
		return world.SurfaceWater // deep water only - a pure sea unit cannot enter the coast band

	default:
		return world.SurfaceLand
	}
}

// IsWaterBuilding reports whether a stationary building is placed on open water instead of land.
// Shipyard, Dock, and Water Platform use SURFACE_TYPE_WATER. Other buildings are land buildings and may stand on a Water Platform, whose cell reads as land.
//
// C++ ported : the land_type == SURFACE_TYPE_WATER test (taskmanagebuildings.cpp).
func IsWaterBuilding(t assets.UnitType) bool {
	switch t {
	case assets.Shipyard, assets.Dock, assets.WtrPltfm:
		return true

	default:
		return false
	}
}

// IsTerrainSpecial identifies flat structures that finish without an exit animation: Road, Connector, Bridge, and Water Platform.
// Their builder is free immediately and may continue along a build path. Access_DestroyUtilities also preserves this set when remove_road is false.
//
// C++ ported : unitinfo.cpp - the ROAD/BRIDGE/WTRPLTFM/CNCT_4W branch in the
// build complete handler frees the builder at once (no ORDER_AWAIT_TAPE_POSITIONING).
func IsTerrainSpecial(t assets.UnitType) bool {
	switch t {
	case assets.Road, assets.Cnct4W, assets.Bridge, assets.WtrPltfm:
		return true

	default:
		return false
	}
}

// LandTypeForUnit applies per-type overrides that movement flags alone cannot express.
// The Surveyor carries MOBILE_LAND_UNIT but also crosses open water at normal cost. Shipyard, Dock, and Water Platform use a water land type.
// Callers that know the unit type should prefer it to LandTypeFor.
//
// C++ ported : access.cpp - Access_IsAccessible (unit_type != SURVEYOR branch) +
// the per type land_type entries in the unit data (units.cpp - ParseLandType).
func LandTypeForUnit(t assets.UnitType, flags game.Flags) uint8 {
	switch {
	case t == assets.Surveyor || t == assets.Constrct || t == assets.Engineer || t == assets.Scout:
		// The Surveyor, the two land builders (Constructor, Engineer) and the Scout
		// carry only MOBILE_LAND_UNIT, but their land_type includes water so they drive
		// onto open water and cross bridges on top. They still pay the +8 amphibious cost
		// below; only the Surveyor is exempt. Movement rule only: the Surveyor swaps to its
		// water sprite on open water (units/sprite.go isAmphibious); the Engineer/Constructor
		// water sprite swap is still TODO (build pose interleave).
		return world.SurfaceLand | world.SurfaceWater | world.SurfaceCoast

	case t == assets.Cnct4W:
		// A connector can be laid on land, coast or open water: the engineer builds it
		// to bridge a shore / water building (a Shipyard) into the base complex.
		// C++: the CONNECTOR land_type spans land + coast + water.
		return world.SurfaceLand | world.SurfaceWater | world.SurfaceCoast

	case IsWaterBuilding(t):
		// A naval building sits on open water only.
		return world.SurfaceWater

	case t == assets.Bridge:
		// A bridge is laid across open water or the coast band (land units cross over it,
		// sea units pass under a raised one), so it can start at the shoreline and span out
		// to bridge land across to water platforms. A water platform stays water only
		// (IsWaterBuilding above); a bridge spans both.
		// C++: a bridge deploys on SURFACE_TYPE_WATER || SURFACE_TYPE_COAST
		// (game_manager.cpp:4767, DeployConstructionSiteUtilities treats water+coast alike).
		return world.SurfaceWater | world.SurfaceCoast

	default:
		return LandTypeFor(flags)
	}
}

// SurfaceSource yields the raw terrain surface at a cell (*world.Map satisfies it).
//
// C++ ported : World::GetSurfaceType
type SurfaceSource interface {
	SurfaceAt(x, y int) uint8
}

// UnitSource yields the units standing on a cell (*units.MapHash satisfies it).
//
// C++ ported : Hash_MapHash
type UnitSource interface {
	At(x, y int) []*game.UnitInfo
}

// Grid bundles the map dimensions, terrain surface and cell unit index the queries read.
// Units may be nil to query bare terrain.
type Grid struct {
	W, H  int
	Surf  SurfaceSource
	Units UnitSource
}

// InBounds reports whether (x, y) is a valid cell.
func (g *Grid) InBounds(x, y int) bool {
	return x >= 0 && x < g.W && y >= 0 && y < g.H
}

// surfaceAt returns the raw surface type at a cell, SurfaceNone off map.
//
// C++ ported : access.cpp - Access_GetSurfaceType
func (g *Grid) surfaceAt(x, y int) uint8 {
	if !g.InBounds(x, y) || g.Surf == nil {
		return world.SurfaceNone
	}

	return g.Surf.SurfaceAt(x, y)
}

// unitsAt returns the units on a cell (nil when none).
func (g *Grid) unitsAt(x, y int) []*game.UnitInfo {
	if g.Units == nil {
		return nil
	}

	return g.Units.At(x, y)
}

// ModifiedSurface returns the surface a unit perceives at a cell.
// A water platform, and a bridge for a land mover, reads as land. mode true selects the sea-mover view.
//
// C++ ported : access.cpp - Access_GetModifiedSurfaceType
func (g *Grid) ModifiedSurface(x, y int, mode bool) uint8 {
	surface := g.surfaceAt(x, y)

	if surface != world.SurfaceWater && surface != world.SurfaceCoast {
		return surface
	}

	for _, u := range g.unitsAt(x, y) {
		switch {
		case mode:
			// Sea view: a platform makes the cell land; a bridge leaves water below.
			if u.Type == assets.WtrPltfm {
				return world.SurfaceLand
			}
			if u.Type == assets.Bridge {
				return surface
			}

		default:
			// Land view: platform or bridge reads as land.
			if u.Type == assets.WtrPltfm || u.Type == assets.Bridge {
				return world.SurfaceLand
			}
		}
	}

	return surface
}

// IsAccessible reports whether unit d, owned by team, may occupy cell (x, y) and its movement cost.
// It does not model visibility or stealth, so every unit is visible. Bridges are not elevated.
//
// C++ ported : access.cpp - Access_IsAccessible
func (g *Grid) IsAccessible(d UnitDesc, team, x, y int, mod Modifier) Cost {
	if !g.InBounds(x, y) {
		return Blocked
	}

	if mod&IgnoreSurfaceClass != 0 {
		return Clear
	}

	result := Clear
	surface := g.ModifiedSurface(x, y, false)

	// A land unit over open water (not the surveyor) starts at amphibious cost; the land type test below still blocks it.
	if surface&world.SurfaceWater != 0 && d.Flags.Has(game.FlagMobileLandUnit) && d.Type != assets.Surveyor {
		result = Water
	}

	for _, other := range g.unitsAt(x, y) {
		// A path build arm time check treats mobile units as transient obstacles: with
		// IgnoreMobileUnits set, skip every mobile unit so a cell one is merely parked or
		// paused on (Order OrderAwait after a finished move, not OrderIdle) is not dropped
		// from the run. tickBuildPaths waits for the unit to clear at build time, then lays
		// the cell it stood on, so the path has no permanent gap where a unit once sat.
		if mod&IgnoreMobileUnits != 0 &&
			other.Flags&(game.FlagMobileLandUnit|game.FlagMobileSeaUnit|game.FlagMobileAirUnit) != 0 {
			continue
		}

		// Reject a terrain placement when the cell already contains the same ground
		// cover type. Player roads carry FlagGroundCover, while AI roads may carry
		// FlagStationary, so the check accepts either placement descriptor. Mobile
		// movers carry neither flag and continue to drive over roads.
		// C++ ported : access.cpp - Access_IsAccessible returns 0 for a same type ROAD over a ROAD.
		if d.Type == other.Type && other.Flags.Has(game.FlagGroundCover) &&
			(d.Flags.Has(game.FlagGroundCover) || d.Flags.Has(game.FlagStationary)) {
			return Blocked
		}

		// Visibility is always considered. An idle not stationary unit does not block.
		// A builder in its build pose (ORDER_BUILD) is NOT idle, so it blocks like any
		// working mobile unit and the site stays reserved while it is raised.
		// C++: Access_IsAccessible has no build pose exception (ORDER_BUILD returns 0).
		if !(other.Order != game.OrderIdle || other.Flags.Has(game.FlagStationary)) {
			continue
		}

		// A building still scaling up (ORDER_STATE_EXPAND, Unfold>0) is not yet a solid
		// obstacle: it stays passable so the builder that raised it can drive clear of
		// the footprint before the structure becomes collidable. This separates
		// visibility (HiddenUntilExpand) from collision (Unfold), so a 1x1 builder is
		// not trapped under its own building. C++: not a movement obstacle during expand.
		if other.Flags.Has(game.FlagBuilding) && other.Unfold > 0 {
			continue
		}

		sameClassBlocks := mod&SameClassBlocks != 0 || (mod&EnemySameClassBlocks != 0 && other.Team != team)

		if d.Flags.Has(game.FlagMobileAirUnit) {
			if sameClassBlocks && other.Flags.Has(game.FlagMobileAirUnit) {
				return Blocked
			}

			continue
		}

		// Ground/sea mover.
		if sameClassBlocks {
			if other.Flags&(game.FlagMobileSeaUnit|game.FlagMobileLandUnit) != 0 {
				return Blocked
			}

			// A landed (not hovering) air unit blocks ground/sea movers.
			if other.Flags.Has(game.FlagMobileAirUnit) && !other.Flags.Has(game.FlagHovering) {
				return Blocked
			}
		}

		// A stationary mover (building being placed) cannot sit on a bridge, rubble/cones or a mine (except the 4-way connector).
		if d.Flags.Has(game.FlagStationary) && d.Type != assets.Cnct4W &&
			(other.Type == assets.Bridge ||
				(other.Type >= assets.LrgRuble && other.Type <= assets.SmlCones) ||
				other.Type == assets.LandMine || other.Type == assets.SeaMine) {
			return Blocked
		}

		if other.Flags.Has(game.FlagStationary) {
			switch other.Type {
			case assets.Road:
				if d.Type == assets.Road {
					return Blocked
				}
				result = Reduced

			case assets.Bridge:
				result = Clear
				if d.Flags.Has(game.FlagMobileLandUnit) {
					result = Reduced
					surface = world.SurfaceLand // bridge treated as not elevated
				} else {
					surface = world.SurfaceWater
				}

			case assets.LandMine, assets.SeaMine:
				// Mines stay passable so hidden enemies can enter the cell and trigger
				// detonateMines. The placement check above still rejects stationary
				// units on a mine.

			case assets.LrgRuble, assets.SmlRuble:
				surface = world.SurfaceLand

			case assets.LrgSlab, assets.SmlSlab:
				if d.Type == assets.Road {
					return Blocked
				}
				result = Reduced

			case assets.Cnct4W:
				if d.Type == assets.Cnct4W {
					return Blocked
				}

				if d.Flags.Has(game.FlagStationary) && other.Team != team &&
					d.Type != assets.WtrPltfm && d.Type != assets.Bridge && d.Type != assets.Road {
					return Blocked
				}

			case assets.Waldo, assets.Harvster:
				// Solid 1x1 easter egg props: you cannot drive over them (confirmed against
				// retail vanilla). Harvster also hits the default block; listed here for clarity.
				return Blocked

			case assets.WtrPltfm:
				surface = world.SurfaceLand
				result = Clear

			default:
				return Blocked
			}
		}
	}

	if d.LandType&surface == 0 {
		return Blocked
	}

	return result
}

// IsAnyLandPresent reports whether the cell (or any of a building's 4 cells) is land.
//
// C++ ported : access.cpp - Access_IsAnyLandPresent
func (g *Grid) IsAnyLandPresent(x, y int, building bool) bool {
	if building && (g.surfaceAt(x+1, y) == world.SurfaceLand || g.surfaceAt(x, y+1) == world.SurfaceLand || g.surfaceAt(x+1, y+1) == world.SurfaceLand) {
		return true
	}

	return g.surfaceAt(x, y) == world.SurfaceLand
}

// IsFullyLandCovered reports whether the cell (or all of a building's 4 cells) is land.
//
// C++ ported : access.cpp - Access_IsFullyLandCovered
func (g *Grid) IsFullyLandCovered(x, y int, building bool) bool {
	if building && !(g.surfaceAt(x+1, y) == world.SurfaceLand && g.surfaceAt(x, y+1) == world.SurfaceLand && g.surfaceAt(x+1, y+1) == world.SurfaceLand) {
		return false
	}

	return g.surfaceAt(x, y) == world.SurfaceLand
}
