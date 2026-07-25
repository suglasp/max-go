// power_optimize_test.go : tests automatic generator selection.
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

// TestTargetGenerators checks the minimum fuel generator mix from
// ProductionManager::OptimizePowerProduction). A Power Station makes 6 power/8 fuel
// (1.33 fuel/power), a Power Generator 1 power/2 fuel (2 fuel/power), so stations
// carry the bulk and generators fill the small remainder, with a station preferred
// at the crossover (4 power). When generators are too few it returns the most they
// can supply.
func TestTargetGenerators(t *testing.T) {
	cases := []struct {
		name                           string
		demand, stationCount, genCount int
		wantStations, wantGenerators   int
	}{
		{"no demand", 0, 4, 4, 0, 0},
		{"one power -> one generator (cheaper than a station)", 1, 5, 5, 0, 1},
		{"three power -> three generators (6 fuel < 8)", 3, 5, 5, 0, 3},
		{"four power -> one station at the crossover", 4, 5, 5, 1, 0},
		{"six power -> one station", 6, 5, 5, 1, 0},
		{"seven power -> a station plus a generator", 7, 5, 5, 1, 1},
		{"twelve power -> two stations, not twelve generators", 12, 5, 5, 2, 0},
		{"capacity short: only two generators for three demand", 3, 0, 2, 0, 2},
		{"only a station available is used for one power", 1, 1, 0, 1, 0},
	}
	for _, tc := range cases {
		stations, generators := targetGenerators(tc.demand, tc.stationCount, tc.genCount)
		if stations != tc.wantStations || generators != tc.wantGenerators {
			t.Errorf("%s: targetGenerators(%d,%d,%d) = (%d stations, %d gens), want (%d, %d)",
				tc.name, tc.demand, tc.stationCount, tc.genCount, stations, generators, tc.wantStations, tc.wantGenerators)
		}
	}
}

// TestOptimizerPicksMinFuel: a complex with one mine (demand 1) plus BOTH a station
// and a generator powers ONLY the cheaper generator and leaves the station off - the
// automatic management the player relies on to save fuel.
func TestOptimizerPicksMinFuel(t *testing.T) {
	gen := powerGen(game.OrderPowerOff)
	station := &game.UnitInfo{Type: assets.PowerStn, Order: game.OrderPowerOn, State: game.OrderStateExecutingOrder}
	c := &Complex{Members: []*game.UnitInfo{miningStation(5, game.OrderPowerOn), gen, station}, Fuel: 50}

	c.BalanceTurn(4)

	if gen.Order != game.OrderPowerOn {
		t.Errorf("the cheap generator should power on for the mine, got %v", gen.Order)
	}
	if station.Order != game.OrderPowerOff {
		t.Errorf("the costly station should be left off (min fuel), got %v", station.Order)
	}
}

// TestShedOrderKlei: when one small generator cannot power a mine plus research,
// Eco Sphere, gold refinery and a producing factory, OptimizeProduction sheds in
// shed order - Research, then Eco Sphere, then Gold Refinery, then the Factory -
// leaving the MINE running. Aux buildings are durably powered off; the factory is
// HELD (PowerStarved, its build resumes when power returns).
func TestShedOrderKlei(t *testing.T) {
	mine := miningStation(5, game.OrderPowerOn)
	research := &game.UnitInfo{Type: assets.Research, Order: game.OrderPowerOn, State: game.OrderStateExecutingOrder}
	eco := greenhouse(game.OrderPowerOn)
	refinery := &game.UnitInfo{Type: assets.CommTwr, Order: game.OrderPowerOn, State: game.OrderStateExecutingOrder}
	factory := &game.UnitInfo{Type: assets.LightPlt, Order: game.OrderPowerOn, State: game.OrderStateExecutingOrder, BuildRate: 1}
	gen := powerGen(game.OrderPowerOn) // one generator: 1 power for 5 demand
	c := &Complex{Members: []*game.UnitInfo{mine, research, eco, refinery, factory, gen}, Fuel: 50}

	shed := c.OptimizeProduction()

	// Shed order: Research, Eco Sphere, Gold Refinery, Factory (mine never shed).
	want := []*game.UnitInfo{research, eco, refinery, factory}
	if len(shed) != len(want) {
		t.Fatalf("shed %d buildings, want %d (%v)", len(shed), len(want), shed)
	}
	for i, u := range want {
		if shed[i] != u {
			t.Errorf("shed[%d] = %v, want %v (Klei order Research->Eco->Gold->Factory)", i, shed[i].Type, u.Type)
		}
	}
	for _, aux := range []*game.UnitInfo{research, eco, refinery} {
		if aux.Order != game.OrderPowerOff {
			t.Errorf("%v should be durably powered off, got %v", aux.Type, aux.Order)
		}
	}
	if !factory.PowerStarved {
		t.Error("the factory should be HELD (PowerStarved), not durably off")
	}
	if factory.Order != game.OrderPowerOn {
		t.Errorf("a held factory keeps Order PowerOn (build paused), got %v", factory.Order)
	}
	if mine.Order != game.OrderPowerOn || mine.PowerStarved {
		t.Errorf("the mine must keep running (shed last), order=%v starved=%v", mine.Order, mine.PowerStarved)
	}
}

// TestCanAbsorbPower checks the on demand power on capacity gate: a complex whose
// generators are fully committed cannot absorb another consumer, but adding a
// generator gives it the headroom.
func TestCanAbsorbPower(t *testing.T) {
	mine := miningStation(5, game.OrderPowerOn)
	c := &Complex{Members: []*game.UnitInfo{mine, powerGen(game.OrderPowerOn)}, Fuel: 50}
	if c.CanAbsorbPower(1) {
		t.Error("a complex with one generator powering one mine cannot absorb a second consumer")
	}
	c.Members = append(c.Members, powerGen(game.OrderPowerOff)) // a spare generator
	if !c.CanAbsorbPower(1) {
		t.Error("with a spare generator the complex should absorb the new consumer's draw")
	}
}
