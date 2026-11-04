// held_units_sweep_test.go : sweeps every factory and storage bay for the rules on the units they hold.
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
	"github.com/suglasp/max-go/internal/paths"
)

// checkParkedSilent checks that a unit waiting inside a building scans nothing: given a scan range, the cell three
// steps east of it stays uncovered.
func checkParkedSilent(t *testing.T, g *Game, parked *game.UnitInfo, name string) {
	t.Helper()
	if !parked.HiddenUntilExpand {
		t.Errorf("%s: the waiting unit is not hidden", name)
	}
	parked.Values.Set(game.AttribScan, 3)
	g.RefreshVisibility()
	if g.Vis.Team(parked.Team).Complete(parked.GridX+3, parked.GridY) != 0 {
		t.Errorf("%s: the waiting unit scans", name)
	}
}

// checkGone checks that a unit is off the map and out of the unit registry.
func checkGone(t *testing.T, g *Game, u *game.UnitInfo, cellX, cellY int, name string) {
	t.Helper()
	if g.Units.ByID.Get(u.ID) != nil {
		t.Errorf("%s: %s survived its building", name, u.Type.Tag())
	}
	for _, c := range g.Units.Cells.At(cellX, cellY) {
		if c == u {
			t.Errorf("%s: %s still stands on (%d,%d)", name, u.Type.Tag(), cellX, cellY)
		}
	}
}

// TestFactoriesLoseTheirWaitingUnit checks every factory: its finished unit waits hidden and silent, and is lost when
// the factory is destroyed before it drove out.
func TestFactoriesLoseTheirWaitingUnit(t *testing.T) {
	for _, tc := range []struct {
		factory, product assets.UnitType
	}{
		{assets.Shipyard, assets.Corvette},
		{assets.LightPlt, assets.Scout},
		{assets.LandPlt, assets.Tank},
		{assets.AirPlt, assets.Fighter},
		{assets.TrainHal, assets.Infantry},
	} {
		name := tc.factory.Tag()
		g := newGame(t, 16, 16)
		g.Resources = economy.NewResourceMap(16, 16)
		g.MakeUnit = factoryUnitMaker(2)
		f := stationaryBuilding(tc.factory, 0, 4, 2)
		g.PlaceBuilding(f, 4, 2)
		f.Storage = 1000
		g.SetFactoryQueue(f, []assets.UnitType{tc.product}, 1, false)
		buildUntilReady(t, g, f)

		parked := g.BuildReadyInfo(f).Building
		checkParkedSilent(t, g, parked, name)

		g.DestroyUnit(f)
		checkGone(t, g, parked, 4, 2, name)
	}
}

// TestStorageBaysLoseTheirUnits checks every storage bay: its stored units are off the map and scan nothing, and when
// the bay is destroyed they are lost with it, with no route or timer left behind. (A bay unloads straight onto an
// adjacent cell, so it never parks a waiting unit the way a factory does.)
func TestStorageBaysLoseTheirUnits(t *testing.T) {
	for _, tc := range []struct {
		bay, cargo assets.UnitType
		flags      game.Flags
	}{
		{assets.Depot, assets.Tank, game.FlagMobileLandUnit},
		{assets.Dock, assets.Corvette, game.FlagMobileSeaUnit},
		{assets.Hangar, assets.Fighter, game.FlagMobileAirUnit},
		{assets.Barracks, assets.Infantry, game.FlagMobileLandUnit},
	} {
		name := tc.bay.Tag()
		g := newGame(t, 16, 16)
		bay := stationaryBuilding(tc.bay, 0, 4, 2)
		g.PlaceBuilding(bay, 4, 2)

		var stored []*game.UnitInfo
		for i := 0; i < 2; i++ {
			u := game.NewUnitInfo(tc.cargo, 0, 500+i, 0)
			u.Flags = tc.flags | game.FlagSelectable
			u.Values = &game.UnitValues{}
			u.Values.Set(game.AttribHits, 10)
			u.Values.Set(game.AttribScan, 3)
			u.InitState()
			g.Units.Deploy(u, 10, 10+i)
			// Stored as LoadUnit does: off the map, carried by the bay.
			g.Units.Remove(u)
			u.Parent = bay
			bay.Carried = append(bay.Carried, u)
			stored = append(stored, u)
		}
		// Residue the death must clear.
		if g.pending == nil {
			g.pending = make(map[*game.UnitInfo][]paths.Step)
		}
		if g.held == nil {
			g.held = make(map[*game.UnitInfo][]paths.Step)
		}
		g.pending[stored[0]] = nil
		g.held[stored[1]] = nil

		g.RefreshVisibility()
		if g.Vis.Team(0).Complete(bay.GridX+5, bay.GridY) != 0 {
			t.Errorf("%s: a stored unit scans", name)
		}

		g.DestroyUnit(bay)
		for _, u := range stored {
			checkGone(t, g, u, 4, 2, name)
		}
		if _, left := g.pending[stored[0]]; left {
			t.Errorf("%s: a stored unit kept a pending route", name)
		}
		if _, left := g.held[stored[1]]; left {
			t.Errorf("%s: a stored unit kept a held route", name)
		}
	}
}
