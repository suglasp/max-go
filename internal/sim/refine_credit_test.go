// refine_credit_test.go : tests gold refinery credit generation per turn.
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

// TestRunEconomyRefineGoldCreditsTeam verifies that a powered Gold Refinery converts five units of complex gold reserve into five spendable team gold per turn.
func TestRunEconomyRefineGoldCreditsTeam(t *testing.T) {
	g := newGame(t, 16, 16)
	g.Resources = economy.NewResourceMap(16, 16)
	g.MiningDifficulty = 4

	mkBld := func(typ assets.UnitType, gx, gy int) *game.UnitInfo {
		u := game.NewUnitInfo(typ, 0, 0, 0)
		u.Flags = game.FlagBuilding | game.FlagStationary
		u.Values = &game.UnitValues{}
		u.Values.Set(game.AttribStorage, retailStorage[typ])
		u.GridX, u.GridY = gx, gy
		u.Order = game.OrderPowerOn
		u.State = game.OrderStateExecutingOrder
		return u
	}

	c := economy.NewComplex(1)
	c.Team = 0
	c.Grow(mkBld(assets.PowGen, 1, 1)) // powers the refinery (1 power out >= 1 draw)
	c.Grow(mkBld(assets.CommTwr, 2, 1))
	// A Gold Vault, or the base has nowhere to keep gold: neither a refinery nor a power
	// generator stores any, and an unstorable reserve spills (CapStores).
	c.Grow(mkBld(assets.GoldSm, 3, 1))
	c.Gold = 20
	g.Complexes = append(g.Complexes, c)

	g.Gold[0] = 0
	g.RunEconomy(0)

	if g.Gold[0] != 5 {
		t.Errorf("team credit after one refine = %d, want 5", g.Gold[0])
	}
	if c.Gold != 15 {
		t.Errorf("reserve after one refine = %d, want 15 (BalanceTurn drew the 5 upkeep once)", c.Gold)
	}

	// A second turn refines another 5 / draws another 5.
	g.RunEconomy(0)
	if g.Gold[0] != 10 || c.Gold != 10 {
		t.Errorf("after two turns credit=%d reserve=%d, want 10/10", g.Gold[0], c.Gold)
	}
}

// TestRunEconomyRefineGoldStopsWhenEmpty: once the reserve is exhausted the refinery
// credits nothing (no gold conjured from an empty pool).
func TestRunEconomyRefineGoldStopsWhenEmpty(t *testing.T) {
	g := newGame(t, 16, 16)
	g.Resources = economy.NewResourceMap(16, 16)
	g.MiningDifficulty = 4

	mkBld := func(typ assets.UnitType, gx, gy int) *game.UnitInfo {
		u := game.NewUnitInfo(typ, 0, 0, 0)
		u.Flags = game.FlagBuilding | game.FlagStationary
		u.Values = &game.UnitValues{}
		u.GridX, u.GridY = gx, gy
		u.Order = game.OrderPowerOn
		u.State = game.OrderStateExecutingOrder
		return u
	}

	c := economy.NewComplex(1)
	c.Team = 0
	c.Grow(mkBld(assets.PowGen, 1, 1))
	c.Grow(mkBld(assets.CommTwr, 2, 1))
	c.Gold = 0
	g.Complexes = append(g.Complexes, c)

	g.Gold[0] = 0
	g.RunEconomy(0)
	if g.Gold[0] != 0 || c.Gold != 0 {
		t.Errorf("empty-reserve refine credit=%d reserve=%d, want 0/0", g.Gold[0], c.Gold)
	}
}
