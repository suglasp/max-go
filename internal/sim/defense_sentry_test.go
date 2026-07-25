// defense_sentry_test.go : tests defense building OrderSentry defaults in sim.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// Note: Tests that a finished defense building defaults to OrderSentry and that
// other stationary buildings keep their existing defaults.
package sim

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// finishedBuilding runs a one turn build order for a stationary product (no
// builder; directly calls finishBuild via AdvanceBuilds) and returns the product.
// The product must have Values set with a sensible AttribTurns so the build
// completes in one AdvanceBuilds call.
func finishedBuilding(t *testing.T, g *Game, typ assets.UnitType, withAttack bool) *game.UnitInfo {
	t.Helper()
	p := game.NewUnitInfo(typ, 0, 0, 0)
	p.Flags = game.FlagBuilding | game.FlagStationary
	p.Team = 0
	p.Values = &game.UnitValues{}
	p.Values.Set(game.AttribHits, 10)
	p.Values.Set(game.AttribTurns, 1) // finishes in one turn
	if withAttack {
		p.Values.Set(game.AttribAttack, 5) // signals "defense building"
		p.Values.Set(game.AttribRange, 4)
	}
	// Provide enough storage so Advance can draw material (orders with a nil builder use
	// p.Storage as the pool; the base rate for a stationary product is Constrct * 2 per turn).
	p.Storage = 100
	// Start with a nil builder (in the style of a factory: the builder field is nil for
	// structures built without a constructor on the test bench).
	g.StartBuild(nil, p, 2, 2, 1)
	g.AdvanceBuilds(0)
	return p
}

// TestDefenseBuildingDefaultsSentry verifies that a newly finished defense
// building (stationary + attack value > 0) starts with OrderSentry, faithful
// to vanilla which defaults turrets to being on guard.
func TestDefenseBuildingDefaultsSentry(t *testing.T) {
	g := newGame(t, 16, 16)
	p := finishedBuilding(t, g, assets.GunTurrt, true /* withAttack */)
	if p.Order != game.OrderSentry {
		t.Errorf("finished GunTurrt order = %v, want OrderSentry", p.Order)
	}
}

// TestPowerStationDefaultsOff confirms that a stationary building that is not a
// defense building
// still defaults to OrderPowerOff (the Sentry default must not leak onto it).
func TestPowerStationDefaultsOff(t *testing.T) {
	g := newGame(t, 16, 16)
	p := finishedBuilding(t, g, assets.PowerStn, false /* no attack */)
	if p.Order != game.OrderPowerOff {
		t.Errorf("finished PowerStn order = %v, want OrderPowerOff", p.Order)
	}
}

// TestMiningStationDefaultsOn confirms that a Mining Station still defaults to
// OrderPowerOn (the existing mining station exception is unchanged).
func TestMiningStationDefaultsOn(t *testing.T) {
	g := newGame(t, 16, 16)
	p := game.NewUnitInfo(assets.MiningSt, 0, 0, 0)
	p.Flags = game.FlagBuilding | game.FlagStationary
	p.Team = 0
	p.Values = &game.UnitValues{}
	p.Values.Set(game.AttribHits, 10)
	p.Values.Set(game.AttribTurns, 1)
	p.Storage = 100
	g.StartBuild(nil, p, 4, 4, 1)
	g.AdvanceBuilds(0)
	if p.Order != game.OrderPowerOn {
		t.Errorf("finished MiningSt order = %v, want OrderPowerOn", p.Order)
	}
}

// TestAntiAirDefaultsSentry confirms the same Sentry default for a different
// defense building type (AntiAir), not just GunTurrt.
func TestAntiAirDefaultsSentry(t *testing.T) {
	g := newGame(t, 16, 16)
	p := finishedBuilding(t, g, assets.AntiAir, true /* withAttack */)
	if p.Order != game.OrderSentry {
		t.Errorf("finished AntiAir order = %v, want OrderSentry", p.Order)
	}
}
