// startselect_test.go : tests initial builder selection when entering the map.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"math/rand"
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// TestMapEntryPreSelectsFirstBuilder: deployStartBase returns the first builder among the
// deployed mobile units (the unit map entry pre selects), even when a non builder leads the
// roster; a roster with no builder falls back to the first deployed unit.
func TestMapEntryPreSelectsFirstBuilder(t *testing.T) {
	s, _ := newClickScreen(t)
	rng := rand.New(rand.NewSource(1))

	// A non builder (Scout) leads the roster; the Engineer is the first builder.
	roster := []unitSpec{
		{assets.Scout, flagsLand},
		{assets.Tank, flagsLand},
		{assets.Engineer, flagsLand | game.FlagConstructorUnit},
		{assets.Constrct, flagsLand | game.FlagConstructorUnit},
	}
	sel := s.deployStartBase(0, game.Point{X: 8, Y: 8}, roster, nil, false, rng)
	if sel == nil {
		t.Fatal("deployStartBase returned no selection unit")
	}
	if sel.Type != assets.Engineer {
		t.Errorf("map entry pre-selected %v; want the first builder (Engineer)", sel.Type)
	}

	// A roster with no builder falls back to the first deployed unit.
	roster2 := []unitSpec{
		{assets.Scout, flagsLand},
		{assets.Tank, flagsLand},
	}
	sel2 := s.deployStartBase(1, game.Point{X: 3, Y: 3}, roster2, nil, false, rng)
	if sel2 == nil {
		t.Fatal("deployStartBase (no-builder roster) returned no selection unit")
	}
	if sel2.Type != assets.Scout {
		t.Errorf("no-builder roster pre-selected %v; want the first deployed unit (Scout)", sel2.Type)
	}
}
