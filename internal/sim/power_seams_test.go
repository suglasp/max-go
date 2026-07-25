// power_seams_test.go : tests the interactive power seams.
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

// poweredBase places a mining station, a power generator and one extra consumer in a
// single team 0 complex (the placement pattern that forms one complex), with fuel.
func poweredBase(t *testing.T, consumer assets.UnitType) (g *Game, mine, gen, extra *game.UnitInfo) {
	t.Helper()
	g = newGame(t, 16, 16)
	g.Resources = economy.NewResourceMap(16, 16)
	g.Resources.SetDeposit(2, 2, economy.CargoRaw, 16)
	mine = stationaryBuilding(assets.MiningSt, 0, 2, 2)
	mine.RawMining = 5
	mine.TotalMining = 5
	g.PlaceBuilding(mine, 2, 2)
	extra = stationaryBuilding(consumer, 0, 4, 2)
	g.PlaceBuilding(extra, 4, 2)
	gen = stationaryBuilding(assets.PowGen, 0, 2, 4)
	g.PlaceBuilding(gen, 2, 4)
	for _, cx := range g.Complexes {
		if cx.Team == 0 {
			cx.Fuel = 50
		}
	}
	return g, mine, gen, extra
}

// TestPowerOnConsumerBounce: a complex whose single generator is fully committed to
// the mine cannot power another consumer (PowerOnConsumer returns false so the caller
// bounces it); adding a second generator gives it the headroom and it powers on.
func TestPowerOnConsumerBounce(t *testing.T) {
	g, _, _, refinery := poweredBase(t, assets.CommTwr) // CommTwr draws power, needs no workers
	refinery.Order = game.OrderPowerOff
	refinery.State = game.OrderStateExecutingOrder

	if g.PowerOnConsumer(refinery) {
		t.Error("PowerOnConsumer should bounce (false): one generator is already committed to the mine")
	}
	if refinery.Order != game.OrderPowerOff {
		t.Errorf("a bounced consumer stays off, got %v", refinery.Order)
	}

	// Add a second generator: now there is spare capacity, so it powers on.
	gen2 := stationaryBuilding(assets.PowGen, 0, 3, 4)
	g.PlaceBuilding(gen2, 3, 4)
	if !g.PowerOnConsumer(refinery) {
		t.Error("with a spare generator PowerOnConsumer should succeed")
	}
	if refinery.Order != game.OrderPowerOn {
		t.Errorf("a powered-on consumer should be PowerOn, got %v", refinery.Order)
	}
}

// TestLastPowerLostHeldFactory: a producing factory the lone generator cannot also
// power is HELD on End Turn, and LastPowerLost names it so the screen can banner it.
func TestLastPowerLostHeldFactory(t *testing.T) {
	g, _, _, factory := poweredBase(t, assets.LandPlt)
	tank := game.NewUnitInfo(assets.Tank, 0, 0, 0)
	tank.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	tank.Values = &game.UnitValues{}
	tank.Values.Set(game.AttribTurns, 5)
	tank.Values.Set(game.AttribHits, 36)
	g.StartBuild(factory, tank, factory.GridX, factory.GridY, 1)
	factory.BuildRate = 1

	g.BeginTeamTurn(0)

	lost := g.LastPowerLost()
	found := false
	for _, u := range lost {
		if u == factory {
			found = true
		}
	}
	if !found {
		t.Errorf("LastPowerLost should name the held factory (got %d entries)", len(lost))
	}
	if !factory.PowerStarved {
		t.Error("a factory the generator cannot power should be HELD (PowerStarved)")
	}
}
