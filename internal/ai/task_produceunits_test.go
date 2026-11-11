// task_produceunits_test.go : testing unit for task_produceunits.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
)

// TestProductionUsesOnlyItsOwnTeamsPlants checks isIdleFactory's team rule: production
// never gives an order to an idle plant of another team.
func TestProductionUsesOnlyItsOwnTeamsPlants(t *testing.T) {
	f := newObtainFixture(t)
	liveSurveyor(1, 2, 2, f.units) // team 1's recon need is met
	theirs := stationary(2, 60, assets.LightPlt, 10, 10, f.units)

	production := newTaskProduceUnits(1, StrategyCombinedArms, f.ctx)
	production.BeginTurn()

	if f.g.BuildOrderFor(theirs) != nil {
		t.Error("team 1's production ordered a unit at team 2's plant")
	}
	if f.g.BuildOrderFor(f.plant) == nil {
		t.Error("team 1's own idle plant got no order")
	}
}

// TestChooseProductTieKeepsListOrder checks the strict compare: with no unit of any type
// fielded every ratio is 0, a tie, and the first product in the factory list wins.
func TestChooseProductTieKeepsListOrder(t *testing.T) {
	mix := func(assets.UnitType) int { return 0 }
	for factory, list := range factoryProducts {
		got, ok := chooseProduct(factory, StrategyCombinedArms, mix, nil)
		if !ok || got.t != list[0].t {
			t.Errorf("%s: a tie picked %s, want the first listed %s", factory.Tag(), got.t.Tag(), list[0].t.Tag())
		}
	}
}
