// task_obtainunits_test.go : testing unit for task_obtainunits.go
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

// surveyorNeed is the want most tests use: a Surveyor from a Light Vehicle Plant.
var surveyorNeed = unitNeed{unit: assets.Surveyor, factory: assets.LightPlt, flags: flagLandUnit}

// obtainFixture is a 16x16 land map for team 1 with one idle Light Vehicle Plant, a task
// manager and a requesting probe task registered at the survey tier.
type obtainFixture struct {
	g      *sim.Game
	units  *units.Manager
	plant  *game.UnitInfo
	tasks  *TaskManager
	asker  *probeTask
	ctx    *aiContext
	parent Task
}

// newObtainFixture builds the fixture.
func newObtainFixture(t *testing.T) *obtainFixture {
	t.Helper()

	const w, h = 16, 16
	unitsMgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: unitsMgr.Cells}
	p := &game.StartParams{}
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), unitsMgr, grid, w, h)

	f := &obtainFixture{g: g, units: unitsMgr, tasks: NewTaskManager()}
	f.plant = stationary(1, 10, assets.LightPlt, 4, 4, unitsMgr)
	f.ctx = &aiContext{Game: g, MakeUnit: testUnitFactory}
	f.asker = &probeTask{TaskBase: newTaskBase(1, nil, taskPriorityAutoSurvey)}
	f.tasks.AppendTask(f.asker)
	f.tasks.ExecuteReminders()
	f.parent = f.asker

	return f
}

// liveSurveyor deploys a Surveyor with hits, so only the rule under test can rule it out.
func liveSurveyor(team, gx, gy int, mgr *units.Manager) *game.UnitInfo {
	u := surveyorUnit(team, 1, gx, gy, mgr)
	u.Values.Set(game.AttribHits, 10)
	u.Hits = 10

	return u
}

// requests returns the open unit requests of the fixture's manager.
func (f *obtainFixture) requests() []*taskObtainUnits {
	var out []*taskObtainUnits
	for _, task := range f.tasks.tasks {
		if request, ok := task.(*taskObtainUnits); ok {
			out = append(out, request)
		}
	}

	return out
}

// orders counts the build orders for a unit type.
func (f *obtainFixture) orders(unitType assets.UnitType) int {
	return unitsInProduction(f.g, 1, unitType)
}

// landBuild finishes the plant's build at once: the order goes and the product is deployed.
func (f *obtainFixture) landBuild(t *testing.T) *game.UnitInfo {
	t.Helper()

	order := findOrder(f.g, assets.Surveyor)
	if order == nil {
		t.Fatal("fixture: no Surveyor order to land")
	}
	product := order.Product
	f.g.CancelBuild(f.plant)
	f.units.Deploy(product, 8, 8)

	return product
}

// TestObtainUnitsIsAChildWithItsParentsPriority checks the request's place in the tree:
// a child of the asker, typed as a unit request, as urgent as its parent.
func TestObtainUnitsIsAChildWithItsParentsPriority(t *testing.T) {
	f := newObtainFixture(t)
	f.asker.requestUnit(f.ctx, surveyorNeed)

	requests := f.requests()
	if len(requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(requests))
	}
	request := requests[0]
	if request.Type() != TaskTypeObtainUnits {
		t.Errorf("Type() = %d, want TaskTypeObtainUnits", request.Type())
	}
	if request.parent != Task(f.asker) {
		t.Error("the request is not a child of the task that asked")
	}
	if request.Priority() != f.asker.Priority() || request.Team() != 1 {
		t.Errorf("request priority %#x team %d, want the parent's %#x and team 1", request.Priority(), request.Team(), f.asker.Priority())
	}
	if !request.IsNeeded() {
		t.Error("an open request is not needed")
	}
}

// TestObtainUnitsBuildsAMissingUnitAtOnce checks the first pass: no unit anywhere, an idle
// plant, so one build order is placed inside the asking call itself.
func TestObtainUnitsBuildsAMissingUnitAtOnce(t *testing.T) {
	f := newObtainFixture(t)
	f.asker.requestUnit(f.ctx, surveyorNeed)

	if n := f.orders(assets.Surveyor); n != 1 {
		t.Fatalf("Surveyor orders right after the request = %d, want 1", n)
	}
	order := findOrder(f.g, assets.Surveyor)
	if order.Builder != f.plant {
		t.Error("the Surveyor was not ordered at the team's Light Vehicle Plant")
	}
}

// TestObtainUnitsWaitsWhileItsUnitIsBuilt checks the later turns while the order is in
// flight: no second order, the request stays open.
func TestObtainUnitsWaitsWhileItsUnitIsBuilt(t *testing.T) {
	f := newObtainFixture(t)
	f.asker.requestUnit(f.ctx, surveyorNeed)

	for turn := 0; turn < 3; turn++ {
		f.tasks.BeginTurn(1)
		f.tasks.ExecuteReminders()
	}

	if n := f.orders(assets.Surveyor); n != 1 {
		t.Errorf("Surveyor orders after three turns = %d, want 1", n)
	}
	if len(f.requests()) != 1 {
		t.Error("the request closed while its unit was still being built")
	}
}

// TestObtainUnitsCompletesWhenItsUnitLands checks the hand-off: once the unit it had built
// stands on the map, the request closes at its next turn, leaves the list, and gives the
// unit no order (the hand-off is a notice only). It closes even when another task drove
// the new unit off first.
func TestObtainUnitsCompletesWhenItsUnitLands(t *testing.T) {
	for _, driven := range []bool{false, true} {
		f := newObtainFixture(t)
		f.asker.requestUnit(f.ctx, surveyorNeed)
		product := f.landBuild(t)
		if driven && !f.g.OrderMove(product, 12, 12) {
			t.Fatal("fixture: could not drive the new Surveyor")
		}
		request := f.requests()[0]

		f.tasks.BeginTurn(1)
		f.tasks.ExecuteReminders()

		if request.IsNeeded() {
			t.Errorf("driven=%v: the request is still open after its unit landed", driven)
		}
		f.tasks.BeginTurn(1) // the prune
		if len(f.requests()) != 0 {
			t.Errorf("driven=%v: the closed request is still in the list", driven)
		}
		if n := f.orders(assets.Surveyor); n != 0 {
			t.Errorf("driven=%v: Surveyor orders = %d, want 0 (no second build)", driven, n)
		}
		if !driven && f.g.HasOrder(product) {
			t.Error("the hand-off gave the new unit an order; it is a notice only")
		}
	}
}

// TestObtainUnitsRetriesWhenItsOrderIsLost checks a cancelled build: the request orders
// a new one at its next turn.
func TestObtainUnitsRetriesWhenItsOrderIsLost(t *testing.T) {
	f := newObtainFixture(t)
	f.asker.requestUnit(f.ctx, surveyorNeed)
	f.g.CancelBuild(f.plant)

	f.tasks.BeginTurn(1)
	f.tasks.ExecuteReminders()

	if n := f.orders(assets.Surveyor); n != 1 {
		t.Errorf("Surveyor orders after the lost order = %d, want 1 (a retry)", n)
	}
	if len(f.requests()) != 1 {
		t.Error("the request closed although its unit was never built")
	}
}

// TestObtainUnitsTakesAFreeUnitFirst checks the find: a free unit of the type closes the
// request at once, with no build.
func TestObtainUnitsTakesAFreeUnitFirst(t *testing.T) {
	f := newObtainFixture(t)
	liveSurveyor(1, 6, 6, f.units)

	f.asker.requestUnit(f.ctx, surveyorNeed)

	if n := f.orders(assets.Surveyor); n != 0 {
		t.Errorf("Surveyor orders = %d, want 0 (a free one was on the map)", n)
	}
	if len(f.requests()) != 0 {
		t.Error("a request met at once was left in the list")
	}
}

// TestObtainUnitsSkipsUnitsThatDoNotCount checks the candidate rules: a unit of another
// team, a busy unit, one the need's own test rejects, and one held by a more urgent task
// are passed over, so the request builds a new one.
func TestObtainUnitsSkipsUnitsThatDoNotCount(t *testing.T) {
	cases := []struct {
		name  string
		setup func(f *obtainFixture) unitNeed
	}{
		{"another team's unit", func(f *obtainFixture) unitNeed {
			liveSurveyor(2, 6, 6, f.units)
			return surveyorNeed
		}},
		{"a unit with an order", func(f *obtainFixture) unitNeed {
			u := liveSurveyor(1, 6, 6, f.units)
			if !f.g.OrderMove(u, 12, 12) {
				t.Fatal("fixture: could not order the Surveyor")
			}
			return surveyorNeed
		}},
		{"a unit the need rejects", func(f *obtainFixture) unitNeed {
			liveSurveyor(1, 6, 6, f.units)
			need := surveyorNeed
			need.accept = func(*game.UnitInfo) bool { return false }
			return need
		}},
		{"a unit a more urgent task holds", func(f *obtainFixture) unitNeed {
			u := liveSurveyor(1, 6, 6, f.units)
			urgent := &probeTask{TaskBase: newTaskBase(1, nil, taskPriorityEmergency)}
			f.tasks.AppendTask(urgent)
			if !urgent.claim(u) {
				t.Fatal("fixture: the urgent task could not claim the Surveyor")
			}
			return surveyorNeed
		}},
	}
	for _, c := range cases {
		f := newObtainFixture(t)
		need := c.setup(f)

		f.asker.requestUnit(f.ctx, need)

		if n := f.orders(assets.Surveyor); n != 1 {
			t.Errorf("%s: Surveyor orders = %d, want 1 (the unit does not count, so one is built)", c.name, n)
		}
	}
}

// TestObtainUnitsWaitsForAnotherBuildOfTheType checks that a unit of the type already in
// production elsewhere stops a second order.
func TestObtainUnitsWaitsForAnotherBuildOfTheType(t *testing.T) {
	f := newObtainFixture(t)
	if produceUnit(f.ctx, f.plant, assets.Surveyor, flagLandUnit) == nil {
		t.Fatal("fixture: could not start the first Surveyor")
	}
	second := stationary(1, 11, assets.LightPlt, 10, 10, f.units) // a second idle plant

	f.asker.requestUnit(f.ctx, surveyorNeed)

	if n := f.orders(assets.Surveyor); n != 1 {
		t.Errorf("Surveyor orders = %d, want 1 (one was already on the way)", n)
	}
	if f.g.BuildOrderFor(second) != nil {
		t.Error("the second plant took an order for a unit already being built")
	}
	if len(f.requests()) != 1 {
		t.Error("the request should stay open while it waits")
	}
}

// TestObtainUnitsWaitsWithoutAPlant checks a team with no plant of the type: no order, the
// request stays open for a later turn.
func TestObtainUnitsWaitsWithoutAPlant(t *testing.T) {
	f := newObtainFixture(t)
	need := unitNeed{unit: assets.Constrct, factory: assets.LandPlt, flags: flagLandUnit}

	f.asker.requestUnit(f.ctx, need)

	if n := f.orders(assets.Constrct); n != 0 {
		t.Errorf("Constructor orders = %d, want 0 (no Heavy Vehicle Plant)", n)
	}
	if len(f.requests()) != 1 {
		t.Error("the request closed without a unit")
	}
}

// TestObtainUnitsGoesWithItsParent checks the tree: a removed parent takes its open
// request with it, and the request plans nothing more.
func TestObtainUnitsGoesWithItsParent(t *testing.T) {
	f := newObtainFixture(t)
	f.asker.requestUnit(f.ctx, surveyorNeed)
	request := f.requests()[0]
	f.g.CancelBuild(f.plant) // a turn of work would place a new order

	f.asker.RemoveSelf()
	if request.IsNeeded() {
		t.Error("the request is still needed after its parent was removed")
	}
	f.tasks.BeginTurn(1)
	f.tasks.ExecuteReminders()

	if len(f.requests()) != 0 {
		t.Error("the request outlived its parent")
	}
	if n := f.orders(assets.Surveyor); n != 0 {
		t.Errorf("Surveyor orders = %d, want 0 (an orphan request must not build)", n)
	}
}

// TestRequestUnitDedupesPerParentAndType checks that one parent keeps one open request
// per type, while a second type or a second parent gets its own.
func TestRequestUnitDedupesPerParentAndType(t *testing.T) {
	f := newObtainFixture(t)
	f.asker.requestUnit(f.ctx, surveyorNeed)
	f.asker.requestUnit(f.ctx, surveyorNeed)
	if n := len(f.requests()); n != 1 {
		t.Fatalf("requests after asking twice = %d, want 1", n)
	}
	if n := f.orders(assets.Surveyor); n != 1 {
		t.Errorf("Surveyor orders after asking twice = %d, want 1", n)
	}

	f.asker.requestUnit(f.ctx, unitNeed{unit: assets.Constrct, factory: assets.LandPlt, flags: flagLandUnit})
	if n := len(f.requests()); n != 2 {
		t.Errorf("requests after a second type = %d, want 2", n)
	}

	other := &probeTask{TaskBase: newTaskBase(1, nil, taskPriorityExplore)}
	f.tasks.AppendTask(other)
	other.requestUnit(f.ctx, surveyorNeed)
	if n := len(f.requests()); n != 3 {
		t.Errorf("requests after a second parent = %d, want 3", n)
	}
}

// TestRequestUnitFromALoneTask checks a task with no manager (a lone task in a test): the
// request still works once and builds the unit.
func TestRequestUnitFromALoneTask(t *testing.T) {
	f := newObtainFixture(t)
	lone := newTaskProduceUnits(1, StrategyCombinedArms, f.ctx)

	lone.requestUnit(f.ctx, surveyorNeed)

	if n := f.orders(assets.Surveyor); n != 1 {
		t.Errorf("Surveyor orders from a lone task = %d, want 1", n)
	}
}

// TestRequestersAskThroughTheObtainTask checks the three live requesters through a task
// manager: the missing Surveyor (production), the missing Engineer (building manager)
// and the missing Bulldozer each open a request that is a child of the task that asked,
// and the plant turns out exactly the one unit.
func TestRequestersAskThroughTheObtainTask(t *testing.T) {
	cases := []struct {
		name string
		unit assets.UnitType
		ask  func(f *obtainFixture) Task
	}{
		{"production asks for a Surveyor", assets.Surveyor, func(f *obtainFixture) Task {
			task := newTaskProduceUnits(1, StrategyCombinedArms, f.ctx)
			f.tasks.AppendTask(task)
			task.ensureRecon()
			return task
		}},
		{"the building manager asks for an Engineer", assets.Engineer, func(f *obtainFixture) Task {
			task := newTaskManageBuildings(1, StrategyCombinedArms, 0, false, f.ctx)
			f.tasks.AppendTask(task)
			task.ensureBuilders()
			return task
		}},
		{"the bulldozer task asks for a Bulldozer", assets.Bulldozr, func(f *obtainFixture) Task {
			task := newTaskBulldozer(1, f.ctx)
			f.tasks.AppendTask(task)
			task.ensureBulldozer()
			return task
		}},
	}
	for _, c := range cases {
		f := newObtainFixture(t)
		asker := c.ask(f)

		found := false
		for _, request := range f.requests() {
			for _, want := range request.wants {
				if want.unit == c.unit && request.parent == asker {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("%s: no open request of the asking task for the unit", c.name)
		}
		if n := f.orders(c.unit); n != 1 {
			t.Errorf("%s: orders = %d, want 1", c.name, n)
		}
	}
}

// TestComputerTeamObtainsItsMissingSurveyor is the live seam through the real turn: a
// computer team with its builders, a Light Vehicle Plant and no Surveyor gets one built
// through a request of its production task, and only one over two turns.
func TestComputerTeamObtainsItsMissingSurveyor(t *testing.T) {
	f := newObtainFixture(t)
	constructorUnit(1, 1, 10, 4, f.units)
	engineerUnit(1, 2, 10, 6, f.units)
	f.g.AI = New(testUnitFactory)

	f.g.BeginTeamTurn(1)

	if n := f.orders(assets.Surveyor); n != 1 {
		t.Fatalf("Surveyor orders after the first turn = %d, want 1", n)
	}
	a := f.g.AI.(*AI)
	found := false
	for _, task := range a.mgr.tasks {
		if request, ok := task.(*taskObtainUnits); ok && request.Team() == 1 {
			if _, fromProduction := request.parent.(*taskProduceUnits); fromProduction {
				found = true
			}
		}
	}
	if !found {
		t.Error("the Surveyor was not asked for through a request of the production task")
	}

	f.g.BeginTeamTurn(1)
	if n := f.orders(assets.Surveyor); n != 1 {
		t.Errorf("Surveyor orders after the second turn = %d, want 1 (no double build)", n)
	}
}

// TestObtainUnitsUsesOnlyItsOwnIdlePlantOfTheType checks the plant choice: another team's
// plant, a plant of another type, a plant still being built and a busy plant are passed over
// for a good one further down the unit list.
func TestObtainUnitsUsesOnlyItsOwnIdlePlantOfTheType(t *testing.T) {
	cases := []struct {
		name  string
		spoil func(f *obtainFixture)
	}{
		{"another team's plant", func(f *obtainFixture) { f.plant.Team = 2 }},
		{"a plant of another type", func(f *obtainFixture) { f.plant.Type = assets.LandPlt }},
		{"a plant still being built", func(f *obtainFixture) { f.plant.Building = true }},
		{"a busy plant", func(f *obtainFixture) {
			if produceUnit(f.ctx, f.plant, assets.Scout, flagLandUnit) == nil {
				t.Fatal("fixture: could not busy the plant")
			}
		}},
	}
	for _, c := range cases {
		f := newObtainFixture(t)
		c.spoil(f)
		good := stationary(1, 61, assets.LightPlt, 10, 10, f.units)

		f.asker.requestUnit(f.ctx, surveyorNeed)

		order := findOrder(f.g, assets.Surveyor)
		if order == nil || order.Builder != good {
			t.Errorf("%s: the Surveyor was not ordered at the good plant (order %v)", c.name, order)
		}
	}
}

// TestObtainUnitsKeepsTrackOfItsBuildAcrossTurns checks the tracking: a turn passes while the
// unit is built, then it lands and another task drives it off at once; the request still
// closes on it and orders no second one.
func TestObtainUnitsKeepsTrackOfItsBuildAcrossTurns(t *testing.T) {
	f := newObtainFixture(t)
	f.asker.requestUnit(f.ctx, surveyorNeed)
	request := f.requests()[0]

	f.tasks.BeginTurn(1)
	f.tasks.ExecuteReminders() // a turn while it is built

	product := f.landBuild(t)
	if !f.g.OrderMove(product, 12, 12) {
		t.Fatal("fixture: could not drive the new Surveyor")
	}
	request.BeginTurn()

	if request.IsNeeded() {
		t.Error("the request lost track of its unit while it was built")
	}
	if n := f.orders(assets.Surveyor); n != 0 {
		t.Errorf("Surveyor orders = %d, want 0 (no second build)", n)
	}
}

// TestOrphanRequestKeepsItsOwnPriority checks the request made by a lone task with no task
// value to point at: with no parent it reports the priority it was built with.
func TestOrphanRequestKeepsItsOwnPriority(t *testing.T) {
	f := newObtainFixture(t)
	request := &taskObtainUnits{TaskBase: newTaskBase(1, nil, taskPriorityExplore), ctx: f.ctx}

	if got := request.Priority(); got != taskPriorityExplore {
		t.Errorf("orphan request Priority() = %#x, want its own %#x", got, taskPriorityExplore)
	}
}
