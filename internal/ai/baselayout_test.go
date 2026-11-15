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

// ringOfTanks parks one armed own tank on each of the eight cells around (x, y), skipping
// the cell named by skip (use [2]int{0, 0}, the center, to skip none), and returns them.
func ringOfTanks(mgr *units.Manager, x, y int, skip [2]int) []*game.UnitInfo {
	var tanks []*game.UnitInfo
	id := 300

	for _, d := range mooreOffsets {
		if d == skip {
			continue
		}

		tanks = append(tanks, armedTank(1, id, x+d[0], y+d[1], mgr))
		id++
	}

	return tanks
}

// TestOwnUnitsInTheWay checks that the flood reports parked friendly units and rejects
// pockets blocked by buildings, enemies, busy units, or insufficient open ground.
func TestOwnUnitsInTheWay(t *testing.T) {
	none := [2]int{0, 0}
	cases := []struct {
		name      string
		setup     func(mgr *units.Manager)
		wantOK    bool
		wantCells int
	}{
		{"boxed by own tanks", func(mgr *units.Manager) {
			ringOfTanks(mgr, 10, 10, none)
		}, true, 8},
		{"boxed by tanks and one building: the building is no cell to clear", func(mgr *units.Manager) {
			ringOfTanks(mgr, 10, 10, [2]int{0, -1})
			stationary(1, 400, assets.ADump, 10, 9, mgr)
		}, true, 7},
		{"boxed by buildings only", func(mgr *units.Manager) {
			for i, d := range mooreOffsets {
				stationary(1, 400+i, assets.Research, 10+d[0], 10+d[1], mgr)
			}
		}, false, 0},
		{"boxed by enemy tanks", func(mgr *units.Manager) {
			for i, d := range mooreOffsets {
				armedTank(2, 400+i, 10+d[0], 10+d[1], mgr)
			}
		}, false, 0},
		{"boxed by tanks that are busy", func(mgr *units.Manager) {
			for _, tank := range ringOfTanks(mgr, 10, 10, none) {
				tank.Building = true
			}
		}, false, 0},
		{"one busy tank in the ring is a wall, the rest still open a way", func(mgr *units.Manager) {
			ring := ringOfTanks(mgr, 10, 10, none)
			ring[1].Building = true // (0,-1): the north cell
		}, true, 7},
		{"open ground: reached, nothing to clear", func(mgr *units.Manager) {}, true, 0},
		{"the cell the builder stands on may hold a building", func(mgr *units.Manager) {
			stationary(1, 400, assets.Research, 10, 10, mgr)
		}, true, 0},
	}

	for _, c := range cases {
		g, mgr := newLandGame(20, 20)
		con := constructorUnit(1, 1, 10, 10, mgr)
		con.Hits = 24 // alive and idle, so it would count as a clearable unit if it stayed in the hash
		c.setup(mgr)
		bm := newBuildManager(g, 1)

		cells, ok := bm.ownUnitsInTheWay(con)

		for _, cell := range cells {
			if cell == [2]int{10, 10} {
				t.Errorf("%s: the builder's own cell is listed as a cell to clear", c.name)
			}
		}

		if ok != c.wantOK || (c.wantOK && len(cells) != c.wantCells) {
			t.Errorf("%s: ok = %v with %d cells, want %v with %d", c.name, ok, len(cells), c.wantOK, c.wantCells)
		}

		if !hashHas(g, con, 10, 10) {
			t.Errorf("%s: the builder is not back in the cell hash", c.name)
		}
	}
}

// TestOwnUnitsInTheWayReachesTheMapEdge checks that the flood includes units on the map
// edge when the builder stands beside it.
func TestOwnUnitsInTheWayReachesTheMapEdge(t *testing.T) {
	g, mgr := newLandGame(20, 20)
	con := constructorUnit(1, 1, 1, 5, mgr)
	ringOfTanks(mgr, 1, 5, [2]int{0, 0})
	cells, ok := newBuildManager(g, 1).ownUnitsInTheWay(con)

	found := false
	for _, c := range cells {
		if c == [2]int{0, 5} {
			found = true
		}
	}

	if !ok || !found {
		t.Errorf("ok = %v, cells = %v, want the edge tank cell (0,5) among them", ok, cells)
	}
}

// TestOwnUnitsInTheWayCountsTwelveCells checks the reach threshold on a one-row map and
// verifies that returned cells follow flood order.
func TestOwnUnitsInTheWayCountsTwelveCells(t *testing.T) {
	for _, c := range []struct {
		length int
		wantOK bool
	}{{11, false}, {12, true}} {
		f := newClearFixture(c.length, 1, allLand{}) // a one row map: its edges are the walls
		con := constructorUnit(1, 1, 0, 0, f.mgr)

		for x := 1; x < c.length; x++ {
			armedTank(1, 100+x, x, 0, f.mgr)
		}

		cells, ok := f.manager().ownUnitsInTheWay(con)

		if ok != c.wantOK {
			t.Errorf("a strip of %d cells: ok = %v, want %v", c.length, ok, c.wantOK)
		}

		if c.wantOK {
			if len(cells) != 11 || cells[0] != [2]int{1, 0} || cells[10] != [2]int{11, 0} {
				t.Errorf("a strip of %d cells: cells = %v, want x = 1..11 in flood order", c.length, cells)
			}
		}
	}
}

// TestRescueClearsOwnUnitsInsteadOfDemolishing checks that friendly tanks trigger a clear
// zone, while buildings still require demolition and a manager without a clearing task
// keeps its fallback behavior.
func TestRescueClearsOwnUnitsInsteadOfDemolishing(t *testing.T) {
	// boxed puts a constructor at (10,10) with an ADump on its north cell and tanks (or walls) elsewhere.
	boxed := func(tanksAround bool) (*sim.Game, *game.UnitInfo, *game.UnitInfo) {
		g, mgr := newLandGame(20, 20)
		con := constructorUnit(1, 1, 10, 10, mgr)
		dump := stationary(1, 400, assets.ADump, 10, 9, mgr)

		for i, d := range mooreOffsets {
			if d == [2]int{0, -1} {
				continue
			}

			if tanksAround {
				armedTank(1, 300+i, 10+d[0], 10+d[1], mgr)
			} else {
				stationary(1, 410+i, assets.Research, 10+d[0], 10+d[1], mgr)
			}
		}

		stationary(1, 200, assets.LandPlt, 3, 3, mgr) // an idle plant: a replacement builder would be ordered here

		return g, con, dump
	}

	t.Run("boxed by own tanks: a zone, no demolition, no replacement", func(t *testing.T) {
		g, con, dump := boxed(true)
		bm := newBuildManager(g, 1)
		bm.clearer = newTaskClearZone(1, bm.ctx)
		bm.rescueTrappedBuilders()
		zones := bm.clearer.zones

		if !hashHas(g, dump, 10, 9) {
			t.Error("the adjacent dump was demolished although the tanks could be moved aside")
		}

		if len(zones) != 1 || zones[0].requester != con || len(zones[0].cells) != 7 {
			t.Errorf("zones = %+v, want one zone of the 7 tank cells for the builder", zones)
		}

		if findOrder(g, assets.Constrct) != nil {
			t.Error("a replacement constructor was ordered although the builder is about to be freed")
		}
	})

	t.Run("sealed by buildings: demolition stays the last resort", func(t *testing.T) {
		g, _, dump := boxed(false)
		bm := newBuildManager(g, 1)
		bm.clearer = newTaskClearZone(1, bm.ctx)
		bm.rescueTrappedBuilders()

		if hashHas(g, dump, 10, 9) {
			t.Error("the dump was not demolished for a builder sealed by buildings")
		}

		if len(bm.clearer.zones) != 0 {
			t.Errorf("zones = %+v, want none for a builder sealed by buildings", bm.clearer.zones)
		}
	})

	t.Run("sealed by buildings with a tank inside the pocket: demolition", func(t *testing.T) {
		g, mgr := newLandGame(20, 20)
		constructorUnit(1, 1, 10, 10, mgr)
		dump := stationary(1, 400, assets.ADump, 9, 10, mgr)
		armedTank(1, 300, 10, 9, mgr) // the only free neighbor, a dead end one cell deep

		for i, c := range [][2]int{{9, 9}, {11, 9}, {11, 10}, {9, 11}, {10, 11}, {11, 11}, {9, 8}, {10, 8}, {11, 8}} {
			stationary(1, 410+i, assets.Research, c[0], c[1], mgr)
		}

		bm := newBuildManager(g, 1)
		bm.clearer = newTaskClearZone(1, bm.ctx)
		bm.rescueTrappedBuilders()

		if hashHas(g, dump, 9, 10) {
			t.Error("the dump was not demolished: a tank in a dead end pocket is no way out")
		}

		if len(bm.clearer.zones) != 0 {
			t.Errorf("zones = %+v, want none for a dead end pocket", bm.clearer.zones)
		}
	})

	t.Run("a bare manager keeps the old behavior", func(t *testing.T) {
		g, _, dump := boxed(true)
		bm := newBuildManager(g, 1)
		bm.rescueTrappedBuilders()

		if hashHas(g, dump, 10, 9) {
			t.Error("the dump was not demolished by a manager with no clearing task")
		}
	})
}

// TestUsableBuilders checks that working, mobile, and soon-to-be-freed builders count, but
// builders sealed by buildings do not.
func TestUsableBuilders(t *testing.T) {
	sealed := func(mgr *units.Manager) {
		for i, d := range mooreOffsets {
			stationary(1, 400+i, assets.Research, 10+d[0], 10+d[1], mgr)
		}
	}
	boxed := func(mgr *units.Manager) { ringOfTanks(mgr, 10, 10, [2]int{0, 0}) }
	open := func(mgr *units.Manager) {}

	cases := []struct {
		name    string
		setup   func(mgr *units.Manager)
		atWork  bool // the constructor is raising a structure
		clearer bool // the manager has a clearing task
		want    int
	}{
		{"boxed by own tanks, with a clearing task", boxed, false, true, 1},
		{"boxed by own tanks, without a clearing task", boxed, false, false, 0},
		{"free to roam, without a clearing task", open, false, false, 1},
		{"free to roam, with a clearing task", open, false, true, 1},
		{"sealed by buildings, with a clearing task", sealed, false, true, 0},
		{"sealed by buildings but at work", sealed, true, false, 1},
	}

	for _, c := range cases {
		g, mgr := newLandGame(20, 20)
		con := constructorUnit(1, 1, 10, 10, mgr)
		c.setup(mgr)
		con.Building = c.atWork
		bm := newBuildManager(g, 1)

		if c.clearer {
			bm.clearer = newTaskClearZone(1, bm.ctx)
		}

		if got := bm.usableBuilders(assets.Constrct); got != c.want {
			t.Errorf("%s: usable constructors = %d, want %d", c.name, got, c.want)
		}
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
