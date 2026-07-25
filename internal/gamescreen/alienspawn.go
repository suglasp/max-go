// alienspawn.go : implements the alien team spawner.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/paths"
	"github.com/suglasp/max-go/internal/world"
)

// The neutral alien derelict roster: one random artifact building per site plus
// mobile derelicts with a budgeted value. C++: GameManager_AlienBuildings /
// GameManager_AlienUnits (game_manager.cpp). LRGSLAB is not deployed separately -
// PlaceBuilding lays the permanent slab under the artifact for us.
var alienBuildings = [...]assets.UnitType{assets.ShieldGn, assets.SuprtPlt, assets.RecCentr}
var alienUnits = [...]assets.UnitType{assets.AlnTank, assets.AlnAsGun, assets.AlnPlane}

// SetAlienDerelicts records the custom game alien derelict density level (0 None,
// 1 Rare, 2 Common) chosen on the setup screen, consumed by the new game spawn.
func (s *Screen) SetAlienDerelicts(level int) {
	s.alienDerelicts = level
}

// alienDensity maps the alien derelict density level to the original's separation grid
// spacing and per site unit value budget. C++: menu.cpp - the resource_derelicts
// switch (0 -> 0/0, 1 -> 65/20, 2 -> 50/30).
func alienDensity(level int) (separation, value int) {
	switch level {
	case 1:
		return 65, 20
	case 2:
		return 50, 30
	default:
		return 0, 0
	}
}

// alienTurns is a unit type's build turns cost, used as its "value" against the
// per site budget (the original reads ATTRIB_TURNS). Falls back to 1 when ATTRIBS is not
// loaded (headless), so the budget loop always terminates.
func (s *Screen) alienTurns(t assets.UnitType) int {
	if int(t) >= 0 && int(t) < len(s.buildUI.attribs) {
		if n := s.buildUI.attribs[t].TurnsToBuild; n > 0 {
			return n
		}
	}

	return 1
}

// alienUnitFlags is the movement/role flags for a deployed alien derelict: the
// plane flies, the juggernaut sits on water (sea flag, so its water sprite shows),
// the rest are land units. All are selectable so a player can target and salvage them.
func alienUnitFlags(t assets.UnitType) game.Flags {
	switch t {
	case assets.AlnPlane:
		return game.FlagMobileAirUnit | game.FlagHovering | game.FlagSelectable
	case assets.Juggrnt:
		return game.FlagMobileSeaUnit | game.FlagSelectable
	default:
		return game.FlagMobileLandUnit | game.FlagSelectable
	}
}

// populateAlienDerelicts scatters the neutral alien derelict faction (game.AlienTeam)
// across the map at the start of a new custom game: a separation grid of derelict
// sites, each an artifact building plus alien units with a budgeted value, all owned by the
// fixed unclaimable alien slot so they sit inert until a player attacks or salvages
// them. avoid lists the teams' starting positions (sites stay >= 16 tiles clear of
// them). A zero separation or value spawns nothing (density None / missions).
//
// C++ ported : game_manager.cpp - GameManager_PopulateMapWithAlienUnits.
func (s *Screen) populateAlienDerelicts(separation, value int, avoid []game.Point, rng *game.Randomizer) {
	if separation <= 0 || value <= 0 {
		return
	}

	w, h := s.wmap.W, s.wmap.H
	before := len(s.game.Units.Units())

	stepX := (separation * 4) / 5

	if stepX <= 0 {
		stepX = 1
	}

	originX := rng.Generate(stepX + 1)
	originY := rng.Generate(separation/2 + 1)

	stagger := false

	for i := originX; i < w; i += stepX {
		startY := originY

		if stagger {
			startY = separation/2 + originY
		}

		for j := startY; j < h; j += separation {
			// Jitter each site by +-separation/5 (two independent draws), matching
			// the original's point2 computation exactly (integer division truncates to zero,
			// so (-separation)/5 == -(separation/5)).
			lo := (-separation) / 5
			span := (separation / 5) - lo + 1
			px := rng.Generate(span) + lo + i
			py := rng.Generate(span) + lo + j
			s.spawnAlienDerelicts(game.Point{X: px, Y: py}, value, avoid, rng)
		}

		stagger = !stagger
	}

	dbg.Printf("alien derelicts: sep=%d value=%d deployed=%d", separation, value, len(s.game.Units.Units())-before)
}

// spawnAlienDerelicts populates one derelict site near point: it snaps to a valid
// spot, places a random artifact building, frames it with BRIDGE tiles on the
// surrounding water, then deploys alien units onto the surrounding ring tiles until
// the per site value budget runs out (juggernauts on water, tanks/assault guns/planes
// on land). C++ ported : game_manager.cpp - GameManager_SpawnAlienDerelicts.
func (s *Screen) spawnAlienDerelicts(point game.Point, value int, avoid []game.Point, rng *game.Randomizer) {
	w, h := s.wmap.W, s.wmap.H
	s.findAlienSpot(&point, avoid)

	building := game.FlagStationary | game.FlagBuilding | game.FlagSelectable
	artifact := alienBuildings[rng.Generate(len(alienBuildings))]
	s.placeStartBuilding(game.AlienTeam, artifact, building, point.X, point.Y)

	s.bridgeArtifactRing(point)

	cost := s.alienTurns(assets.AlnTank) // the cheapest land derelict gates the budget
	juggCost := s.alienTurns(assets.Juggrnt)

	for offset := 2; value >= cost && offset > 0; offset-- {
		var land, water []game.Point
		pos := game.Point{X: point.X - offset, Y: point.Y + offset + 1}

		for dir := 0; dir < 8; dir += 2 {
			d := paths.StepOffset(dir)

			for step := 0; step < 2*offset+1; step++ {
				pos.X += d.X
				pos.Y += d.Y

				if pos.X < 0 || pos.Y < 0 || pos.X >= w || pos.Y >= h {
					continue
				}

				switch s.wmap.SurfaceAt(pos.X, pos.Y) {
				case world.SurfaceWater:
					water = append(water, pos)
				case world.SurfaceLand:
					land = append(land, pos)
				}
			}
		}

		// the original's water clear guard: drop water tiles only once the budget is spent
		// below 1 while a juggernaut is already unaffordable (an edge case that does almost nothing).
		if value < clearWaterThreshold(value, juggCost) {
			water = nil
		}

		for value >= cost && len(land)+len(water) > 0 {
			// Vanilla deploys every derelict facing up; we scatter them in random 8-way
			// facings (by design) so a derelict field looks abandoned, not like a parade ground.
			facing := uint8(rng.Generate(game.AngleCount))

			if rng.Generate(len(land)+len(water))+1 <= len(water) {
				idx := rng.Generate(len(water))
				wp := water[idx]
				water = append(water[:idx], water[idx+1:]...)
				s.deployUnit(assets.Juggrnt, alienUnitFlags(assets.Juggrnt), game.AlienTeam, facing, wp.X, wp.Y)
				value -= juggCost
			} else {
				idx := rng.Generate(len(land))
				lp := land[idx]
				land = append(land[:idx], land[idx+1:]...)
				t := alienUnits[0]

				for tries := 0; tries < 100; tries++ {
					t = alienUnits[rng.Generate(len(alienUnits))]

					if s.alienTurns(t) <= value {
						break
					}
				}

				s.deployUnit(t, alienUnitFlags(t), game.AlienTeam, facing, lp.X, lp.Y)
				value -= s.alienTurns(t)
			}

			if value < clearWaterThreshold(value, juggCost) {
				water = nil
			}
		}
	}
}

// bridgeArtifactRing frames the artifact with BRIDGE props: it walks the 12-tile perimeter
// ring around the 2x2 footprint (3 steps in each cardinal from one tile below and left) and
// lays a bridge owned by the alien team on every water/coast tile - a full ring in open water,
// just the tiles along the shore on land. Cosmetic (FlagStationary, no complex). C++: the
// SURFACE_TYPE_WATER/COAST bridge loop in GameManager_SpawnAlienDerelicts.
func (s *Screen) bridgeArtifactRing(point game.Point) {
	w, h := s.wmap.W, s.wmap.H
	bridgePos := game.Point{X: point.X - 1, Y: point.Y + 2}

	for dir := 0; dir < 8; dir += 2 {
		d := paths.StepOffset(dir)

		for step := 0; step < 3; step++ {
			bridgePos.X += d.X
			bridgePos.Y += d.Y

			if bridgePos.X < 0 || bridgePos.Y < 0 || bridgePos.X >= w || bridgePos.Y >= h {
				continue
			}

			switch s.wmap.SurfaceAt(bridgePos.X, bridgePos.Y) {
			case world.SurfaceWater, world.SurfaceCoast:
				s.deployProp(game.AlienTeam, assets.Bridge, game.FlagStationary, bridgePos.X, bridgePos.Y)
			}
		}
	}
}

// clearWaterThreshold reproduces the original's local "value" guard in
// GameManager_SpawnAlienDerelicts: 1 when a juggernaut is unaffordable, else 0.
// Water tiles are cleared when the remaining budget falls below this.
func clearWaterThreshold(value, juggCost int) int {
	if value < juggCost {
		return 1
	}

	return 0
}

// findAlienSpot snaps point to the nearest valid 2x2 derelict site by spiralling
// outward: in bounds (with a 3-tile margin), clear of every team's starting
// position by >= 16 tiles (256 squared), and not over an air/space tile. C++
// ported : game_manager.cpp - GameManager_FindSpot. A whole map bound guards the
// (otherwise unbounded) loop in the original against a hang on a degenerate map.
func (s *Screen) findAlienSpot(point *game.Point, avoid []game.Point) {
	w, h := s.wmap.W, s.wmap.H

	for span := 2; span <= w+h; span += 2 {
		point.X--
		point.Y++

		for dir := 0; dir < 8; dir += 2 {
			d := paths.StepOffset(dir)

			for i := 0; i < span; i++ {
				point.X += d.X
				point.Y += d.Y

				if point.X < 1 || point.Y < 1 || point.X+3 >= w || point.Y+3 >= h {
					continue
				}

				if !s.alienSpotClear(*point, avoid) {
					continue
				}

				return
			}
		}
	}
}

// alienSpotClear reports whether a 2x2 derelict footprint at p is far enough from
// every team start and clear of air tiles.
func (s *Screen) alienSpotClear(p game.Point, avoid []game.Point) bool {
	for _, a := range avoid {
		dx := a.X - p.X
		dy := a.Y - p.Y

		if dx*dx+dy*dy < 256 {
			return false
		}
	}

	for gx := p.X; gx < p.X+2; gx++ {
		for gy := p.Y; gy < p.Y+2; gy++ {
			if s.wmap.SurfaceAt(gx, gy) == world.SurfaceAir {
				return false
			}
		}
	}

	return true
}
