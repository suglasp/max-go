// stranded.go : a destroyed ground host takes its ground covers with it and strands the units left on invalid terrain.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : units_manager.cpp - UnitsManager_BuildClearing, access.cpp - Access_DestroyGroundCovers
package sim

import (
	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/world"
)

// isTerrainCover reports whether t is one of the four cover types a dead host drags along.
func isTerrainCover(t assets.UnitType) bool {
	return t == assets.Road || t == assets.LandPad || t == assets.WtrPltfm || t == assets.Bridge
}

// registered reports whether u is still in the unit registry (not yet destroyed).
func (g *Game) registered(u *game.UnitInfo) bool {
	return g.Units.ByID.Get(u.ID) == u
}

// hostCells lists the cells a destroyed host stood on: 2x2 for a building, else one.
func hostCells(host *game.UnitInfo) []game.Point {
	size := 1
	if host.Flags.Has(game.FlagBuilding) {
		size = 2
	}

	cells := make([]game.Point, 0, size*size)
	for dy := 0; dy < size; dy++ {
		for dx := 0; dx < size; dx++ {
			cells = append(cells, game.Point{X: host.GridX + dx, Y: host.GridY + dy})
		}
	}

	return cells
}

// strandedLandType is the surface mask a unit may rest on. A mine is not a mover, so
// the movement flags say nothing: a land mine rests on land, a sea mine on water.
// A unit that may stand on land may also stand on the coast band.
func strandedLandType(u *game.UnitInfo) uint8 {
	switch u.Type {
	case assets.LandMine:
		return world.SurfaceLand | world.SurfaceCoast
	case assets.SeaMine:
		return world.SurfaceWater
	}

	mask := access.LandTypeForUnit(u.Type, u.Flags)
	if mask&world.SurfaceLand != 0 {
		mask |= world.SurfaceCoast
	}

	return mask
}

// destroyGroundCovers removes every road, landing pad, water platform and bridge on the
// host's cells, silently: no casualty, no rubble. The host itself is skipped.
// C++ ported : access.cpp - Access_DestroyGroundCovers
func (g *Game) destroyGroundCovers(host *game.UnitInfo) {
	covers := map[*game.UnitInfo]bool{}
	for _, c := range hostCells(host) {
		for _, e := range g.Units.Cells.At(c.X, c.Y) {
			if e != host && isTerrainCover(e.Type) {
				covers[e] = true
			}
		}
	}

	if len(covers) == 0 {
		return
	}

	for cover := range covers {
		g.Units.Remove(cover)
	}

	g.invalidateReach() // a bridge or platform going changes terrain connectivity
}

// findStrandedOnCell appends the units left stranded on one cell once host is gone.
// C++ ported : access.cpp - Access_FindUnitsOnInvalidTerrain
func (g *Game) findStrandedOnCell(host *game.UnitInfo, cellX, cellY int, out []*game.UnitInfo) []*game.UnitInfo {
	here := g.Units.Cells.At(cellX, cellY)

	hasPad := false
	for _, e := range here {
		if e != host && e.Hits > 0 && e.Type == assets.LandPad {
			hasPad = true
			break
		}
	}

	for _, e := range here {
		if e == host || e.Hits <= 0 || isTerrainCover(e.Type) {
			continue
		}

		air := e.Flags.Has(game.FlagMobileAirUnit)
		if air && e.Flags.Has(game.FlagHovering) {
			continue // a flying aircraft may be over any terrain
		}

		if air {
			if !hasPad {
				out = append(out, e)
			}
			continue
		}

		seaOnly := e.Flags.Has(game.FlagMobileSeaUnit) && !e.Flags.Has(game.FlagMobileLandUnit)
		if strandedLandType(e)&g.Grid.ModifiedSurface(cellX, cellY, seaOnly) == 0 {
			out = append(out, e)
		}
	}

	return out
}

// clearStranded handles a host that just left the registry: its covers go, then every
// unit the loss leaves on invalid terrain. Mines sink without a blast, any other unit is
// destroyed (cargo and its own covers and strandings included). Nothing here queues a
// blast or explosion event, so the deaths are silent.
// C++ ported : units_manager.cpp - UnitsManager_BuildClearing (the stranded units pass)
func (g *Game) clearStranded(host *game.UnitInfo) {
	if host.Flags&(game.FlagMobileLandUnit|game.FlagMobileAirUnit|game.FlagStationary) == 0 ||
		host.Flags.Has(game.FlagHovering) {
		return
	}

	g.destroyGroundCovers(host)

	var stranded []*game.UnitInfo
	for _, c := range hostCells(host) {
		stranded = g.findStrandedOnCell(host, c.X, c.Y, stranded)
	}

	seen := map[*game.UnitInfo]bool{}
	for _, e := range stranded {
		if seen[e] || !g.registered(e) || e.Hits <= 0 {
			continue // a unit listed twice, or already taken down by an earlier one
		}
		seen[e] = true

		if IsMine(e) {
			g.Units.Remove(e) // a sunken mine does not detonate
			g.recordCasualty(e)

			continue
		}

		g.DestroyUnit(e)
	}
}
