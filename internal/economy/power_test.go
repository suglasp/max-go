// power_test.go : tests power production, demand, and shortages.
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

// operational buildings carry State = ExecutingOrder; with the default Init
// state NetProduction reads PriorOrder and treats them as not yet powered.
func miningStation(raw int, order game.Order) *game.UnitInfo {
	return &game.UnitInfo{Type: assets.MiningSt, Order: order, State: game.OrderStateExecutingOrder, RawMining: raw, TotalMining: raw}
}
func powerGen(order game.Order) *game.UnitInfo {
	return &game.UnitInfo{Type: assets.PowGen, Order: order, State: game.OrderStateExecutingOrder}
}

// TestComplexPowerBalance checks that mining needs power: it produces when a
// generator supplies it, browns out without one, and an idle generator is
// powered automatically to keep mining running (the forced minimum generator rule).
func TestComplexPowerBalance(t *testing.T) {
	// Mining + an active generator (with fuel to burn): mining produces.
	c := &Complex{Members: []*game.UnitInfo{miningStation(5, game.OrderPowerOn), powerGen(game.OrderPowerOn)}, Fuel: 10}
	if net := c.BalanceTurn(4); net.Raw != 5 {
		t.Errorf("powered mining net raw = %d, want 5", net.Raw)
	}

	// Mining with no generator: browned out, produces nothing.
	c = &Complex{Members: []*game.UnitInfo{miningStation(5, game.OrderPowerOn)}, Fuel: 10}
	if net := c.BalanceTurn(4); net.Raw != 0 {
		t.Errorf("unpowered mining net raw = %d, want 0 (browned out)", net.Raw)
	}

	// Mining + a generator with NO fuel in store: the generator STILL supplies power
	// (a generator does not gate on its fuel stock - a base short on fuel shows a
	// shortage but keeps the lights on), so mining continues and can restore fuel.
	c = &Complex{Members: []*game.UnitInfo{miningStation(5, game.OrderPowerOn), powerGen(game.OrderPowerOn)}, Fuel: 0}
	if net := c.BalanceTurn(4); net.Raw != 5 {
		t.Errorf("fuel-starved base net raw = %d, want 5 (generator still powers mining)", net.Raw)
	}

	// Mining + a powered OFF generator (with fuel): powered on automatically so mining runs.
	gen := powerGen(game.OrderPowerOff)
	c = &Complex{Members: []*game.UnitInfo{miningStation(5, game.OrderPowerOn), gen}, Fuel: 10}
	if net := c.BalanceTurn(4); net.Raw != 5 {
		t.Errorf("auto-powered mining net raw = %d, want 5", net.Raw)
	}
	if gen.Order != game.OrderPowerOn {
		t.Errorf("idle generator was not auto-powered (order = %v)", gen.Order)
	}

	// Two mining stations but only one generator: one browns out (only one unit
	// of power for two units of demand, no spare generator to activate).
	c = &Complex{Members: []*game.UnitInfo{
		miningStation(5, game.OrderPowerOn), miningStation(5, game.OrderPowerOn), powerGen(game.OrderPowerOn),
	}, Fuel: 10}
	if net := c.BalanceTurn(4); net.Raw != 5 {
		t.Errorf("power-short pair net raw = %d, want 5 (one mining browned out)", net.Raw)
	}
}

func greenhouse(order game.Order) *game.UnitInfo {
	return &game.UnitInfo{Type: assets.Greenhse, Order: order, State: game.OrderStateExecutingOrder}
}

func habitat(order game.Order) *game.UnitInfo {
	return &game.UnitInfo{Type: assets.Habitat, Order: order, State: game.OrderStateExecutingOrder}
}

// TestStartShortage checks the power on gate names the DEFICIENT resource: an
// Eco Sphere with no generator is power short; one with power but no spare worker
// is worker short; one with power AND an active habitat can start; a generator
// always can. The worker check is LIVE (workerSurplus over the active members), so
// no habitat in the complex = no free worker.
func TestStartShortage(t *testing.T) {
	gh := greenhouse(game.OrderPowerOff)

	// No generator -> power is the (first checked) shortage.
	c := &Complex{Members: []*game.UnitInfo{gh}}
	if got := c.StartShortage(gh); got != ShortagePower {
		t.Errorf("no-generator eco-sphere shortage = %v, want ShortagePower", got)
	}

	// Powered (generator + fuel) but NO habitat -> no free worker -> worker shortage.
	c = &Complex{Members: []*game.UnitInfo{gh, powerGen(game.OrderPowerOn)}, Fuel: 10}
	if got := c.StartShortage(gh); got != ShortageWorkers {
		t.Errorf("worker-short eco-sphere shortage = %v, want ShortageWorkers", got)
	}

	// Powered + an ACTIVE habitat (houses 3 workers) -> a free worker -> it can start.
	c = &Complex{Members: []*game.UnitInfo{gh, powerGen(game.OrderPowerOn), habitat(game.OrderPowerOn)}, Fuel: 10}
	if got := c.StartShortage(gh); got != ShortageNone {
		t.Errorf("powered+staffed eco-sphere shortage = %v, want ShortageNone", got)
	}

	// A DESTROYED/removed habitat (no longer a member) immediately removes the
	// surplus, so the same Eco Sphere is worker short again (the blow up case).
	c = &Complex{Members: []*game.UnitInfo{gh, powerGen(game.OrderPowerOn)}, Fuel: 10}
	if got := c.StartShortage(gh); got != ShortageWorkers {
		t.Errorf("after losing the habitat the eco-sphere should be worker-short, got %v", got)
	}

	// A generator can always start (it makes power, draws no workers).
	g := powerGen(game.OrderPowerOff)
	c = &Complex{Members: []*game.UnitInfo{g}}
	if got := c.StartShortage(g); got != ShortageNone {
		t.Errorf("generator shortage = %v, want ShortageNone", got)
	}
}

// TestFactoryAutoPowersReactor checks that a producing factory starts the only
// available reactor. An idle factory has no demand and lets the optimizer switch
// the reactor off again.
func TestFactoryAutoPowersReactor(t *testing.T) {
	factory := &game.UnitInfo{
		Type: assets.LightPlt, Order: game.OrderPowerOn, State: game.OrderStateExecutingOrder,
		BuildQueue: []assets.UnitType{assets.Tank}, BuildRate: 1, // producing -> draws power
	}
	reactor := &game.UnitInfo{Type: assets.PowerStn, Order: game.OrderPowerOff, State: game.OrderStateExecutingOrder}
	c := &Complex{Members: []*game.UnitInfo{factory, reactor}, Fuel: 50}

	c.BalanceTurn(4)

	if reactor.Order != game.OrderPowerOn {
		t.Errorf("a producing factory's demand should auto-power the large reactor, reactor order = %v", reactor.Order)
	}
	if factory.PowerStarved {
		t.Error("the factory should not be power-starved once the reactor is online")
	}

	// An empty queue removes the factory's demand, so the optimizer powers the
	// reactor off to conserve fuel.
	factory.BuildQueue = nil
	factory.BuildRate = 0
	c.BalanceTurn(4)
	if reactor.Order != game.OrderPowerOff {
		t.Errorf("an idle factory leaves no demand, so the reactor should power off, got %v", reactor.Order)
	}
}

// TestFactoryBuildDraw checks a factory draws raw + power while building and
// nothing while idle.
func TestFactoryBuildDraw(t *testing.T) {
	idle := &game.UnitInfo{Type: assets.LightPlt, Order: game.OrderPowerOn}
	if got := NetProduction(idle, 4); got.Raw != 0 || got.Power != 0 {
		t.Errorf("idle factory draws %+v, want zero", got)
	}

	building := &game.UnitInfo{Type: assets.LightPlt, Order: game.OrderBuild, State: game.OrderStateExecutingOrder, BuildRate: 2}
	got := NetProduction(building, 4)
	if got.Power >= 0 {
		t.Errorf("building factory power = %d, want negative (draws power)", got.Power)
	}
	if got.Raw >= 0 {
		t.Errorf("building factory raw = %d, want negative (draws material)", got.Raw)
	}

	// A finished factory (UNIT_READY) no longer draws.
	done := &game.UnitInfo{Type: assets.LightPlt, Order: game.OrderBuild, State: game.OrderStateUnitReady, BuildRate: 2}
	if got := NetProduction(done, 4); got.Raw != 0 || got.Power != 0 {
		t.Errorf("finished factory draws %+v, want zero", got)
	}
}
