// baselayout_test.go : testing unit for the AI base layout escape guard and the trapped
// builder rescue.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import (
	"testing"

	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/units"
)

// mooreOffsets are the eight cells around a cell, the ring a wall test seals.
var mooreOffsets = [][2]int{{-1, -1}, {0, -1}, {1, -1}, {-1, 0}, {1, 0}, {-1, 1}, {0, 1}, {1, 1}}

// newLandGame builds a bare all land computer game for the layout tests.
func newLandGame(w, h int) (*sim.Game, *units.Manager) {
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[1].Type = game.TeamComputer
	return sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h), mgr
}

// TestBuilderCanRoamDetectsPocket locks the trapped detection: a constructor walled in on
// every side cannot roam; open one wall (its wreck leaves drivable rubble) and it can.
func TestBuilderCanRoamDetectsPocket(t *testing.T) {
	g, mgr := newLandGame(20, 20)
	con := constructorUnit(1, 1, 10, 10, mgr)

	var north *game.UnitInfo
	id := 100
	for _, d := range mooreOffsets {
		wall := stationary(1, id, assets.Research, 10+d[0], 10+d[1], mgr)
		if d == [2]int{0, -1} {
			north = wall
		}
		id++
	}

	bm := newBuildManager(g, 1)
	if bm.canRoam(con) {
		t.Fatal("a fully walled constructor should read as boxed in (canRoam false)")
	}

	g.DestroyUnit(north) // opens the north cell to drivable rubble
	if !bm.canRoam(con) {
		t.Fatal("after a wall opens the constructor should roam out to open ground")
	}
}

// TestRescueFreesTrappedConstructorByDemolition checks the first recovery: a constructor
// boxed in by its own base, with a surplus storage dump among the walls, is freed by
// blowing up that dump (the least needed neighbour) - not a replacement.
func TestRescueFreesTrappedConstructorByDemolition(t *testing.T) {
	g, mgr := newLandGame(20, 20)
	con := constructorUnit(1, 1, 10, 10, mgr)

	var dump *game.UnitInfo
	id := 100
	for _, d := range mooreOffsets {
		ut := assets.Research // load bearing walls (never demolished)
		if d == [2]int{0, -1} {
			ut = assets.ADump // a surplus dump on the north wall (demolishable)
		}
		wall := stationary(1, id, ut, 10+d[0], 10+d[1], mgr)
		if ut == assets.ADump {
			dump = wall
		}
		id++
	}

	bm := newBuildManager(g, 1)
	bm.rescueTrappedBuilders()

	if hashHas(g, dump, 10, 9) {
		t.Error("the surplus dump was not demolished to free the trapped constructor")
	}
	if !bm.canRoam(con) {
		t.Error("the constructor is still boxed in after the demolition")
	}
	if findOrder(g, assets.Constrct) != nil {
		t.Error("a replacement constructor was built even though demolition freed the trapped one")
	}
}

// TestRescueBuildsReplacementWhenWalledByCritical checks the fallback recovery: a
// constructor boxed in only by load bearing buildings (nothing safe to demolish) is left
// in place and a fresh constructor is built at the Heavy Vehicle Plant.
func TestRescueBuildsReplacementWhenWalledByCritical(t *testing.T) {
	g, mgr := newLandGame(20, 20)
	constructorUnit(1, 1, 10, 10, mgr)

	id := 100
	for _, d := range mooreOffsets {
		stationary(1, id, assets.Research, 10+d[0], 10+d[1], mgr) // all load bearing: none may be demolished
		id++
	}
	stationary(1, 200, assets.LandPlt, 3, 3, mgr) // an idle Heavy plant with open ground to deploy into

	bm := newBuildManager(g, 1)
	bm.rescueTrappedBuilders()

	if findOrder(g, assets.Constrct) == nil {
		t.Error("no replacement constructor was built when the trapped one could not be freed")
	}
}

// hashHas reports whether u is still hashed at (x, y) - a demolished unit is not.
func hashHas(g *sim.Game, u *game.UnitInfo, x, y int) bool {
	for _, o := range g.Units.Cells.At(x, y) {
		if o == u {
			return true
		}
	}
	return false
}
