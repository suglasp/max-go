// factory_power_test.go : tests factory power starvation gating in production.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package sim

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
)

// TestUnpoweredFactoryProducesNothing verifies the production power gate: a factory in a
// power deficit builds no units even with ample material - production is gated on the
// factory being powered AND not starved of power, the way an unconnected mining station
// mines nothing. NOTE: queuing now powers the factory ON automatically (SetFactoryQueue), so an
// "unpowered" factory is one the economy marks STARVED OF POWER (no generator), not one left
// OrderPowerOff. Clearing the starvation lets it produce again (the gate is power, not a block).
func TestUnpoweredFactoryProducesNothing(t *testing.T) {
	g := newGame(t, 16, 16)
	g.Resources = economy.NewResourceMap(16, 16)
	g.MakeUnit = factoryUnitMaker(2) // two build turns per unit
	f := placeFactory(g, 4, 2)

	// Queuing powers it ON automatically, but with no generator the balance marks it STARVED OF POWER.
	g.SetFactoryQueue(f, []assets.UnitType{assets.Tank}, 1, false)
	f.PowerStarved = true // the balance result of no power source: must not produce
	for turn := 0; turn < 8 && len(g.builds) > 0; turn++ {
		g.AdvanceBuilds(0)
	}
	if got := countType(g, assets.Tank); got != 0 {
		t.Fatalf("a power-starved factory produced %d tanks, want 0", got)
	}

	// Power restored (a generator now supplies it): the held order progresses + deploys.
	f.PowerStarved = false
	for turn := 0; turn < 8 && len(g.builds) > 0; turn++ {
		g.AdvanceBuilds(0)
	}
	if got := countType(g, assets.Tank); got != 1 {
		t.Errorf("a powered factory produced %d tanks, want 1", got)
	}
}

// TestReconcilePowerIdlesDetachedBuilding verifies that scenario setup idles a building whose complex has no generator while leaving powered buildings on.
func TestReconcilePowerIdlesDetachedBuilding(t *testing.T) {
	g := newGame(t, 16, 16)

	// Detached: a lone Shipyard, its own complex, no generator.
	yard := stationaryBuilding(assets.Shipyard, 0, 2, 2)
	g.PlaceBuilding(yard, 2, 2)
	// Connected: a factory adjacent along an edge to a power station (one complex, powered).
	factory := stationaryBuilding(assets.LandPlt, 0, 6, 6)
	gen := stationaryBuilding(assets.PowerStn, 0, 8, 6)
	g.PlaceBuilding(factory, 6, 6)
	g.PlaceBuilding(gen, 8, 6)

	g.ReconcilePower(0)

	if yard.Order != game.OrderPowerOff {
		t.Errorf("detached shipyard order = %v, want OrderPowerOff (no power source)", yard.Order)
	}
	if factory.Order != game.OrderPowerOn {
		t.Errorf("connected factory order = %v, want OrderPowerOn (has a generator)", factory.Order)
	}
}
