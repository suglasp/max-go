// effects_test.go : tests effects.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// TestSpawnHitImpact: a hit that is not fatal records a HITEXPLD impact effect over the
// target footprint and leaves no rubble (the target survives).
func TestSpawnHitImpact(t *testing.T) {
	s := &Screen{}

	unit := game.NewUnitInfo(assets.Tank, 0, 1, 0)
	unit.SetPosition(5, 5)
	s.spawnHitImpact(unit)

	if len(s.effects.explosions) != 1 {
		t.Fatalf("explosions = %d, want 1", len(s.effects.explosions))
	}

	e := s.effects.explosions[0]
	if e.sprite != hitImpactType {
		t.Errorf("sprite = %v, want HitExpld", e.sprite)
	}

	if e.n != 1 {
		t.Errorf("footprint n = %d, want 1 for a mobile unit", e.n)
	}

	if e.dur != hitImpactMS {
		t.Errorf("dur = %d, want %d", e.dur, hitImpactMS)
	}

	// A struck building uses the 2x2 footprint.
	bld := game.NewUnitInfo(assets.PowerStn, 0, 2, 0)
	bld.Flags = game.FlagStationary | game.FlagBuilding
	bld.SetPosition(8, 8)
	s.spawnHitImpact(bld)

	if got := s.effects.explosions[1].n; got != 2 {
		t.Errorf("building impact footprint n = %d, want 2", got)
	}
}

// TestEffectsCentreOnConstructorSite: a Constructor raising a 2x2 is drawn at the site
// centre, so its hit flash and death blast centre on the site, not on its own corner cell.
func TestEffectsCentreOnConstructorSite(t *testing.T) {
	s := &Screen{}

	con := game.NewUnitInfo(assets.Constrct, 0, 3, 0)
	con.Flags = game.FlagMobileLandUnit | game.FlagConstructorUnit
	con.SetPosition(6, 6)       // the lower right corner of the site
	con.StartBuildPose(5, 5, 2) // a 2x2 site anchored at (5,5)

	s.spawnHitImpact(con)
	s.spawnExplosion(con)

	for i, e := range s.effects.explosions {
		if e.cellX != 5 || e.cellY != 5 || e.n != 2 {
			t.Errorf("effect %d at (%d,%d) n=%d, want the site (5,5) n=2", i, e.cellX, e.cellY, e.n)
		}
	}

	if got := s.effects.explosions[1].sprite; got != smallExplosionType {
		t.Errorf("death blast sprite = %v, want the small land blast", got)
	}

	// Out of the pose the effects use the unit's own cell again.
	con.EndBuildPose()
	s.effects.explosions = nil
	s.spawnHitImpact(con)
	s.spawnExplosion(con)

	for i, e := range s.effects.explosions {
		if e.cellX != 6 || e.cellY != 6 || e.n != 1 {
			t.Errorf("effect %d after the pose at (%d,%d) n=%d, want (6,6) n=1", i, e.cellX, e.cellY, e.n)
		}
	}
}
