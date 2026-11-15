// task_clearzone_test.go covers the zone clearing task and its building-manager wiring.
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
	"github.com/suglasp/max-go/internal/world"
)

// landPatch is a surface source with land inside one rectangle and water all around it, so a
// test can fence a pocket in for tanks without placing a wall building. A builder crosses
// water, so a test that floods from a builder fences with the map edge instead.
type landPatch struct {
	startX, startY, endX, endY int // the land cells: startX <= x < endX, startY <= y < endY
}

// SurfaceAt returns land inside the patch and water outside it.
func (p landPatch) SurfaceAt(x, y int) uint8 {
	if x >= p.startX && x < p.endX && y >= p.startY && y < p.endY {
		return world.SurfaceLand
	}

	return world.SurfaceWater
}

// clearFixture is a game for team 1 (computer) with a bare zone clearing task.
type clearFixture struct {
	g     *sim.Game
	mgr   *units.Manager
	ctx   *aiContext
	zones *taskClearZone
}

// newClearFixture builds a w x h game on the given surface with a zone clearing task for team 1.
func newClearFixture(w, h int, surf access.SurfaceSource) *clearFixture {
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: surf, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)
	ctx := &aiContext{Game: g, MakeUnit: testUnitFactory}

	return &clearFixture{g: g, mgr: mgr, ctx: ctx, zones: newTaskClearZone(1, ctx)}
}

// manager returns a building manager of team 1 wired to the fixture's clearing task.
func (f *clearFixture) manager() *taskManageBuildings {
	bm := newTaskManageBuildings(1, StrategyCombinedArms, goalKill, false, f.ctx)
	bm.clearer = f.zones

	return bm
}

// moveDestination returns the cell the unit's active move order ends at.
func moveDestination(g *sim.Game, u *game.UnitInfo) ([2]int, bool) {
	for _, o := range g.MoveOrders() {
		if o.Unit == u && len(o.Steps) > 0 {
			last := o.Steps[len(o.Steps)-1].To

			return [2]int{last.X, last.Y}, true
		}
	}

	return [2]int{}, false
}

// cellCount returns how many times u is hashed at (x, y).
func cellCount(g *sim.Game, u *game.UnitInfo, x, y int) int {
	n := 0

	for _, o := range g.Units.Cells.At(x, y) {
		if o == u {
			n++
		}
	}

	return n
}

// TestAddZoneKeepsOneZonePerRequester checks that a requester replaces its zone and resets
// its timer, while another requester can open a separate zone.
func TestAddZoneKeepsOneZonePerRequester(t *testing.T) {
	f := newClearFixture(20, 20, allLand{})
	first := constructorUnit(1, 1, 3, 3, f.mgr)
	second := constructorUnit(1, 2, 12, 12, f.mgr)

	f.zones.addZone(first, [][2]int{{4, 4}})
	f.zones.zones[0].turnsLeft = 1
	f.zones.addZone(first, [][2]int{{5, 5}, {6, 5}})

	if len(f.zones.zones) != 1 {
		t.Fatalf("the same requester holds %d zones, want 1", len(f.zones.zones))
	}

	if got := f.zones.zones[0]; len(got.cells) != 2 || got.cells[0] != [2]int{5, 5} || got.turnsLeft != clearZoneTurns {
		t.Errorf("the replaced zone = %+v, want the new two cells and %d turns", got, clearZoneTurns)
	}

	f.zones.addZone(second, [][2]int{{13, 13}})

	if len(f.zones.zones) != 2 || f.zones.zones[1].requester != second {
		t.Errorf("a second requester gave %d zones, want 2 with the new one last", len(f.zones.zones))
	}
}

// TestZoneLifecycle checks when a zone stays open, completes, or expires, and which units
// BeginTurn moves.
func TestZoneLifecycle(t *testing.T) {
	cases := []struct {
		name      string
		setup     func(f *clearFixture) (zone clearZone, moved, still []*game.UnitInfo)
		wantKept  bool
		wantTurns int // the turns the kept zone has left
	}{
		{"a blocker keeps the zone open and its turns count down", func(f *clearFixture) (clearZone, []*game.UnitInfo, []*game.UnitInfo) {
			blocker := armedTank(1, 50, 5, 5, f.mgr)
			zone := clearZone{requester: constructorUnit(1, 1, 1, 1, f.mgr), cells: [][2]int{{5, 5}}, turnsLeft: 3}

			return zone, []*game.UnitInfo{blocker}, nil
		}, true, 2},
		{"no own unit on the zone: it is done", func(f *clearFixture) (clearZone, []*game.UnitInfo, []*game.UnitInfo) {
			bystander := armedTank(1, 50, 9, 9, f.mgr)
			zone := clearZone{requester: constructorUnit(1, 1, 1, 1, f.mgr), cells: [][2]int{{5, 5}}, turnsLeft: 3}

			return zone, nil, []*game.UnitInfo{bystander}
		}, false, 0},
		{"an enemy on the zone drops it and nothing moves", func(f *clearFixture) (clearZone, []*game.UnitInfo, []*game.UnitInfo) {
			own := armedTank(1, 50, 5, 5, f.mgr)
			armedTank(2, 51, 6, 5, f.mgr)
			zone := clearZone{requester: constructorUnit(1, 1, 1, 1, f.mgr), cells: [][2]int{{5, 5}, {6, 5}}, turnsLeft: 3}

			return zone, nil, []*game.UnitInfo{own}
		}, false, 0},
		{"a zone past its turns is dropped and nothing moves", func(f *clearFixture) (clearZone, []*game.UnitInfo, []*game.UnitInfo) {
			own := armedTank(1, 50, 5, 5, f.mgr)
			zone := clearZone{requester: constructorUnit(1, 1, 1, 1, f.mgr), cells: [][2]int{{5, 5}}, turnsLeft: 0}

			return zone, nil, []*game.UnitInfo{own}
		}, false, 0},
		{"the last turn still moves the unit", func(f *clearFixture) (clearZone, []*game.UnitInfo, []*game.UnitInfo) {
			own := armedTank(1, 50, 5, 5, f.mgr)
			zone := clearZone{requester: constructorUnit(1, 1, 1, 1, f.mgr), cells: [][2]int{{5, 5}}, turnsLeft: 1}

			return zone, []*game.UnitInfo{own}, nil
		}, true, 0},
		{"the requester is never moved, another unit on its zone is", func(f *clearFixture) (clearZone, []*game.UnitInfo, []*game.UnitInfo) {
			requester := armedTank(1, 50, 5, 5, f.mgr)
			other := armedTank(1, 51, 6, 5, f.mgr)
			zone := clearZone{requester: requester, cells: [][2]int{{5, 5}, {6, 5}}, turnsLeft: 3}

			return zone, []*game.UnitInfo{other}, []*game.UnitInfo{requester}
		}, true, 2},
		{"the requester alone on its zone: it is done", func(f *clearFixture) (clearZone, []*game.UnitInfo, []*game.UnitInfo) {
			requester := armedTank(1, 50, 5, 5, f.mgr)
			zone := clearZone{requester: requester, cells: [][2]int{{5, 5}}, turnsLeft: 3}

			return zone, nil, []*game.UnitInfo{requester}
		}, false, 0},
		{"a unit with a build order is not moved", func(f *clearFixture) (clearZone, []*game.UnitInfo, []*game.UnitInfo) {
			busy := armedTank(1, 50, 5, 5, f.mgr)
			f.g.StartBuild(busy, testUnitFactory(1, assets.Tank, game.FlagMobileLandUnit), 9, 9, 1)
			zone := clearZone{requester: constructorUnit(1, 1, 1, 1, f.mgr), cells: [][2]int{{5, 5}}, turnsLeft: 3}

			return zone, nil, []*game.UnitInfo{busy}
		}, false, 0},
		{"an own aircraft on the zone is ignored: the zone is done", func(f *clearFixture) (clearZone, []*game.UnitInfo, []*game.UnitInfo) {
			plane := armedUnit(1, 50, assets.Fighter, game.FlagMobileAirUnit|game.FlagSelectable, 5, 5, f.mgr)
			zone := clearZone{requester: constructorUnit(1, 1, 1, 1, f.mgr), cells: [][2]int{{5, 5}}, turnsLeft: 3}

			return zone, nil, []*game.UnitInfo{plane}
		}, false, 0},
		{"an enemy aircraft does not drop the zone", func(f *clearFixture) (clearZone, []*game.UnitInfo, []*game.UnitInfo) {
			own := armedTank(1, 50, 5, 5, f.mgr)
			armedUnit(2, 51, assets.Fighter, game.FlagMobileAirUnit|game.FlagSelectable, 6, 5, f.mgr)
			zone := clearZone{requester: constructorUnit(1, 1, 1, 1, f.mgr), cells: [][2]int{{5, 5}, {6, 5}}, turnsLeft: 3}

			return zone, []*game.UnitInfo{own}, nil
		}, true, 2},
		{"a unit that builds is not moved", func(f *clearFixture) (clearZone, []*game.UnitInfo, []*game.UnitInfo) {
			busy := armedTank(1, 50, 5, 5, f.mgr)
			busy.Building = true
			zone := clearZone{requester: constructorUnit(1, 1, 1, 1, f.mgr), cells: [][2]int{{5, 5}}, turnsLeft: 3}

			return zone, nil, []*game.UnitInfo{busy}
		}, false, 0},
		{"a dead unit is not moved", func(f *clearFixture) (clearZone, []*game.UnitInfo, []*game.UnitInfo) {
			dead := armedTank(1, 50, 5, 5, f.mgr)
			dead.Hits = 0
			zone := clearZone{requester: constructorUnit(1, 1, 1, 1, f.mgr), cells: [][2]int{{5, 5}}, turnsLeft: 3}

			return zone, nil, []*game.UnitInfo{dead}
		}, false, 0},
	}

	for _, c := range cases {
		f := newClearFixture(20, 20, allLand{})
		zone, moved, still := c.setup(f)
		f.zones.zones = []clearZone{zone}
		f.zones.BeginTurn()

		if kept := len(f.zones.zones) == 1; kept != c.wantKept {
			t.Errorf("%s: zone kept = %v, want %v", c.name, kept, c.wantKept)
		} else if kept && f.zones.zones[0].turnsLeft != c.wantTurns {
			t.Errorf("%s: turns left = %d, want %d", c.name, f.zones.zones[0].turnsLeft, c.wantTurns)
		}

		for _, u := range moved {
			if !f.g.HasOrder(u) {
				t.Errorf("%s: unit %d got no move order", c.name, u.ID)
			}
		}

		for _, u := range still {
			if f.g.HasOrder(u) {
				t.Errorf("%s: unit %d was moved but must stay", c.name, u.ID)
			}
		}
	}
}

// TestBeginTurnKeepsTheOtherZonesWhenOneIsDone checks that completing one zone preserves
// the other open zones.
func TestBeginTurnKeepsTheOtherZonesWhenOneIsDone(t *testing.T) {
	f := newClearFixture(20, 20, allLand{})
	doneFor := constructorUnit(1, 1, 1, 1, f.mgr)
	openFor := constructorUnit(1, 2, 2, 2, f.mgr)
	armedTank(1, 50, 5, 5, f.mgr)

	f.zones.addZone(doneFor, [][2]int{{9, 9}})
	f.zones.addZone(openFor, [][2]int{{5, 5}})
	f.zones.BeginTurn()

	if len(f.zones.zones) != 1 || f.zones.zones[0].requester != openFor {
		t.Fatalf("zones after the turn = %+v, want only the open one", f.zones.zones)
	}
}

// TestBeginTurnWithoutAGameDoesNothing checks the nil-game guard for an unwired task.
func TestBeginTurnWithoutAGameDoesNothing(t *testing.T) {
	zones := newTaskClearZone(1, &aiContext{})
	zones.zones = []clearZone{{turnsLeft: 2}}
	zones.BeginTurn()

	if len(zones.zones) != 1 || zones.zones[0].turnsLeft != 2 {
		t.Errorf("a task with no game changed its zones: %+v", zones.zones)
	}
}

// TestBusyUnitKeepsItsOrder checks that a unit with an existing move order stays untouched
// and its zone completes when it leaves.
func TestBusyUnitKeepsItsOrder(t *testing.T) {
	f := newClearFixture(20, 20, allLand{})
	busy := armedTank(1, 50, 5, 5, f.mgr)

	if !f.g.OrderMove(busy, 5, 12) {
		t.Fatal("setup: the busy tank got no order")
	}

	f.zones.addZone(constructorUnit(1, 1, 1, 1, f.mgr), [][2]int{{5, 5}})
	f.zones.BeginTurn()

	if dest, ok := moveDestination(f.g, busy); !ok || dest != [2]int{5, 12} {
		t.Errorf("the busy tank's order ends at %v (has order %v), want its own (5,12)", dest, ok)
	}

	if len(f.zones.zones) != 0 {
		t.Errorf("a zone held only by a busy unit stays open: %+v", f.zones.zones)
	}
}

// TestMoveAside checks that a blocker moves to the nearest free cell outside every zone,
// only the outer unit in a chain moves, and no unit moves when no cell is free.
func TestMoveAside(t *testing.T) {
	// strip builds a one row land strip x = 1..last on row 1 of a 12 x 3 map.
	strip := func(last int) *clearFixture {
		return newClearFixture(12, 3, landPatch{startX: 1, startY: 1, endX: last + 1, endY: 2})
	}

	t.Run("a single tank takes the nearest free cell", func(t *testing.T) {
		f := newClearFixture(20, 20, allLand{})
		tank := armedTank(1, 50, 5, 5, f.mgr)
		f.zones.addZone(constructorUnit(1, 1, 1, 1, f.mgr), [][2]int{{5, 5}})
		f.zones.BeginTurn()

		if dest, ok := moveDestination(f.g, tank); !ok || dest != [2]int{6, 5} {
			t.Errorf("the tank goes to %v (ordered %v), want the next free cell (6,5)", dest, ok)
		}
	})

	t.Run("the outer tank of a chain is the one ordered", func(t *testing.T) {
		f := strip(5)
		builder := constructorUnit(1, 1, 1, 1, f.mgr)
		inner := armedTank(1, 50, 2, 1, f.mgr)
		outer := armedTank(1, 51, 3, 1, f.mgr)
		f.zones.addZone(builder, [][2]int{{2, 1}, {3, 1}})
		f.zones.BeginTurn()

		if f.g.HasOrder(inner) {
			t.Error("the inner tank was ordered; its way is walled by the outer one")
		}

		if dest, ok := moveDestination(f.g, outer); !ok || dest != [2]int{4, 1} {
			t.Errorf("the outer tank goes to %v (ordered %v), want (4,1)", dest, ok)
		}
	})

	t.Run("no free cell: no order", func(t *testing.T) {
		f := strip(3)
		builder := constructorUnit(1, 1, 1, 1, f.mgr)
		inner := armedTank(1, 50, 2, 1, f.mgr)
		outer := armedTank(1, 51, 3, 1, f.mgr)
		f.zones.addZone(builder, [][2]int{{2, 1}, {3, 1}})
		f.zones.BeginTurn()

		if f.g.HasOrder(inner) || f.g.HasOrder(outer) {
			t.Error("a unit was ordered although the pocket has no free cell")
		}
	})

	t.Run("a busy unit is a wall for the chain", func(t *testing.T) {
		f := strip(5)
		manager := NewTaskManager()
		manager.AppendTask(f.zones)
		builder := constructorUnit(1, 1, 1, 1, f.mgr)
		inner := armedTank(1, 50, 2, 1, f.mgr)
		busy := armedTank(1, 51, 3, 1, f.mgr)
		busy.Building = true
		f.zones.addZone(builder, [][2]int{{2, 1}})
		f.zones.BeginTurn()

		if f.g.HasOrder(inner) || f.g.HasOrder(busy) {
			t.Error("the tank was moved through a unit that cannot move")
		}

		if manager.owner[inner] != nil {
			t.Error("the tank is held by the clearing task although it was not moved")
		}
	})

	t.Run("the nearest way out may lie past a busy unit's wall: the chain goes the other way", func(t *testing.T) {
		f := newClearFixture(8, 3, landPatch{startX: 0, startY: 1, endX: 6, endY: 2})
		dummy := engineerUnit(1, 2, 7, 0, f.mgr)
		west := armedTank(1, 50, 1, 1, f.mgr)
		middle := armedTank(1, 51, 2, 1, f.mgr)
		blocker := armedTank(1, 52, 3, 1, f.mgr)
		busy := armedTank(1, 53, 4, 1, f.mgr)
		busy.Building = true
		f.zones.addZone(dummy, [][2]int{{3, 1}})
		f.zones.BeginTurn()

		if dest, ok := moveDestination(f.g, west); !ok || dest != [2]int{0, 1} {
			t.Errorf("the west tank goes to %v (ordered %v), want the free cell (0,1)", dest, ok)
		}

		if f.g.HasOrder(middle) || f.g.HasOrder(blocker) {
			t.Error("a tank other than the end of the western chain was ordered")
		}
	})

	t.Run("the requester is a wall for the chain even when it could move", func(t *testing.T) {
		f := strip(4)
		blocker := armedTank(1, 50, 1, 1, f.mgr)
		requester := armedTank(1, 51, 2, 1, f.mgr)
		f.zones.addZone(requester, [][2]int{{1, 1}})
		f.zones.BeginTurn()

		if f.g.HasOrder(requester) || f.g.HasOrder(blocker) {
			t.Error("a unit was moved although the requester stands between it and the free cells")
		}
	})

	t.Run("the free cell may lie on the map edge", func(t *testing.T) {
		f := newClearFixture(3, 1, allLand{})
		builder := constructorUnit(1, 1, 2, 0, f.mgr)
		tank := armedTank(1, 50, 1, 0, f.mgr)
		f.zones.addZone(builder, [][2]int{{1, 0}})
		f.zones.BeginTurn()

		if dest, ok := moveDestination(f.g, tank); !ok || dest != [2]int{0, 0} {
			t.Errorf("the tank goes to %v (ordered %v), want the edge cell (0,0)", dest, ok)
		}
	})

	t.Run("never into the cell of another zone", func(t *testing.T) {
		f := strip(6)
		builder := constructorUnit(1, 1, 1, 1, f.mgr)
		other := engineerUnit(1, 2, 0, 0, f.mgr)
		tank := armedTank(1, 50, 2, 1, f.mgr)
		f.zones.addZone(builder, [][2]int{{2, 1}})
		f.zones.addZone(other, [][2]int{{3, 1}})
		f.zones.BeginTurn()

		if dest, ok := moveDestination(f.g, tank); !ok || dest != [2]int{4, 1} {
			t.Errorf("the tank goes to %v (ordered %v), want (4,1) past the other zone's (3,1)", dest, ok)
		}
	})
}

// TestClearZoneClaimsTheMovedUnit checks that the task claims a moved unit and leaves a
// unit that cannot take orders with its current owner.
func TestClearZoneClaimsTheMovedUnit(t *testing.T) {
	cases := []struct {
		name      string
		ready     bool
		wantMoved bool
	}{
		{"a ready unit of a weaker task is taken", true, true},
		{"a unit not ready for orders stays with its task", false, false},
	}

	for _, c := range cases {
		f := newClearFixture(20, 20, allLand{})
		manager := NewTaskManager()
		manager.AppendTask(f.zones)
		weak := &probeTask{TaskBase: newTaskBase(1, nil, taskPriorityAutoSurvey)}
		manager.AppendTask(weak)
		tank := armedTank(1, 50, 5, 5, f.mgr)
		manager.claimUnit(weak, tank)
		manager.readyForOrders = func(*game.UnitInfo) bool { return c.ready }

		f.zones.addZone(constructorUnit(1, 1, 1, 1, f.mgr), [][2]int{{5, 5}})
		f.zones.BeginTurn()

		wantOwner := Task(weak)
		if c.wantMoved {
			wantOwner = f.zones
		}

		if got := f.g.HasOrder(tank); got != c.wantMoved {
			t.Errorf("%s: moved = %v, want %v", c.name, got, c.wantMoved)
		}

		if manager.owner[tank] != wantOwner {
			t.Errorf("%s: owner = %v, want %v", c.name, manager.owner[tank], wantOwner)
		}
	}
}

// TestClearZoneDoesNotHoldAUnitItCouldNotMove checks that a refused move does not transfer
// ownership from another task.
func TestClearZoneDoesNotHoldAUnitItCouldNotMove(t *testing.T) {
	f := newClearFixture(20, 20, allLand{})
	manager := NewTaskManager()
	manager.AppendTask(f.zones)
	tank := armedTank(1, 50, 5, 5, f.mgr)
	tank.SetOrder(game.OrderDisable)

	f.zones.addZone(constructorUnit(1, 1, 1, 1, f.mgr), [][2]int{{5, 5}})
	f.zones.BeginTurn()

	if f.g.HasOrder(tank) {
		t.Fatal("setup: the disabled tank took a move order")
	}

	if manager.owner[tank] != nil {
		t.Errorf("the clearing task holds a tank it could not move: owner %v", manager.owner[tank])
	}
}

// TestStartBuildingClearsTheSiteOwnUnitsBlock checks that parked tanks blocking a footprint
// trigger a clearing zone, while an unavailable site or missing clearing task does not.
// Busy units stay in the map hash during the search, and all units are restored afterward.
func TestStartBuildingClearsTheSiteOwnUnitsBlock(t *testing.T) {
	spec := buildSpec{t: assets.PowerStn, large: true}

	// crowdedMap fills a side x side land map with the builder at (0,0) and a tank on every other cell.
	crowdedMap := func(side int) (*clearFixture, *game.UnitInfo, []*game.UnitInfo) {
		f := newClearFixture(side, side, allLand{})
		builder := constructorUnit(1, 1, 0, 0, f.mgr)
		builder.Hits = 24 // alive: only the explicit "never the builder" rule keeps it on the map
		var tanks []*game.UnitInfo
		id := 100

		for y := 0; y < side; y++ {
			for x := 0; x < side; x++ {
				if x == 0 && y == 0 {
					continue
				}

				tanks = append(tanks, armedTank(1, id, x, y, f.mgr))
				id++
			}
		}

		return f, builder, tanks
	}

	t.Run("a zone opens over the footprint and the builder waits", func(t *testing.T) {
		f, builder, tanks := crowdedMap(5)
		bm := f.manager()

		if bm.startBuilding(builder, spec) {
			t.Error("startBuilding committed the builder to a site the tanks still block")
		}

		if len(f.zones.zones) != 1 || f.zones.zones[0].requester != builder {
			t.Fatalf("zones = %+v, want one zone for the builder", f.zones.zones)
		}

		want := [][2]int{{0, 0}, {1, 0}, {0, 1}, {1, 1}}
		got := f.zones.zones[0].cells

		if len(got) != len(want) {
			t.Fatalf("zone cells = %v, want %v", got, want)
		}

		for i := range want {
			if got[i] != want[i] {
				t.Errorf("zone cell %d = %v, want %v (row order)", i, got[i], want[i])
			}
		}

		for _, tank := range tanks {
			if cellCount(f.g, tank, tank.GridX, tank.GridY) != 1 {
				t.Errorf("tank %d is hashed %d times at (%d,%d) after the search, want once", tank.ID, cellCount(f.g, tank, tank.GridX, tank.GridY), tank.GridX, tank.GridY)
			}
		}

		if cellCount(f.g, builder, 0, 0) != 1 {
			t.Errorf("the builder is hashed %d times after the search, want once", cellCount(f.g, builder, 0, 0))
		}
	})

	t.Run("a bare manager opens nothing", func(t *testing.T) {
		f, builder, tanks := crowdedMap(5)
		bm := f.manager()
		bm.clearer = nil

		if bm.startBuilding(builder, spec) {
			t.Error("startBuilding committed the builder to a blocked site")
		}

		if len(f.zones.zones) != 0 {
			t.Errorf("zones = %+v, want none", f.zones.zones)
		}

		for _, tank := range tanks {
			if cellCount(f.g, tank, tank.GridX, tank.GridY) != 1 {
				t.Errorf("tank %d left the cell hash", tank.ID)
			}
		}
	})

	t.Run("a pocket too small for any site opens nothing", func(t *testing.T) {
		f, builder, _ := crowdedMap(3)
		bm := f.manager()

		if bm.startBuilding(builder, spec) {
			t.Error("startBuilding committed the builder in a pocket with no escape")
		}

		if len(f.zones.zones) != 0 {
			t.Errorf("zones = %+v, want none: clearing the tanks would not give a site", f.zones.zones)
		}
	})

	t.Run("a busy tank is not lifted off the map", func(t *testing.T) {
		f, builder, tanks := crowdedMap(5)
		busy := tanks[0] // the tank at (1,0), inside the first footprint
		busy.Building = true
		bm := f.manager()
		bm.startBuilding(builder, spec)

		if len(f.zones.zones) != 1 || f.zones.zones[0].cells[0] != [2]int{0, 1} {
			t.Fatalf("zones = %+v, want one zone for the site at (0,1), below the busy tank", f.zones.zones)
		}

		for _, c := range f.zones.zones[0].cells {
			if c == [2]int{busy.GridX, busy.GridY} {
				t.Errorf("the zone covers the busy tank's cell %v", c)
			}
		}

		if cellCount(f.g, busy, busy.GridX, busy.GridY) != 1 {
			t.Error("the busy tank is not hashed once after the search")
		}
	})
}

// TestAIPlayerRegistersTheClearZoneTaskAfterTheManager checks that the player registers
// the clearing task immediately after the building manager and wires both together.
func TestAIPlayerRegistersTheClearZoneTaskAfterTheManager(t *testing.T) {
	g, mgr := newLandGame(20, 20)
	constructorUnit(1, 1, 10, 10, mgr)
	stationary(1, 2, assets.Research, 6, 6, mgr)

	a := New(testUnitFactory)
	g.AI = a
	g.BeginTeamTurn(1)

	var manager *taskManageBuildings
	managerAt := -1

	for i, task := range a.mgr.tasks {
		if m, ok := task.(*taskManageBuildings); ok {
			manager, managerAt = m, i
		}
	}

	if manager == nil {
		t.Fatal("the AI registered no building manager")
	}

	if manager.clearer == nil {
		t.Fatal("the building manager has no clearing task")
	}

	if managerAt+1 >= len(a.mgr.tasks) || a.mgr.tasks[managerAt+1] != Task(manager.clearer) {
		t.Errorf("the clearing task is not registered right after the manager (manager at %d)", managerAt)
	}
}
