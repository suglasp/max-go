// easteregg.go : places the hidden Waldo / Harvster gag units as rare solid map props.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/world"
)

// easterEggUnits are the two hidden gag units left in MAX.RES: Waldo (a Where's
// Waldo cameo) and Harvster (a Command and Conquer harvester look alike). The original
// defines them as unit types but never deploys them; vanilla scattered at most
// one as a rare land prop you cannot drive over (confirmed against the retail game).
var easterEggUnits = [...]assets.UnitType{assets.Waldo, assets.Harvster}

// easterEggOdds is the 1 in N chance that a fresh random map carries a gag unit:
// low enough to stay a surprise, common enough that a player will eventually find one.
const easterEggOdds = 3

// easterEggSeedSalt offsets the map seed for the easter egg roll so it is reproducible
// for a given game yet uncorrelated with the resource / derelict / clan draws.
const easterEggSeedSalt = 0x57A1D0 // "WALDO"

// spawnEasterEgg places AT MOST ONE gag unit (Waldo or Harvster) on the map at the
// low random easterEggOdds chance. It always lands on a clear LAND tile and behaves
// as a solid 1x1 prop: you cannot drive over it, build over it, select it, or attack
// it (the access / cursor / target paths all exclude it). avoid is every team's
// starting position; the prop stays well clear of them so it never lands on a base.
// The roll is independent of the alien derelict density, so it can appear even when
// derelicts are set to None.
func (s *Screen) spawnEasterEgg(avoid []game.Point, rng *game.Randomizer) {
	if s.wmap == nil {
		return
	}

	// Roll the rare spawn chance first: most maps carry none.
	if rng.Generate(easterEggOdds) != 0 {
		return
	}

	pick := easterEggUnits[rng.Generate(len(easterEggUnits))]
	w, h := s.wmap.W, s.wmap.H

	// Try a bounded number of random land tiles for a spot that is clear at the start.
	for tries := 0; tries < 200; tries++ {
		x := rng.Generate(w)
		y := rng.Generate(h)

		if !s.easterEggCellClear(x, y, avoid) {
			continue
		}

		// Owned by the neutral alien slot so it sits inert (no team unit count, no AI
		// interest); it draws as a plain stationary sprite. C++: WALDO / HARVSTER are
		// GROUND_COVER unit types in the engine, never spawned by the original.
		s.deployProp(game.AlienTeam, pick, game.FlagStationary, x, y)
		dbg.Printf("easter egg: %s placed at %d,%d", pick.Tag(), x, y)

		return
	}
}

// easterEggCellClear reports whether (x, y) is a valid easter egg spot: a fully LAND
// tile (never water or coast), inside the map with a one tile edge margin, empty of
// any other unit/prop, and at least 16 tiles from every team's starting position.
func (s *Screen) easterEggCellClear(x, y int, avoid []game.Point) bool {
	w, h := s.wmap.W, s.wmap.H

	// Keep a one tile margin from the map edge so the sprite sits fully on the map.
	if x < 1 || y < 1 || x >= w-1 || y >= h-1 {
		return false
	}

	// Land only: it is a land prop, never placed on water or a coast tile.
	if s.wmap.SurfaceAt(x, y) != world.SurfaceLand {
		return false
	}

	// The cell must be empty (no unit or prop already there).
	if len(s.game.Units.Cells.At(x, y)) > 0 {
		return false
	}

	// Stay well clear of every team's landing spot (16 tiles, 256 squared).
	for _, a := range avoid {
		dx := a.X - x
		dy := a.Y - y

		if dx*dx+dy*dy < 256 {
			return false
		}
	}

	return true
}
