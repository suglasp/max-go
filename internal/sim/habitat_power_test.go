// habitat_power_test.go : tests the freshly built Habitat auto power on rule.
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

// TestBuiltHabitatAutoPowersOn verifies that a completed HABITAT starts active and supplies its three workers.
// C++: UnitsManager_DeployUnit assigns ORDER_POWER_ON to MININGST, HABITAT, and COMMTWR.
func TestBuiltHabitatAutoPowersOn(t *testing.T) {
	g := newGame(t, 16, 16)

	con := game.NewUnitInfo(assets.Constrct, 0, 4, 0)
	con.Flags = game.FlagMobileLandUnit | game.FlagConstructorUnit | game.FlagSelectable
	con.Values = &game.UnitValues{}
	con.InitState()
	con.Storage = 500 // build from its own cargo
	g.Units.Deploy(con, 6, 6)

	hab := game.NewUnitInfo(assets.Habitat, 0, 5, 0)
	hab.Flags = game.FlagBuilding | game.FlagStationary | game.FlagSelectable
	hab.Values = &game.UnitValues{}
	hab.Values.Set(game.AttribTurns, 1)
	hab.Values.Set(game.AttribHits, 10)
	g.StartBuild(con, hab, 6, 6, 1)
	g.AdvanceBuilds(0) // a 1-turn building finishes this turn

	if hab.Order != game.OrderPowerOn {
		t.Fatalf("a freshly-built Habitat order = %v, want OrderPowerOn (so it is active and houses workers)", hab.Order)
	}
	if !economy.IsActive(hab) {
		t.Error("a freshly-built Habitat must be active; an inactive habitat houses 0 workers and blocks eco-sphere/research/training")
	}
}

// TestBuiltDefaultBuildingStaysPowerOff verifies that a non-provider such as an Eco Sphere still starts with OrderPowerOff.
func TestBuiltDefaultBuildingStaysPowerOff(t *testing.T) {
	g := newGame(t, 16, 16)

	con := game.NewUnitInfo(assets.Constrct, 0, 4, 0)
	con.Flags = game.FlagMobileLandUnit | game.FlagConstructorUnit | game.FlagSelectable
	con.Values = &game.UnitValues{}
	con.InitState()
	con.Storage = 500
	g.Units.Deploy(con, 6, 6)

	eco := game.NewUnitInfo(assets.Greenhse, 0, 5, 0)
	eco.Flags = game.FlagBuilding | game.FlagStationary | game.FlagSelectable
	eco.Values = &game.UnitValues{}
	eco.Values.Set(game.AttribTurns, 1)
	eco.Values.Set(game.AttribHits, 10)
	g.StartBuild(con, eco, 6, 6, 1)
	g.AdvanceBuilds(0)

	if eco.Order != game.OrderPowerOff {
		t.Errorf("a freshly-built eco-sphere order = %v, want OrderPowerOff (player powers it on)", eco.Order)
	}
}
