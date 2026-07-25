// complexpower_save_test.go : complex power save/load round trip tests for the saveload package.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package saveload

import (
	"bytes"
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
)

// TestSaveRestorePreservesComplexPower checks that v11 complex stores retain enough fuel to power an Eco Sphere after Restore rebuilds its network.
func TestSaveRestorePreservesComplexPower(t *testing.T) {
	m := landMap(16, 16)
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.New(p, m)

	// A power generator + an adjacent Eco Sphere form one complex.
	gen := game.NewUnitInfo(assets.PowGen, 0, 1, 0)
	gen.Flags = game.FlagStationary
	gen.Order = game.OrderPowerOn
	gen.State = game.OrderStateExecutingOrder
	gen.Values = &game.UnitValues{}
	gen.Values.Set(game.AttribHits, 40)
	g.PlaceBuilding(gen, 5, 5)

	eco := game.NewUnitInfo(assets.Greenhse, 0, 2, 0)
	eco.Flags = game.FlagBuilding | game.FlagStationary | game.FlagSelectable
	eco.Order = game.OrderPowerOn
	eco.State = game.OrderStateExecutingOrder
	eco.Values = &game.UnitValues{}
	eco.Values.Set(game.AttribHits, 20)
	g.PlaceBuilding(eco, 6, 5) // directly right of the generator -> same complex

	c := g.ComplexOf(eco)
	if c == nil || c != g.ComplexOf(gen) {
		t.Fatal("setup: the eco-sphere and generator are not in one complex")
	}
	c.Fuel = 50 // the fuel pool a running base accumulates
	if !c.CanPower(eco) {
		t.Fatal("precondition: the eco-sphere should be powerable in memory (generator + fuel)")
	}

	var buf bytes.Buffer
	if err := Save(&buf, Capture(g, "GREEN_1.WRL")); err != nil {
		t.Fatalf("save: %v", err)
	}
	snap, err := Load(&buf)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	g2 := snap.Restore(landMap(16, 16))

	var eco2 *game.UnitInfo
	for _, u := range g2.Units.Units() {
		if u.ID == 2 {
			eco2 = u
		}
	}
	if eco2 == nil {
		t.Fatal("after load: eco-sphere not found")
	}
	c2 := g2.ComplexOf(eco2)
	if c2 == nil {
		t.Fatal("after load: eco-sphere not in a complex")
	}
	if c2.Fuel <= 0 {
		t.Errorf("after load: complex fuel pool = %d, want 50 (the rebuilt complex must keep its stores)", c2.Fuel)
	}
	if !c2.CanPower(eco2) {
		t.Error("after load: the eco-sphere is no longer powerable (the 'not enough power' bug)")
	}
}
