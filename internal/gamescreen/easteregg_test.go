// easteregg_test.go : tests easteregg.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/world"
)

// isEasterEgg reports whether t is one of the gag unit types.
func isEasterEgg(t assets.UnitType) bool {
	return t == assets.Waldo || t == assets.Harvster
}

// TestSpawnEasterEggInvariants runs the spawn over many seeds: each run places AT
// MOST ONE gag unit, and whenever one is placed it is a Waldo/Harvster owned by the
// neutral alien slot, sitting on a clear LAND tile well away from the avoid points.
// Across the seed range some runs must spawn (it is reachable) and some must not (it
// stays a rare roll).
func TestSpawnEasterEggInvariants(t *testing.T) {
	avoid := []game.Point{{X: 5, Y: 5}, {X: 58, Y: 58}}
	spawned, skipped := 0, 0

	for seed := uint32(0); seed < 60; seed++ {
		s := newAlienScreen(t, 64)
		before := len(s.game.Units.Units())
		s.spawnEasterEgg(avoid, game.NewRandomizer(seed))
		added := s.game.Units.Units()[before:]

		if len(added) > 1 {
			t.Fatalf("seed %d placed %d gag units, want at most 1", seed, len(added))
		}

		if len(added) == 0 {
			skipped++
			continue
		}

		spawned++
		u := added[0]

		if !isEasterEgg(u.Type) {
			t.Errorf("seed %d placed %s, want Waldo or Harvster", seed, u.Type.Tag())
		}

		if u.Team != game.AlienTeam {
			t.Errorf("seed %d gag unit team = %d, want the neutral alien slot", seed, u.Team)
		}

		if s.wmap.SurfaceAt(u.GridX, u.GridY) != world.SurfaceLand {
			t.Errorf("seed %d placed gag unit on non-land surface at %d,%d", seed, u.GridX, u.GridY)
		}

		for _, a := range avoid {
			dx, dy := a.X-u.GridX, a.Y-u.GridY
			if dx*dx+dy*dy < 256 {
				t.Errorf("seed %d gag unit at %d,%d too close to start %v", seed, u.GridX, u.GridY, a)
			}
		}
	}

	if spawned == 0 {
		t.Error("no seed in [0,60) spawned a gag unit; the roll never fires")
	}

	if skipped == 0 {
		t.Error("every seed spawned a gag unit; the rare roll is not actually rare")
	}
}

// TestEasterEggNotSelectable checks the gag props are inert: selectableOccupant
// rejects a Waldo and a Harvster (you can only walk around them, not click them).
func TestEasterEggNotSelectable(t *testing.T) {
	s := newAlienScreen(t, 16)

	for _, et := range easterEggUnits {
		u := &game.UnitInfo{Type: et, Team: game.AlienTeam, Flags: game.FlagStationary}
		if s.selectableOccupant(u) {
			t.Errorf("%s is selectable, want inert (not selectable)", et.Tag())
		}
	}
}
