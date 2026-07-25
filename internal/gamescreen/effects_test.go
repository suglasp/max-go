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
