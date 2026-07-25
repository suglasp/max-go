// task_produceunits.go : M.A.X. AI producing units
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// taskProduceUnits is a team's production brain: each turn it puts every idle
// factory (light/heavy/air/sea plant, barracks) to work building a mobile unit,
// balancing the army across each factory's product list. It is the tractable
// port of the original's TaskCreateUnit driven from the build managers. The sim
// already handles factory output: StartBuild on a stationary builder finishes
// into a unit deployed at the chosen cell; so this only picks the unit and a free
// drop cell. It does not reschedule itself: TaskManager.BeginTurn queues it again
// each turn. C++: TaskCreateUnit / the factory production assistants
// (taskcreateunit.cpp, aiplayer.cpp).
type taskProduceUnits struct {
	TaskBase
	ctx      *aiContext
	strategy Strategy
}

func newTaskProduceUnits(team int, strategy Strategy, ctx *aiContext) *taskProduceUnits {
	return &taskProduceUnits{
		TaskBase: newTaskBase(team, nil, 900), // just below the building manager
		ctx:      ctx,
		strategy: strategy,
	}
}

// BeginTurn assigns each idle factory a unit to produce. One order per factory
// per turn (an active build order makes the factory not idle next turn; a
// natural rate limit). C++: the production pass run each turn.
func (t *taskProduceUnits) BeginTurn() {
	g := t.ctx.Game
	if g == nil {
		return
	}

	t.ensureRecon() // claim a factory for a missing recon unit before generic production

	for _, f := range g.Units.Units() {
		if !t.isIdleFactory(f) {
			continue
		}

		prod, ok := chooseProduct(f.Type, t.strategy, t.teamUnitCount)
		if !ok {
			continue
		}

		t.produce(f, prod)
	}
}

// ensureRecon makes the team build a recon unit it is short of (a Surveyor for
// resource recon, a Scanner for enemy recon) before generic production. Without
// it a team that loses its starting Surveyor never gets another - the Surveyor is
// in no balanced factory roster - so taskSurvey has nothing to drive, depositScore
// sees no new surveyed deposits, and base expansion silently stalls. It claims one
// idle factory of the right type per missing recon unit; the order already flying then
// satisfies the demand (teamUnitCount counts production), so it builds at most one
// each until the minimum is met. C++ ported : aiplayer.cpp - CheckReconnaissanceNeeds
// + taskfindmines.cpp - TaskObtainUnits(SURVEYOR) (obtaining the reconnaissance unit).
func (t *taskProduceUnits) ensureRecon() {
	g := t.ctx.Game

	for _, d := range reconDemand {
		if t.teamUnitCount(d.unit) >= d.min {
			continue // already fielding / building enough of this recon unit
		}

		for _, f := range g.Units.Units() {
			if f.Type != d.factory || !t.isIdleFactory(f) {
				continue
			}

			if t.produce(f, producible{d.unit, d.flags}) {
				break // one of this recon type per turn
			}
		}
	}
}

// teamUnitCount counts how many of a type the task's team currently fields
// (including ones still in production), so chooseProduct can balance the mix.
func (t *taskProduceUnits) teamUnitCount(unitType assets.UnitType) int {
	g := t.ctx.Game
	n := 0

	for _, u := range g.Units.Units() {
		if u.Team == t.team && u.Type == unitType {
			n++
		}
	}

	for _, o := range g.BuildOrders() {
		if o.Builder != nil && o.Builder.Team == t.team && o.Product != nil && o.Product.Type == unitType {
			n++
		}
	}

	return n
}

// isIdleFactory reports whether f is one of this team's factories that is free
// to take a new production order this turn.
func (t *taskProduceUnits) isIdleFactory(f *game.UnitInfo) bool {
	if f.Team != t.team || !isFactory(f.Type) {
		return false
	}

	if f.Building {
		return false // still under construction itself
	}

	return t.ctx.Game.BuildOrderFor(f) == nil
}

// produce finds a free deploy cell next to the factory and issues the build
// order for the chosen unit. C++: TaskManager.CreateUnit / StartBuilding.
func (t *taskProduceUnits) produce(factory *game.UnitInfo, prod producible) bool {
	g := t.ctx.Game
	x, y, ok := g.FactoryDeployCell(factory, prod.flags)
	if !ok {
		return false
	}

	unit := t.ctx.MakeUnit(t.team, prod.t, prod.flags)
	if unit == nil {
		return false
	}

	g.StartBuild(factory, unit, x, y, aiBuildRate(g, factory, unit))

	return true
}
