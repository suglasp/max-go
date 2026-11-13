// transfer_clamp_test.go : tests transfer amount clamping to source capacity and target free space.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
)

// TestTransferFuelAndGoldClamp verifies the X-Fer apply path (adjustStored) for the
// FUEL and GOLD cargo kinds for live testing: into a unit's own
// storage it clamps to [0, ATTRIB_STORAGE]; into a complex it routes to the
// matching pool (fuel -> Fuel, gold -> Gold), never the raw Material store.
func TestTransferFuelAndGoldClamp(t *testing.T) {
	s := &Screen{}

	// Own storage clamp: a fuel truck fills to capacity and floors at zero.
	fuelTruck := &game.UnitInfo{Type: assets.FuelTrck, Values: &game.UnitValues{}}
	fuelTruck.Values.Set(game.AttribStorage, 25)
	fuelTruck.Storage = 20
	s.adjustStored(fuelTruck, nil, economy.CargoFuel, 10) // 20+10 -> clamp at 25
	if fuelTruck.Storage != 25 {
		t.Errorf("fuel receiver = %d, want clamp at 25", fuelTruck.Storage)
	}
	s.adjustStored(fuelTruck, nil, economy.CargoFuel, -100) // over draw -> floor at 0
	if fuelTruck.Storage != 0 {
		t.Errorf("fuel giver = %d, want floor at 0", fuelTruck.Storage)
	}

	goldTruck := &game.UnitInfo{Type: assets.GoldTrck, Values: &game.UnitValues{}}
	goldTruck.Values.Set(game.AttribStorage, 40)
	goldTruck.Storage = 5
	s.adjustStored(goldTruck, nil, economy.CargoGold, 100) // 5+100 -> clamp at 40
	if goldTruck.Storage != 40 {
		t.Errorf("gold receiver = %d, want clamp at 40", goldTruck.Storage)
	}

	// Complex routing by type: fuel and gold land in their own pools, raw untouched.
	// The complex needs real storage buildings, since a base can hold no more than its
	// buildings hold and a delivery beyond that spills (economy.Complex.CapStores).
	c := economy.NewComplex(1)
	c.Grow(mkBuilding(assets.FDump, 0, 1, -1, -1, game.FlagStationary|game.FlagBuilding, game.OrderPowerOn, 50))
	c.Grow(mkBuilding(assets.GoldSm, 0, 1, -1, -1, game.FlagStationary|game.FlagBuilding, game.OrderPowerOn, 50))
	s.adjustStored(&game.UnitInfo{}, c, economy.CargoFuel, 7)
	if c.Fuel != 7 || c.Material != 0 || c.Gold != 0 {
		t.Errorf("fuel routed wrong: fuel=%d material=%d gold=%d", c.Fuel, c.Material, c.Gold)
	}
	s.adjustStored(&game.UnitInfo{}, c, economy.CargoGold, 3)
	if c.Gold != 3 || c.Fuel != 7 || c.Material != 0 {
		t.Errorf("gold routed wrong: fuel=%d material=%d gold=%d", c.Fuel, c.Material, c.Gold)
	}
}
