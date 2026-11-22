// clearzone_ladder_test.go : testing unit for clearzone_ladder.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/world"
)

// spotSurface is land everywhere except the listed cells, which hold the surface given for them.
type spotSurface map[[2]int]uint8

// SurfaceAt returns the listed surface for a listed cell and land for any other.
func (s spotSurface) SurfaceAt(x, y int) uint8 {
	if surface, ok := s[[2]int{x, y}]; ok {
		return surface
	}

	return world.SurfaceLand
}

// storageBuilding deploys a standing 2x2 storage building of team 1 with room for capacity units.
func storageBuilding(f *clearFixture, id int, unitType assets.UnitType, gridX, gridY, capacity int) *game.UnitInfo {
	u := game.NewUnitInfo(unitType, 1, id, 0)
	u.Flags = game.FlagStationary | game.FlagBuilding | game.FlagSelectable
	values := &game.UnitValues{}
	values.Set(game.AttribStorage, capacity)
	values.Set(game.AttribHits, 20)
	u.Values = values
	u.InitState()
	f.mgr.Deploy(u, gridX, gridY)

	return u
}

// spottedEnemy deploys an armed tank of team 0 that team 1 has spotted.
func spottedEnemy(f *clearFixture, id, gridX, gridY int) *game.UnitInfo {
	e := armedTank(0, id, gridX, gridY, f.mgr)
	f.g.AddSpotted(1, e, gridX, gridY, true)

	return e
}

// busyTank deploys a tank of team 1 that is building, so it is a wall for every search.
func busyTank(f *clearFixture, id, gridX, gridY int) *game.UnitInfo {
	u := armedTank(1, id, gridX, gridY, f.mgr)
	u.Building = true

	return u
}

// ladderOwned returns a manager that holds the fixture's clearing task and a weaker task that
// owns the unit, with the unit ready or not ready for orders.
func ladderOwned(f *clearFixture, u *game.UnitInfo, ready bool) *TaskManager {
	manager := NewTaskManager()
	manager.AppendTask(f.zones)
	weak := &probeTask{TaskBase: newTaskBase(1, nil, taskPriorityAutoSurvey)}
	manager.AppendTask(weak)
	manager.claimUnit(weak, u)
	manager.readyForOrders = func(*game.UnitInfo) bool { return ready }

	return manager
}

// TestMoveAsideWaterFirst checks the amphibious rung: a blocker that can swim goes to the
// nearest open water outside every zone, even past a nearer free land cell, and only within
// waterSearchSteps. A land only unit, a way walled by units, coast, and a water cell in a
// zone give the plain land move.
func TestMoveAsideWaterFirst(t *testing.T) {
	cases := []struct {
		name      string
		width     int
		surface   spotSurface
		unitType  assets.UnitType
		startX    int
		wall      bool     // busy tanks on column x = 1
		extraZone [][2]int // zone cells beside the blocker's own
		want      [2]int
	}{
		{"a scout takes the water over a nearer land cell", 12, spotSurface{{0, 1}: world.SurfaceWater}, assets.Scout, 4, false, nil, [2]int{0, 1}},
		{"a tank never takes the water", 12, spotSurface{{0, 1}: world.SurfaceWater}, assets.Tank, 4, false, nil, [2]int{5, 1}},
		{"a way to the water walled by units gives the land move", 12, spotSurface{{0, 1}: world.SurfaceWater}, assets.Scout, 4, true, nil, [2]int{5, 1}},
		{"coast is not open water", 12, spotSurface{{0, 1}: world.SurfaceWater, {1, 1}: world.SurfaceCoast, {2, 1}: world.SurfaceCoast, {3, 1}: world.SurfaceCoast}, assets.Scout, 4, false, nil, [2]int{0, 1}},
		{"a water cell in a zone is passed over", 12, spotSurface{{0, 1}: world.SurfaceWater, {1, 1}: world.SurfaceWater}, assets.Scout, 4, false, [][2]int{{1, 1}}, [2]int{0, 1}},
		{"water ten steps away counts", 14, spotSurface{{0, 1}: world.SurfaceWater}, assets.Scout, 10, false, nil, [2]int{0, 1}},
		{"water eleven steps away is too far", 14, spotSurface{{0, 1}: world.SurfaceWater}, assets.Scout, 11, false, nil, [2]int{12, 1}},
	}

	for _, c := range cases {
		f := newClearFixture(c.width, 3, c.surface)
		blocker := armedUnit(1, 50, c.unitType, game.FlagMobileLandUnit|game.FlagSelectable, c.startX, 1, f.mgr)

		if c.wall {
			for y := 0; y < 3; y++ {
				busyTank(f, 60+y, 1, y)
			}
		}

		cells := append([][2]int{{c.startX, 1}}, c.extraZone...)
		f.zones.addZone(constructorUnit(1, 1, c.width-1, 0, f.mgr), cells)
		f.zones.BeginTurn()

		if dest, ok := moveDestination(f.g, blocker); !ok || dest != c.want {
			t.Errorf("%s: the %s goes to %v (ordered %v), want %v", c.name, c.unitType.Tag(), dest, ok, c.want)
		}
	}
}

// TestMoveAsideWaterIsClaimed checks that the water move holds the unit through the ledger, and
// that a unit the ledger will not give up stays where it is.
func TestMoveAsideWaterIsClaimed(t *testing.T) {
	for _, ready := range []bool{true, false} {
		f := newClearFixture(12, 3, spotSurface{{0, 1}: world.SurfaceWater})
		scout := armedUnit(1, 50, assets.Scout, game.FlagMobileLandUnit|game.FlagSelectable, 4, 1, f.mgr)
		manager := ladderOwned(f, scout, ready)

		f.zones.addZone(constructorUnit(1, 1, 11, 0, f.mgr), [][2]int{{4, 1}})
		f.zones.BeginTurn()

		if got := f.g.HasOrder(scout); got != ready {
			t.Errorf("ready=%v: the scout has an order = %v, want %v", ready, got, ready)
		}

		if got := manager.owner[scout] == Task(f.zones); got != ready {
			t.Errorf("ready=%v: the clearing task holds the scout = %v, want %v", ready, got, ready)
		}
	}
}

// pocket builds the trapped base: a 12 x 6 map with land on x 1..4, y 1..2, a Depot with room
// for two at (1,1) and a requester off the land. The free land cells are (3,1), (4,1), (3,2)
// and (4,2); the caller fills them.
func pocket() (*clearFixture, *game.UnitInfo) {
	f := newClearFixture(12, 6, landPatch{startX: 1, startY: 1, endX: 5, endY: 3})
	depot := storageBuilding(f, 20, assets.Depot, 1, 1, 2)

	return f, depot
}

// TestStoreBlockerNextToTheBuilding checks the board at once: a tank with no free cell beside
// a Depot with room is loaded the same turn and stamped with the turn, and a farther Depot is
// left alone.
func TestStoreBlockerNextToTheBuilding(t *testing.T) {
	f, depot := pocket()
	far := storageBuilding(f, 21, assets.Depot, 9, 4, 2)
	tank := armedTank(1, 50, 3, 1, f.mgr)
	busyTank(f, 60, 4, 1)
	busyTank(f, 61, 3, 2)
	busyTank(f, 62, 4, 2)
	f.zones.addZone(constructorUnit(1, 1, 10, 0, f.mgr), [][2]int{{3, 1}})
	f.zones.BeginTurn()

	if len(depot.Carried) != 1 || depot.Carried[0] != tank || tank.Parent != depot {
		t.Fatalf("the tank is not in the Depot: carried %d, parent %v", len(depot.Carried), tank.Parent)
	}

	if len(far.Carried) != 0 {
		t.Error("the farther Depot took the tank")
	}

	if at, ok := f.zones.storedAt[tank]; !ok || at != f.g.State.TurnCounter {
		t.Errorf("stored at turn %d (set %v), want the current turn %d", at, ok, f.g.State.TurnCounter)
	}
}

// TestStoreBlockerDrivesToTheBuilding checks the drive: a tank that is not next to the Depot is
// ordered to the load cell and kept in storing, and boards at the next turn once it stands there.
func TestStoreBlockerDrivesToTheBuilding(t *testing.T) {
	f, depot := pocket()
	tank := armedTank(1, 50, 4, 1, f.mgr)
	busyTank(f, 60, 3, 2)
	busyTank(f, 61, 4, 2)
	f.zones.addZone(constructorUnit(1, 1, 10, 0, f.mgr), [][2]int{{3, 1}, {4, 1}})
	f.zones.BeginTurn()

	if dest, ok := moveDestination(f.g, tank); !ok || dest != [2]int{3, 1} {
		t.Fatalf("the tank goes to %v (ordered %v), want the load cell (3,1)", dest, ok)
	}

	if f.zones.storing[tank] != depot || len(depot.Carried) != 0 {
		t.Fatalf("storing = %v, carried %d, want the tank bound for the Depot and not yet aboard", f.zones.storing[tank], len(depot.Carried))
	}

	f.mgr.Move(tank, 3, 1) // the move is done
	f.zones.BeginTurn()

	if tank.Parent != depot || len(f.zones.storing) != 0 {
		t.Errorf("after arrival: parent %v, storing %d, want it aboard and the plan done", tank.Parent, len(f.zones.storing))
	}

	if _, ok := f.zones.storedAt[tank]; !ok {
		t.Error("the stored turn was not recorded")
	}
}

// TestFinishStoresDropsADeadPlan checks the plans that end without a load: a unit that stops
// short without an order, one that dies, and a building that is destroyed.
func TestFinishStoresDropsADeadPlan(t *testing.T) {
	cases := []struct {
		name   string
		breaks func(f *clearFixture, depot, tank *game.UnitInfo)
	}{
		{"the unit stops short", func(f *clearFixture, depot, tank *game.UnitInfo) { f.g.CancelOrder(tank) }},
		{"the unit dies", func(f *clearFixture, depot, tank *game.UnitInfo) { tank.Hits = 0 }},
		{"the building is destroyed", func(f *clearFixture, depot, tank *game.UnitInfo) { depot.Hits = 0 }},
	}

	for _, c := range cases {
		f, depot := pocket()
		tank := armedTank(1, 50, 4, 1, f.mgr)
		busyTank(f, 60, 3, 2)
		busyTank(f, 61, 4, 2)
		f.zones.addZone(constructorUnit(1, 1, 10, 0, f.mgr), [][2]int{{3, 1}, {4, 1}})
		f.zones.BeginTurn()

		if f.zones.storing[tank] != depot {
			t.Fatalf("%s: setup: the tank is not on its way", c.name)
		}

		c.breaks(f, depot, tank)
		f.zones.finishStores()

		if len(f.zones.storing) != 0 {
			t.Errorf("%s: the plan is still kept", c.name)
		}
	}

	// A unit that still has its order keeps its plan.
	f, _ := pocket()
	tank := armedTank(1, 50, 4, 1, f.mgr)
	busyTank(f, 60, 3, 2)
	busyTank(f, 61, 4, 2)
	f.zones.addZone(constructorUnit(1, 1, 10, 0, f.mgr), [][2]int{{3, 1}, {4, 1}})
	f.zones.BeginTurn()
	f.zones.finishStores()

	if f.zones.storing[tank] == nil {
		t.Error("a unit still on its way lost its plan")
	}
}

// TestStoreBlockerPicksTheClassStorage checks the class pick: a tank goes into the Depot and
// an infantry squad into the Barracks, although both buildings have room.
func TestStoreBlockerPicksTheClassStorage(t *testing.T) {
	newBase := func() (*clearFixture, *game.UnitInfo, *game.UnitInfo) {
		f := newClearFixture(12, 6, landPatch{startX: 1, startY: 1, endX: 8, endY: 3})
		depot := storageBuilding(f, 20, assets.Depot, 1, 1, 2)
		barracks := storageBuilding(f, 21, assets.Barracks, 6, 1, 2)
		zone := [][2]int{{3, 1}, {4, 1}, {5, 1}, {3, 2}, {4, 2}, {5, 2}}
		f.zones.addZone(constructorUnit(1, 1, 10, 0, f.mgr), zone)

		return f, depot, barracks
	}

	f, depot, barracks := newBase()
	tank := armedTank(1, 50, 3, 1, f.mgr)
	f.zones.BeginTurn()

	if tank.Parent != depot || len(barracks.Carried) != 0 {
		t.Errorf("the tank is in %v, want the Depot", tank.Parent)
	}

	g, depot, barracks := newBase()
	squad := armedUnit(1, 51, assets.Infantry, game.FlagMobileLandUnit|game.FlagSelectable, 3, 1, g.mgr)
	g.zones.BeginTurn()

	if g.zones.storing[squad] != barracks || len(depot.Carried) != 0 {
		t.Errorf("the squad is bound for %v, want the Barracks", g.zones.storing[squad])
	}

	if dest, ok := moveDestination(g.g, squad); !ok || dest != [2]int{5, 1} {
		t.Errorf("the squad goes to %v (ordered %v), want the cell (5,1) beside the Barracks", dest, ok)
	}
}

// TestStoreBlockerSkipsUnitsItMayNotMove checks that a requester of another zone and a unit
// the ledger holds are not stored.
func TestStoreBlockerSkipsUnitsItMayNotMove(t *testing.T) {
	cases := []struct {
		name  string
		setup func(f *clearFixture, tank *game.UnitInfo)
	}{
		{"a requester of another zone", func(f *clearFixture, tank *game.UnitInfo) {
			f.zones.addZone(tank, [][2]int{{9, 4}})
		}},
		{"a unit held by another task", func(f *clearFixture, tank *game.UnitInfo) {
			ladderOwned(f, tank, false)
		}},
		{"a builder", func(f *clearFixture, tank *game.UnitInfo) {
			tank.Flags |= game.FlagConstructorUnit
		}},
	}

	for _, c := range cases {
		f, depot := pocket()
		tank := armedTank(1, 50, 3, 1, f.mgr)
		busyTank(f, 60, 4, 1)
		busyTank(f, 61, 3, 2)
		busyTank(f, 62, 4, 2)
		f.zones.addZone(constructorUnit(1, 1, 10, 0, f.mgr), [][2]int{{3, 1}})
		c.setup(f, tank)
		f.zones.BeginTurn()

		if tank.Parent != nil || len(depot.Carried) != 0 || f.zones.storing[tank] != nil {
			t.Errorf("%s: the tank was stored", c.name)
		}
	}
}

// TestStoreBlockerWantsStorageWhenThereIsNoRoom checks the want: a full Depot, no building of
// the class and a squad with only a Depot each record the type to build, and a later turn that
// finds no trapped unit starts the record afresh.
func TestStoreBlockerWantsStorageWhenThereIsNoRoom(t *testing.T) {
	cases := []struct {
		name  string
		setup func(f *clearFixture, depot *game.UnitInfo) *game.UnitInfo
		want  assets.UnitType
	}{
		{"a full Depot", func(f *clearFixture, depot *game.UnitInfo) *game.UnitInfo {
			depot.Carried = []*game.UnitInfo{armedTank(1, 70, 8, 4, f.mgr)}
			depot.Values.Set(game.AttribStorage, 1)

			return armedTank(1, 50, 3, 1, f.mgr)
		}, assets.Depot},
		{"no Depot at all", func(f *clearFixture, depot *game.UnitInfo) *game.UnitInfo {
			depot.Type = assets.Research // the same footprint, but no storage

			return armedTank(1, 50, 3, 1, f.mgr)
		}, assets.Depot},
		{"a squad with no Barracks", func(f *clearFixture, depot *game.UnitInfo) *game.UnitInfo {
			return armedUnit(1, 50, assets.Infantry, game.FlagMobileLandUnit|game.FlagSelectable, 3, 1, f.mgr)
		}, assets.Barracks},
	}

	for _, c := range cases {
		f, depot := pocket()
		blocker := c.setup(f, depot)
		busyTank(f, 60, 4, 1)
		busyTank(f, 61, 3, 2)
		busyTank(f, 62, 4, 2)
		f.zones.addZone(constructorUnit(1, 1, 10, 0, f.mgr), [][2]int{{3, 1}})
		f.zones.BeginTurn()

		if got, ok := f.zones.wantedStorage(); !ok || got != c.want {
			t.Errorf("%s: wanted storage = %v (%v), want %v", c.name, got, ok, c.want)
		}

		if blocker.Parent != nil || f.zones.storing[blocker] != nil {
			t.Errorf("%s: the blocker was stored or sent", c.name)
		}

		// The next pass has no trapped unit: the record starts afresh.
		f.zones.zones = nil
		f.zones.BeginTurn()

		if got, ok := f.zones.wantedStorage(); ok {
			t.Errorf("%s: a quiet turn still wants %v", c.name, got)
		}
	}
}

// TestWantedStorageOrder checks the order of the wants: Depot, then Barracks; a Dock is never
// offered.
func TestWantedStorageOrder(t *testing.T) {
	cases := []struct {
		wants  []assets.UnitType
		want   assets.UnitType
		wanted bool
	}{
		{nil, 0, false},
		{[]assets.UnitType{assets.Dock}, 0, false},
		{[]assets.UnitType{assets.Dock, assets.Barracks}, assets.Barracks, true},
		{[]assets.UnitType{assets.Dock, assets.Barracks, assets.Depot}, assets.Depot, true},
	}

	for _, c := range cases {
		task := newTaskClearZone(1, &aiContext{})

		for _, w := range c.wants {
			task.wantStorage[w] = true
		}

		if got, ok := task.wantedStorage(); got != c.want || ok != c.wanted {
			t.Errorf("wants %v: got %v (%v), want %v (%v)", c.wants, got, ok, c.want, c.wanted)
		}
	}
}

// depotBase builds a 20 x 14 land map with a Depot with room for four at (5,5).
func depotBase() (*clearFixture, *game.UnitInfo) {
	f := newClearFixture(20, 14, allLand{})
	depot := storageBuilding(f, 20, assets.Depot, 5, 5, 4)

	return f, depot
}

// storeTank boards a new tank into the Depot and stamps it as stored turnsAgo turns ago.
func storeTank(t *testing.T, f *clearFixture, depot *game.UnitInfo, id, turnsAgo int) *game.UnitInfo {
	t.Helper()

	tank := armedTank(1, id, 4, 5, f.mgr)

	if !f.g.LoadUnit(depot, tank) {
		t.Fatal("setup: the tank could not board the Depot")
	}

	f.zones.storedAt[tank] = f.g.State.TurnCounter - turnsAgo

	return tank
}

// TestRoutineUnload checks the routine pass: with no zone open a unit that rested storeRestTurns
// turns comes out, one that did not, or any unit while a zone is open, stays, and a unit never
// stamped counts as rested.
func TestRoutineUnload(t *testing.T) {
	cases := []struct {
		name     string
		turnsAgo int
		stamped  bool
		zone     bool
		wantOut  bool
	}{
		{"rested", storeRestTurns, true, false, true},
		{"one turn short", storeRestTurns - 1, true, false, false},
		{"a zone is open", storeRestTurns, true, true, false},
		{"never stamped", 0, false, false, true},
	}

	for _, c := range cases {
		f, depot := depotBase()
		tank := storeTank(t, f, depot, 50, c.turnsAgo)

		if !c.stamped {
			delete(f.zones.storedAt, tank)
		}

		if c.zone {
			armedTank(1, 70, 12, 12, f.mgr) // idle, so the zone stays open this turn
			f.zones.addZone(constructorUnit(1, 1, 15, 12, f.mgr), [][2]int{{12, 12}})
		}

		f.zones.BeginTurn()

		if out := tank.Parent == nil; out != c.wantOut {
			t.Errorf("%s: unloaded = %v, want %v", c.name, out, c.wantOut)
		}
	}
}

// TestRoutineUnloadTakesOneUnitPerTurn checks the pace: two rested units, one out per turn.
func TestRoutineUnloadTakesOneUnitPerTurn(t *testing.T) {
	f, depot := depotBase()
	storeTank(t, f, depot, 50, storeRestTurns)
	storeTank(t, f, depot, 51, storeRestTurns)

	f.zones.BeginTurn()

	if len(depot.Carried) != 1 {
		t.Fatalf("carried after one turn = %d, want 1", len(depot.Carried))
	}

	f.zones.BeginTurn()

	if len(depot.Carried) != 0 {
		t.Errorf("carried after two turns = %d, want 0", len(depot.Carried))
	}
}

// TestUnloadThreat checks the urgent pass: a spotted enemy within the engage radius of the
// Depot sends the armed stored unit out at rest zero and leaves an unarmed unit, and an enemy
// past the radius sends nobody.
func TestUnloadThreat(t *testing.T) {
	cases := []struct {
		name       string
		enemyX     int
		wantArmed  bool
		wantUnarmd bool
	}{
		{"an enemy on the ring", 5 + guardEngageRadius, true, false},
		{"an enemy past the ring", 5 + guardEngageRadius + 1, false, false},
	}

	for _, c := range cases {
		f, depot := depotBase()
		armed := storeTank(t, f, depot, 50, 0)
		scout := testUnitFactory(1, assets.Scout, game.FlagMobileLandUnit|game.FlagSelectable)
		f.mgr.Deploy(scout, 4, 6)

		if !f.g.LoadUnit(depot, scout) {
			t.Fatal("setup: the scout could not board the Depot")
		}

		f.zones.storedAt[scout] = f.g.State.TurnCounter
		spottedEnemy(f, 90, c.enemyX, 5)
		f.zones.BeginTurn()

		if out := armed.Parent == nil; out != c.wantArmed {
			t.Errorf("%s: the armed tank is out = %v, want %v", c.name, out, c.wantArmed)
		}

		if out := scout.Parent == nil; out != c.wantUnarmd {
			t.Errorf("%s: the unarmed scout is out = %v, want %v", c.name, out, c.wantUnarmd)
		}
	}
}

// TestUnloadCellAvoidsTheZones checks the unload cell: with a zone open and a free cell
// outside it, the unit lands outside; with every free cell around the Depot in a zone, an
// urgent unload still takes a zone cell.
func TestUnloadCellAvoidsTheZones(t *testing.T) {
	t.Run("a free cell outside the zones is taken", func(t *testing.T) {
		f, depot := depotBase()
		tank := storeTank(t, f, depot, 50, 0)
		spottedEnemy(f, 90, 9, 5)
		armedTank(1, 70, 6, 4, f.mgr) // idle blocker, so the zone stays open
		f.zones.addZone(constructorUnit(1, 1, 15, 12, f.mgr), [][2]int{{4, 4}, {5, 4}, {6, 4}})
		f.zones.BeginTurn()

		if tank.Parent != nil || tank.GridX != 4 || tank.GridY != 5 {
			t.Errorf("the tank is at (%d,%d) (aboard %v), want the first cell outside the zone (4,5)", tank.GridX, tank.GridY, tank.Parent != nil)
		}
	})

	t.Run("an urgent unload may use a zone cell when no other is free", func(t *testing.T) {
		f, depot := depotBase()
		tank := storeTank(t, f, depot, 50, 0)
		spottedEnemy(f, 90, 9, 5)
		armedTank(1, 70, 6, 4, f.mgr) // idle blocker, so the zones stay open
		ring := [][2]int{{4, 4}, {5, 4}, {6, 4}, {4, 5}, {4, 6}}
		f.zones.addZone(constructorUnit(1, 1, 15, 12, f.mgr), ring)
		f.zones.BeginTurn()

		if tank.Parent != nil {
			t.Fatal("the tank stayed aboard although an urgent unload may use a zone cell")
		}

		inRing := false

		for _, c := range ring {
			inRing = inRing || c == [2]int{tank.GridX, tank.GridY}
		}

		if !inRing {
			t.Errorf("the tank is at (%d,%d), want one of the ring cells %v", tank.GridX, tank.GridY, ring)
		}
	})

	t.Run("a routine unload never takes a zone cell", func(t *testing.T) {
		f, depot := depotBase()
		tank := storeTank(t, f, depot, 50, storeRestTurns)

		if f.zones.unloadStored(func(*game.UnitInfo) bool { return true }, false) != tank {
			t.Fatal("the rested tank did not come out")
		}

		f2, depot2 := depotBase()
		tank2 := storeTank(t, f2, depot2, 50, storeRestTurns)
		f2.zones.addZone(constructorUnit(1, 1, 15, 12, f2.mgr), [][2]int{{4, 4}, {5, 4}, {6, 4}, {4, 5}, {4, 6}})

		if f2.zones.unloadStored(func(*game.UnitInfo) bool { return true }, false) != nil || tank2.Parent == nil {
			t.Error("a routine unload took a zone cell")
		}
	})
}

// TestObtainTakesAStoredUnit checks the request hook: a request at the economy tier takes a
// rested stored tank and is met, but not one that rested too little; a more urgent request
// takes it at once; a unit the asking task rules out comes out but leaves the request open.
func TestObtainTakesAStoredUnit(t *testing.T) {
	tankNeed := unitNeed{unit: assets.Tank, factory: assets.LandPlt, flags: flagLandUnit}
	refuse := tankNeed
	refuse.accept = func(*game.UnitInfo) bool { return false }

	cases := []struct {
		name     string
		priority uint16
		turnsAgo int
		need     unitNeed
		wantOut  bool
		wantOpen bool
	}{
		{"the economy tier takes a rested unit", taskPriorityManageBuildings, storeRestTurns, tankNeed, true, false},
		{"the economy tier leaves a unit that rests", taskPriorityManageBuildings, 1, tankNeed, false, true},
		{"a more urgent request takes a unit that rests", taskPriorityDefenseReserve, 1, tankNeed, true, false},
		{"just above the economy tier is urgent", taskPriorityManageBuildings - 1, 0, tankNeed, true, false},
		{"a unit the asker rules out comes out but the request stays open", taskPriorityDefenseReserve, 0, refuse, true, true},
	}

	for _, c := range cases {
		f, depot := depotBase()
		tank := storeTank(t, f, depot, 50, c.turnsAgo)
		manager := NewTaskManager()
		manager.AppendTask(f.zones)
		asker := &probeTask{TaskBase: newTaskBase(1, nil, c.priority)}
		manager.AppendTask(asker)
		asker.requestUnit(f.ctx, c.need)

		open := 0

		for _, task := range manager.tasks {
			if request, ok := task.(*taskObtainUnits); ok && request.IsNeeded() {
				open++
			}
		}

		if out := tank.Parent == nil; out != c.wantOut {
			t.Errorf("%s: the stored tank is out = %v, want %v", c.name, out, c.wantOut)
		}

		if (open == 1) != c.wantOpen {
			t.Errorf("%s: open requests = %d, want open = %v", c.name, open, c.wantOpen)
		}
	}
}

// TestBuildStorageWhenTheClearTaskWantsIt checks the manager rung: with a want for a Depot a
// constructor takes a Depot site before the strategic backbone, a second constructor does not
// start a second one, and without a want the backbone runs.
func TestBuildStorageWhenTheClearTaskWantsIt(t *testing.T) {
	econ := teamEconomy{miningStations: 1, netRaw: 1, netFuel: 1, netGold: 1}

	f := newClearFixture(30, 30, allLand{})
	bm := f.manager()
	first := constructorUnit(1, 1, 6, 6, f.mgr)
	second := constructorUnit(1, 2, 20, 20, f.mgr)
	f.zones.wantStorage[assets.Depot] = true

	if !bm.tryBuildLarge(econ, first) || !bm.handling(assets.Depot) {
		t.Fatalf("the constructor did not start a Depot (handling %v)", bm.handling(assets.Depot))
	}

	if !bm.tryBuildLarge(econ, second) || bm.handling(assets.LandPlt) == false {
		t.Errorf("the second constructor took the Depot again instead of the backbone (heavy plant handled %v)", bm.handling(assets.LandPlt))
	}

	g := newClearFixture(30, 30, allLand{})
	plain := g.manager()
	builder := constructorUnit(1, 1, 6, 6, g.mgr)

	if !plain.tryBuildLarge(econ, builder) || plain.handling(assets.Depot) || !plain.handling(assets.LandPlt) {
		t.Errorf("without a want the backbone must start: Depot %v, heavy plant %v", plain.handling(assets.Depot), plain.handling(assets.LandPlt))
	}
}
