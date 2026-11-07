// idle_test.go : tests inactive and disabled producers.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package economy

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// TestIdleUnpoweredShipyard verifies the bug fix: a power consuming
// building (a shipyard) in a complex with NO generator - a structure sited
// off base with no power connection - is idled (OrderPowerOff) by BalanceTurn so
// it no longer "runs" ungated; the same building in a complex WITH a generator
// stays powered on, and a zero draw store (needs no power) is never idled.
func TestIdleUnpoweredShipyard(t *testing.T) {
	// Detached: a lone shipyard plus a store, no generator. The shipyard idles;
	// the store (draws no power) is left running.
	detached := NewComplex(1)
	yard := building(assets.Shipyard, 0, 5, 5, game.OrderPowerOn, 0)
	store := building(assets.ADump, 0, 7, 5, game.OrderPowerOn, 40)
	detached.Grow(yard)
	detached.Grow(store)
	if detached.hasGeneratorMember() {
		t.Fatal("a shipyard+store complex has no generator")
	}
	detached.BalanceTurn(4)
	if yard.Order != game.OrderPowerOff {
		t.Errorf("detached shipyard order = %v, want OrderPowerOff (idled, no power source)", yard.Order)
	}
	if store.Order != game.OrderPowerOn {
		t.Errorf("zero-draw store order = %v, want OrderPowerOn (needs no power, not idled)", store.Order)
	}

	// Connected: a shipyard with a power station in its complex stays running.
	powered := NewComplex(2)
	yard2 := building(assets.Shipyard, 0, 7, 7, game.OrderPowerOn, 0)
	gen := building(assets.PowerStn, 0, 9, 9, game.OrderPowerOn, 0)
	powered.Grow(yard2)
	powered.Grow(gen)
	powered.Fuel = 50 // fuel for the generator to burn
	if !powered.hasGeneratorMember() {
		t.Fatal("a complex with a power station has a generator")
	}
	powered.BalanceTurn(4)
	if yard2.Order != game.OrderPowerOn {
		t.Errorf("connected shipyard order = %v, want OrderPowerOn (powered, keeps running)", yard2.Order)
	}
}
