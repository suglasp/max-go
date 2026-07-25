// generator_autopower_test.go : tests the turn start auto generator power report.
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

// TestRunEconomyReportsAutoPoweredGenerator: a generator that is OFF while a consumer
// needs its power gets turned on by the optimizer during RunEconomy, and that auto
// transition is reported through LastGeneratorsPoweredOn (so the screen can tell the
// player a generator came online on its own). A second turn, with it already running,
// reports nothing.
func TestRunEconomyReportsAutoPoweredGenerator(t *testing.T) {
	g := newGame(t, 16, 16)
	g.Resources = economy.NewResourceMap(16, 16)
	g.MiningDifficulty = 4

	mkBld := func(typ assets.UnitType, gx, gy int, order game.Order) *game.UnitInfo {
		u := game.NewUnitInfo(typ, 0, 0, 0)
		u.Flags = game.FlagBuilding | game.FlagStationary
		u.Values = &game.UnitValues{}
		u.GridX, u.GridY = gx, gy
		u.Order = order
		u.State = game.OrderStateExecutingOrder
		return u
	}

	c := economy.NewComplex(1)
	c.Team = 0
	gen := mkBld(assets.PowGen, 1, 1, game.OrderPowerOff) // idle generator the base actually needs
	consumer := mkBld(assets.CommTwr, 2, 1, game.OrderPowerOn)
	c.Grow(gen)
	c.Grow(consumer)
	c.Gold = 20 // keep the refinery consumer content (irrelevant to the power edge)
	g.Complexes = append(g.Complexes, c)

	g.RunEconomy(0)

	if gen.Order != game.OrderPowerOn {
		t.Fatalf("the optimizer should have powered the needed generator on, order=%v", gen.Order)
	}
	on := g.LastGeneratorsPoweredOn()
	if len(on) != 1 || on[0] != gen {
		t.Fatalf("LastGeneratorsPoweredOn = %v, want exactly the generator it brought online", on)
	}

	// Already running next turn: no new auto power on edge to report.
	g.RunEconomy(0)
	if on := g.LastGeneratorsPoweredOn(); len(on) != 0 {
		t.Errorf("a generator already online should not report again, got %v", on)
	}
}
