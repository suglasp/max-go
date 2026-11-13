// task_obtainunits.go : the child task that gets its parent a unit it is missing, by finding one or having one built.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : taskobtainunits.cpp - TaskObtainUnits (FindUnit, IsValidCandidate, GetPriority, IsNeeded, EndTurn)
package ai

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
)

// unitNeed is one unit a task wants: its type, the plant that builds it, its deploy
// flags, and an optional test for which existing units count (nil = any of the type).
type unitNeed struct {
	unit    assets.UnitType
	factory assets.UnitType
	flags   game.Flags
	accept  func(u *game.UnitInfo) bool
}

// taskObtainUnits is a child of the task that asked for units. It works with its
// parent's priority: it first looks for a free unit of each wanted type, and when there
// is none it has one built at the type's plant. It leaves the list once nothing is
// wanted any more or its parent is gone. Handing a unit over is a notice only: the
// parent's own pass picks up its units each turn, so nothing is reserved for it.
// C++: class TaskObtainUnits (taskobtainunits.cpp).
type taskObtainUnits struct {
	TaskBase
	ctx     *aiContext
	wants   []unitNeed
	ordered map[assets.UnitType]*game.UnitInfo // the unit this task had built, per wanted type, until it is fielded
}

// Type marks the task as a unit request. C++: TaskObtainUnits::GetType.
func (t *taskObtainUnits) Type() TaskType {
	return TaskTypeObtainUnits
}

// Priority is the parent's priority: a request is as urgent as the task that made it.
// C++: TaskObtainUnits::GetPriority.
func (t *taskObtainUnits) Priority() uint16 {
	if t.parent != nil {
		return t.parent.Priority()
	}

	return t.basePriority
}

// Init works the request at once, inside the call of the task that asked, so a plant
// is taken before the rest of the turn plans. C++: TaskObtainUnits::Init (it queues a
// turn end for the same think pass).
func (t *taskObtainUnits) Init() {
	t.work()
}

// BeginTurn works the request again on a later turn. C++: TaskObtainUnits::BeginTurn.
func (t *taskObtainUnits) BeginTurn() {
	t.work()
}

// work walks the wanted types from the last one back: each is met by a unit this task
// had built, by a free unit of the type, or by a new build order; the task removes
// itself once nothing is left, so the base IsNeeded (removed, or the parent gone) is all
// the request needs. C++: TaskObtainUnits::EndTurn + IsNeeded.
func (t *taskObtainUnits) work() {
	g := t.ctx.Game
	if g == nil {
		return
	}

	for i := len(t.wants) - 1; i >= 0; i-- {
		if t.meet(g, t.wants[i]) {
			t.wants = append(t.wants[:i], t.wants[i+1:]...)
		}
	}

	// Nothing left to get: the request is done.
	if len(t.wants) == 0 {
		t.RemoveSelf()
	}
}

// meet tries to satisfy one want and reports whether it is met now.
func (t *taskObtainUnits) meet(g *sim.Game, need unitNeed) bool {
	// A unit this task had built: met once it stands on the map, wait while it is built.
	if product := t.ordered[need.unit]; product != nil {
		if unitOnMap(g, product) {
			dbg.Logf(dbg.AI, "team %d obtained a new %s for task %d", t.team, need.unit.Tag(), t.requesterID())
			delete(t.ordered, need.unit)

			return true
		}

		if inProduction(g, product) {
			return false
		}

		delete(t.ordered, need.unit) // the order was lost: look again
	}

	// A free unit of the type is handed over (a notice: the parent's own pass drives it).
	if u := t.findUnit(g, need); u != nil {
		dbg.Logf(dbg.AI, "team %d obtained %s %d for task %d", t.team, need.unit.Tag(), u.ID, t.requesterID())

		return true
	}

	// Someone is building one already: wait for it.
	if unitsInProduction(g, t.team, need.unit) > 0 {
		return false
	}

	// Have one built at an idle plant of the type.
	for _, f := range g.Units.Units() {
		if f.Team != t.team || f.Type != need.factory || f.Building || g.BuildOrderFor(f) != nil {
			continue
		}

		if product := produceUnit(t.ctx, f, need.unit, need.flags); product != nil {
			if t.ordered == nil {
				t.ordered = make(map[assets.UnitType]*game.UnitInfo)
			}
			t.ordered[need.unit] = product
			dbg.Logf(dbg.AI, "team %d building a %s at a %s for task %d", t.team, need.unit.Tag(), need.factory.Tag(), t.requesterID())

			return false
		}
	}

	return false
}

// requesterID returns the id of the task that asked, 0 for a request made by a lone
// task in a test (one never bound to a manager has no task value to point at).
func (t *taskObtainUnits) requesterID() uint32 {
	if t.parent == nil {
		return 0
	}

	return t.parent.ID()
}

// wantsUnit reports whether the request still wants a unit of u's type that its
// need accepts.
func (t *taskObtainUnits) wantsUnit(u *game.UnitInfo) bool {
	for _, want := range t.wants {
		if want.unit == u.Type && (want.accept == nil || want.accept(u)) {
			return true
		}
	}

	return false
}

// takeUnit takes u, handed over by the dispatcher, for the first want of its type: a
// notice (the parent's own pass drives it). The request leaves once nothing is wanted.
// C++: TaskObtainUnits::AddUnit.
func (t *taskObtainUnits) takeUnit(u *game.UnitInfo) {
	for i, want := range t.wants {
		if want.unit != u.Type {
			continue
		}

		dbg.Logf(dbg.AI, "team %d dispatched %s %d to task %d", t.team, u.Type.Tag(), u.ID, t.requesterID())
		t.wants = append(t.wants[:i], t.wants[i+1:]...)
		break
	}

	// Nothing left to get: the request is done.
	if len(t.wants) == 0 {
		t.RemoveSelf()
	}
}

// findUnit returns the team's free unit of the wanted type nearest the base centre, or
// nil: alive, not under construction, without an order, accepted by the need's own test,
// and not held by a more urgent task. C++: TaskObtainUnits::FindUnit + IsValidCandidate
// Requests here have no site, so the base centre provides the distance origin.
func (t *taskObtainUnits) findUnit(g *sim.Game, need unitNeed) *game.UnitInfo {
	siteX, siteY, _ := teamBaseCenter(g, t.team)

	var best *game.UnitInfo
	bestDistance := 0

	for _, u := range g.Units.Units() {
		if u.Team != t.team || u.Type != need.unit || u.Hits <= 0 || u.Building || g.HasOrder(u) {
			continue
		}

		// The asking task may rule a unit out (a builder boxed in cannot work).
		if need.accept != nil && !need.accept(u) {
			continue
		}

		if !t.mayClaim(u) {
			continue
		}

		// The dispatcher already gave this unit to another request this turn.
		if t.mgr != nil && t.mgr.handed[u] {
			continue
		}

		distance := chebyshev(u.GridX, u.GridY, siteX, siteY)
		if best == nil || distance < bestDistance {
			best, bestDistance = u, distance
		}
	}

	return best
}

// requestUnit asks for one unit of need.unit for this task through a child
// taskObtainUnits, unless a live request of this task already wants that type. The
// child works at once. A task with no manager (a lone task in a test) still gets the
// one pass. C++: TaskObtainUnits::TaskObtainUnits + AddUnit(type) + TaskManager.AppendTask.
func (t *TaskBase) requestUnit(ctx *aiContext, need unitNeed) {
	// One open request per type and parent.
	if t.mgr != nil {
		for _, task := range t.mgr.tasks {
			request, ok := task.(*taskObtainUnits)
			if !ok || request.parent != t.self || !request.IsNeeded() {
				continue
			}

			// This task already waits for one of the type.
			for _, want := range request.wants {
				if want.unit == need.unit {
					return
				}
			}
		}
	}

	request := &taskObtainUnits{
		TaskBase: newTaskBase(t.team, t.self, t.basePriority),
		ctx:      ctx,
		wants:    []unitNeed{need},
	}

	// A lone task: bind the request to no manager and work it once.
	if t.mgr == nil {
		request.bind(request, nil)
		request.Init()

		return
	}

	t.mgr.AppendTask(request)
}

// produceUnit starts one unitType at the factory on a free deploy cell and returns the
// new unit, or nil when there is no cell or no unit. C++: TaskCreateUnit at the plant.
func produceUnit(ctx *aiContext, factory *game.UnitInfo, unitType assets.UnitType, flags game.Flags) *game.UnitInfo {
	g := ctx.Game
	x, y, ok := g.FactoryDeployCell(factory, flags)
	if !ok {
		return nil
	}

	unit := ctx.MakeUnit(factory.Team, unitType, flags)
	if unit == nil {
		return nil
	}

	g.StartBuild(factory, unit, x, y, aiBuildRate(g, factory, unit))

	return unit
}

// unitsInProduction counts the units of a type the team's factories are building.
func unitsInProduction(g *sim.Game, team int, unitType assets.UnitType) int {
	n := 0
	for _, o := range g.BuildOrders() {
		if o.Builder != nil && o.Builder.Team == team && o.Product != nil && o.Product.Type == unitType {
			n++
		}
	}

	return n
}

// inProduction reports whether a build order is still building product.
func inProduction(g *sim.Game, product *game.UnitInfo) bool {
	for _, o := range g.BuildOrders() {
		if o.Product == product {
			return true
		}
	}

	return false
}

// unitOnMap reports whether u stands on the map among the game's units.
func unitOnMap(g *sim.Game, u *game.UnitInfo) bool {
	for _, other := range g.Units.Units() {
		if other == u {
			return true
		}
	}

	return false
}
