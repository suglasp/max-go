// life_shed_game_test.go : tests the worker shed through the real destroy path.
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

// TestLifeShedAfterHabitatDestroyedInGame verifies that destroying the only Habitat removes it from the complex and powers off the newly unstaffed Eco Sphere.
func TestLifeShedAfterHabitatDestroyedInGame(t *testing.T) {
	g := newGame(t, 16, 16)
	g.Resources = economy.NewResourceMap(16, 16)
	g.Resources.SetDeposit(2, 2, economy.CargoRaw, 16)

	// One complex: mine + power station (plenty of power so only WORKERS gate),
	// a habitat (houses 3 workers) and an Eco Sphere (1 worker, 1 power).
	mine := stationaryBuilding(assets.MiningSt, 0, 2, 2)
	mine.RawMining = 5
	mine.TotalMining = 5
	g.PlaceBuilding(mine, 2, 2)
	stn := stationaryBuilding(assets.PowerStn, 0, 4, 2)
	g.PlaceBuilding(stn, 4, 2)
	hab := stationaryBuilding(assets.Habitat, 0, 2, 4)
	g.PlaceBuilding(hab, 2, 4)
	gh := stationaryBuilding(assets.Greenhse, 0, 4, 4)
	g.PlaceBuilding(gh, 4, 4)
	for _, cx := range g.Complexes {
		cx.Fuel = 50
	}

	// Settle: the Eco Sphere is powered and staffed, so it runs.
	g.RunEconomy(0)
	if gh.Order != game.OrderPowerOn {
		t.Fatalf("pre-condition: the eco-sphere should run (powered + staffed), got %v", gh.Order)
	}

	// Blow up the only habitat: it must LEAVE its complex (the lingering bug).
	g.DestroyUnit(hab)
	if g.ComplexOf(hab) != nil {
		t.Fatal("a destroyed habitat must leave its complex")
	}

	// Next reconcile sheds the now unstaffed Eco Sphere and reports it for the banner.
	g.RunEconomy(0)
	if gh.Order != game.OrderPowerOff {
		t.Errorf("eco-sphere order after losing the habitat = %v, want OrderPowerOff (worker shed)", gh.Order)
	}
	if lost := g.LastLifeLost(); len(lost) != 1 || lost[0] != gh {
		t.Errorf("LastLifeLost = %v, want exactly the eco-sphere", lost)
	}
}
