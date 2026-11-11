// dispatch_test.go : testing unit for the unit dispatcher and the production check in taskmanager.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// unbuildableSurveyor wants a Surveyor from a plant the fixture team does not have, so a
// request waits instead of building one.
var unbuildableSurveyor = unitNeed{unit: assets.Surveyor, factory: assets.AirPlt, flags: flagLandUnit}

// openRequest registers a probe parent of the given priority and has it ask for need.
func openRequest(f *obtainFixture, priority uint16, need unitNeed) (*probeTask, *taskObtainUnits) {
	parent := &probeTask{TaskBase: newTaskBase(1, nil, priority)}
	f.tasks.AppendTask(parent)
	parent.requestUnit(f.ctx, need)

	for _, task := range f.tasks.tasks {
		if request, ok := task.(*taskObtainUnits); ok && request.parent == Task(parent) {
			return parent, request
		}
	}

	return parent, nil
}

// TestDispatchGivesAContestedUnitToTheMostUrgentRequest checks the dispatcher's choice:
// of two requests for one free unit the more urgent gets it, whatever the order they were
// made in; on a tie the first registered request wins.
func TestDispatchGivesAContestedUnitToTheMostUrgentRequest(t *testing.T) {
	f := newObtainFixture(t)
	_, weak := openRequest(f, taskPriorityAutoSurvey, unbuildableSurveyor)
	_, urgent := openRequest(f, taskPriorityEscort, unbuildableSurveyor)
	if weak == nil || urgent == nil {
		t.Fatal("fixture: a request did not stay open")
	}
	unit := liveSurveyor(1, 6, 6, f.units)

	f.tasks.dispatchUnits(f.g, 1)

	if urgent.IsNeeded() {
		t.Error("the more urgent request did not get the unit")
	}
	if !weak.IsNeeded() {
		t.Error("the less urgent request got the unit as well")
	}
	if !f.tasks.handed[unit] {
		t.Error("the dispatcher did not note the hand-off")
	}

	// A tie: the first registered request wins.
	g := newObtainFixture(t)
	_, first := openRequest(g, taskPriorityExplore, unbuildableSurveyor)
	_, second := openRequest(g, taskPriorityExplore, unbuildableSurveyor)
	liveSurveyor(1, 6, 6, g.units)
	g.tasks.dispatchUnits(g.g, 1)
	if first.IsNeeded() || !second.IsNeeded() {
		t.Error("on a tie the dispatcher did not keep the first registered request")
	}
}

// TestDispatchHandsOnlyFreeReadyAcceptedUnits checks the candidates: a unit a task holds,
// a busy unit, a unit the need rejects, another team's unit, and a request's own build
// are not handed out.
func TestDispatchHandsOnlyFreeReadyAcceptedUnits(t *testing.T) {
	cases := []struct {
		name  string
		setup func(f *obtainFixture) unitNeed
	}{
		{"a held unit", func(f *obtainFixture) unitNeed {
			unit := liveSurveyor(1, 6, 6, f.units)
			holder := &probeTask{TaskBase: newTaskBase(1, nil, taskPriorityEmergency)}
			f.tasks.AppendTask(holder)
			holder.claim(unit)
			return unbuildableSurveyor
		}},
		{"a building unit", func(f *obtainFixture) unitNeed {
			liveSurveyor(1, 6, 6, f.units).Building = true
			return unbuildableSurveyor
		}},
		{"a dead unit", func(f *obtainFixture) unitNeed {
			liveSurveyor(1, 6, 6, f.units).Hits = 0
			return unbuildableSurveyor
		}},
		{"a unit the need rejects", func(f *obtainFixture) unitNeed {
			liveSurveyor(1, 6, 6, f.units)
			need := unbuildableSurveyor
			need.accept = func(*game.UnitInfo) bool { return false }
			return need
		}},
		{"another team's unit", func(f *obtainFixture) unitNeed {
			liveSurveyor(2, 6, 6, f.units)
			return unbuildableSurveyor
		}},
	}
	for _, c := range cases {
		f := newObtainFixture(t)
		need := c.setup(f)
		_, request := openRequest(f, taskPriorityExplore, need)
		if request == nil {
			t.Fatalf("%s: fixture: the request closed at once", c.name)
		}

		f.tasks.dispatchUnits(f.g, 1)

		if !request.IsNeeded() {
			t.Errorf("%s: the dispatcher handed it out", c.name)
		}
	}
}

// TestDispatchLeavesARequestsOwnBuildToThatRequest checks that the unit a request had built
// is not given to another request that wants the type: its own request closes on it.
func TestDispatchLeavesARequestsOwnBuildToThatRequest(t *testing.T) {
	f := newObtainFixture(t)
	_, builder := openRequest(f, taskPriorityExplore, surveyorNeed) // builds at the plant
	_, other := openRequest(f, taskPriorityEmergency, unbuildableSurveyor)
	if builder == nil || other == nil {
		t.Fatal("fixture: a request did not stay open")
	}
	product := f.landBuild(t)
	product.Hits = 10 // alive and ready, so only the own build rule keeps it from the other request

	f.tasks.dispatchUnits(f.g, 1)

	if !other.IsNeeded() {
		t.Error("another request was given the unit a request had built for itself")
	}
	builder.BeginTurn()
	if builder.IsNeeded() {
		t.Error("the building request did not close on its own unit")
	}
}

// TestPullSkipsAUnitDispatchedThisTurn checks that a request's own find does not take a unit
// the dispatcher already gave to another request this turn.
func TestPullSkipsAUnitDispatchedThisTurn(t *testing.T) {
	f := newObtainFixture(t)
	_, first := openRequest(f, taskPriorityEscort, unbuildableSurveyor)
	_, second := openRequest(f, taskPriorityExplore, unbuildableSurveyor)
	liveSurveyor(1, 6, 6, f.units)

	f.tasks.dispatchUnits(f.g, 1)
	second.BeginTurn()

	if first.IsNeeded() {
		t.Fatal("fixture: the first request did not get the unit")
	}
	if !second.IsNeeded() {
		t.Error("the second request's own find took the unit already handed out")
	}
}

// TestDispatchNotesNewcomers checks the first sight turn the production check reads, and
// that a unit that left the map is forgotten.
func TestDispatchNotesNewcomers(t *testing.T) {
	f := newObtainFixture(t)
	f.g.State.TurnCounter = 3
	unit := liveSurveyor(1, 6, 6, f.units)

	f.tasks.dispatchUnits(f.g, 1)
	if seen, ok := f.tasks.firstSeen[unit]; !ok || seen != 3 {
		t.Fatalf("firstSeen = %d, %v; want turn 3", seen, ok)
	}

	f.g.State.TurnCounter = 4
	f.tasks.dispatchUnits(f.g, 1)
	if f.tasks.firstSeen[unit] != 3 {
		t.Error("a later dispatch moved the first sight turn")
	}

	f.units.Remove(unit)
	f.tasks.dispatchUnits(f.g, 1)
	if _, ok := f.tasks.firstSeen[unit]; ok {
		t.Error("a unit that left the map is still remembered")
	}
}

// TestComputerTurnDispatchesToARequest is the live seam: the real turn hands a free unit to
// the open request that wants it, before the tasks plan.
func TestComputerTurnDispatchesToARequest(t *testing.T) {
	f := newObtainFixture(t)
	f.g.AI = New(testUnitFactory)
	a := f.g.AI.(*AI)
	f.g.BeginTeamTurn(1) // the tasks register

	parent := &probeTask{TaskBase: newTaskBase(1, nil, taskPriorityEscort)}
	a.mgr.AppendTask(parent)
	parent.requestUnit(a.ctx, unbuildableSurveyor)
	unit := liveSurveyor(1, 12, 12, f.units)

	f.g.BeginTeamTurn(1)

	if !a.mgr.handed[unit] {
		t.Error("the computer turn did not dispatch the free Surveyor to the open request")
	}
	if a.mgr.readyForOrders == nil {
		t.Error("the computer turn did not set the ledger's order check")
	}
}

// TestIsUnitNeededRules checks the production check: a free unit of the type seen on an
// earlier turn stops another; a newcomer, an unseen unit, a held one, a dead one and another
// team's do not; a unit whose task is gone counts as free.
func TestIsUnitNeededRules(t *testing.T) {
	cases := []struct {
		name   string
		setup  func(f *obtainFixture, truck *game.UnitInfo)
		needed bool
	}{
		{"a free truck seen last turn", func(*obtainFixture, *game.UnitInfo) {}, false},
		{"a truck new this turn", func(f *obtainFixture, truck *game.UnitInfo) {
			f.tasks.firstSeen[truck] = 5
		}, true},
		{"a truck the dispatcher never saw", func(f *obtainFixture, truck *game.UnitInfo) {
			delete(f.tasks.firstSeen, truck)
		}, true},
		{"a held truck", func(f *obtainFixture, truck *game.UnitInfo) {
			f.asker.claim(truck)
		}, true},
		{"a truck whose task is no longer needed", func(f *obtainFixture, truck *game.UnitInfo) {
			// A child holds the truck; its parent goes, and the child is not pruned yet.
			child := &probeTask{TaskBase: newTaskBase(1, f.asker, taskPriorityExplore)}
			f.tasks.AppendTask(child)
			child.claim(truck)
			f.asker.RemoveSelf()
		}, false},
		{"a dead truck", func(_ *obtainFixture, truck *game.UnitInfo) {
			truck.Hits = 0
		}, true},
		{"another team's truck", func(_ *obtainFixture, truck *game.UnitInfo) {
			truck.Team = 2
		}, true},
	}
	for _, c := range cases {
		f := newObtainFixture(t)
		truck := armedUnit(1, 30, assets.SplyTrck, flagLandUnit, 8, 8, f.units)
		truck.Hits = 10
		f.g.State.TurnCounter = 4
		f.tasks.dispatchUnits(f.g, 1) // first seen on turn 4
		f.g.State.TurnCounter = 5
		c.setup(f, truck)

		if got := f.tasks.isUnitNeeded(f.g, 1, assets.SplyTrck); got != c.needed {
			t.Errorf("%s: isUnitNeeded = %v, want %v", c.name, got, c.needed)
		}
	}
}

// TestChooseProductSkipsTypesNotNeeded checks the filter: a ruled out type is skipped, all
// ruled out = nothing, and no filter keeps the old choice.
func TestChooseProductSkipsTypesNotNeeded(t *testing.T) {
	mix := func(assets.UnitType) int { return 0 }
	plain, ok := chooseProduct(assets.LightPlt, StrategyCombinedArms, mix, nil)
	if !ok {
		t.Fatal("no product without a filter")
	}

	filtered, ok := chooseProduct(assets.LightPlt, StrategyCombinedArms, mix, func(u assets.UnitType) bool { return u != plain.t })
	if !ok || filtered.t == plain.t {
		t.Errorf("the ruled out %s was still chosen", plain.t.Tag())
	}

	if _, ok := chooseProduct(assets.LightPlt, StrategyCombinedArms, mix, func(assets.UnitType) bool { return false }); ok {
		t.Error("a factory whose every product is ruled out still got one")
	}
}

// TestProductionSkipsATypeThatStandsIdle is the production seam through a manager: a Light
// Vehicle Plant whose every product has an idle unit builds nothing, and once one type has
// none idle, it builds that one.
func TestProductionSkipsATypeThatStandsIdle(t *testing.T) {
	f := newObtainFixture(t)
	liveSurveyor(1, 2, 2, f.units) // the recon need is met, so the plant is free for the mix
	idle := map[assets.UnitType]*game.UnitInfo{}
	for i, p := range factoryProducts[assets.LightPlt] {
		u := armedUnit(1, 40+i, p.t, p.flags, 8+i, 10, f.units)
		u.Hits = 10
		idle[p.t] = u
	}
	f.g.State.TurnCounter = 1
	f.tasks.dispatchUnits(f.g, 1)
	f.g.State.TurnCounter = 2

	production := newTaskProduceUnits(1, StrategyCombinedArms, f.ctx)
	f.tasks.AppendTask(production)
	production.BeginTurn()
	if f.g.BuildOrderFor(f.plant) != nil {
		t.Fatalf("the plant took an order while every product stands idle: %s", f.g.BuildOrderFor(f.plant).Product.Type.Tag())
	}

	// A task takes the Scout into use: Scouts are needed again.
	f.asker.claim(idle[assets.Scout])
	production.BeginTurn()
	order := f.g.BuildOrderFor(f.plant)
	if order == nil || order.Product.Type != assets.Scout {
		t.Errorf("the plant did not build the one type with no idle unit (order %v)", order)
	}
}

// TestDispatchSkipsAnotherTeamsRequest checks the request filter: a request of another team
// never gets the unit, even when it is the only one that wants the type.
func TestDispatchSkipsAnotherTeamsRequest(t *testing.T) {
	f := newObtainFixture(t)
	other := &probeTask{TaskBase: newTaskBase(2, nil, taskPriorityEmergency)}
	f.tasks.AppendTask(other)
	other.requestUnit(f.ctx, unbuildableSurveyor)
	unit := liveSurveyor(1, 6, 6, f.units)

	f.tasks.dispatchUnits(f.g, 1)

	if f.tasks.handed[unit] {
		t.Error("team 1's unit was handed to a request of team 2")
	}
}
