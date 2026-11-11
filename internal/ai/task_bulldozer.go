// task_bulldozer.go : M.A.X. AI bulldozer actions.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import (
	"sort"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
)

// taskBulldozer is the team's ground reclaim brain: each turn it puts an idle Bulldozer
// to work clearing the nearest debris pile near the base and - when there is rubble in
// the base area but the team fields no Bulldozer - has a Light plant build one. A
// destroyed unit or building leaves rubble on its cell (sim/rubble.go); clearing it (an
// ORDER_CLEAR countdown) reclaims the ground so the base can rebuild on the footprint.
// It does not reschedule itself; TaskManager queues it again each turn.
//
// Reductions vs the original: no TaskClearGround tied to one specific planned build (it
// clears the nearest pile in the base area, not a chosen build site); the obtain is a
// fixed "one dozer when rubble exists and none is fielded" instead of the original's
// TaskObtainUnits tied to each build; no terrain LEVELLING (vanilla's dozer also
// flattens bumps) - only rubble removal, which is all the sim models.
//
// C++ ported : the bulldozer clearing task family - the AI driving UnitsManager ORDER_CLEAR
// (TaskClearGround / TaskObtainUnits(BULLDOZER)).
type taskBulldozer struct {
	TaskBase
	ctx *aiContext
}

// bulldozeRange is how far from the base centre a pile may lie to be worth clearing: the
// dozer reclaims debris around its own base, it does not trek the map to a far off wreck.
const bulldozeRange = 16

func newTaskBulldozer(team int, ctx *aiContext) *taskBulldozer {
	return &taskBulldozer{
		TaskBase: newTaskBase(team, nil, taskPriorityRemoveRubble),
		ctx:      ctx,
	}
}

// BeginTurn clears rubble in the base area with the team's bulldozers, building one first if
// there is rubble to reclaim and none is fielded.
func (t *taskBulldozer) BeginTurn() {
	g := t.ctx.Game
	if g == nil {
		return
	}
	piles := t.baseAreaRubble()
	if len(piles) == 0 {
		return // nothing near the base to reclaim
	}
	dozers := t.idleBulldozers()
	if len(dozers) == 0 {
		t.ensureBulldozer() // rubble to clear but no dozer: have a Light plant build one
		return
	}
	for _, u := range dozers {
		if g.CanClearRubble(u) {
			if g.StartClearRubble(u) {
				dbg.Printf("ai: team %d BULLDOZER clears rubble at (%d,%d)", t.team, u.GridX, u.GridY)
			}
			continue
		}
		t.driveToNearestPile(u, piles)
	}
}

// baseAreaRubble returns the debris piles within bulldozeRange of the team's base centre -
// the rubble worth reclaiming (typically destroyed building footprints in the base). It is
// empty when the team owns no building or there is no nearby rubble.
func (t *taskBulldozer) baseAreaRubble() []sim.RubbleCell {
	g := t.ctx.Game
	bx, by, ok := teamBaseCenter(g, t.team)
	if !ok {
		return nil
	}
	var near []sim.RubbleCell
	for _, p := range g.RubbleCells() {
		if chebyshev(bx, by, p.X, p.Y) <= bulldozeRange {
			near = append(near, p)
		}
	}
	return near
}

// idleBulldozers returns the team's bulldozers free to take a new order (not already
// clearing, not mid move, with a move budget), in ID order so the assignment is stable.
func (t *taskBulldozer) idleBulldozers() []*game.UnitInfo {
	g := t.ctx.Game
	var out []*game.UnitInfo
	for _, u := range g.Units.Units() {
		if u.Team != t.team || !sim.IsBulldozer(u) {
			continue
		}
		if g.IsClearingRubble(u) || g.HasOrder(u) || u.Speed <= 0 {
			continue
		}
		out = append(out, u)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// driveToNearestPile sends a bulldozer toward the closest pile in the base area so it can clear
// it next turn. It tries the piles nearest first and issues the first move that yields a
// route, skipping any pile it cannot path to.
func (t *taskBulldozer) driveToNearestPile(u *game.UnitInfo, piles []sim.RubbleCell) {
	g := t.ctx.Game
	order := make([]sim.RubbleCell, 0, len(piles))
	for _, p := range piles {
		if chebyshev(u.GridX, u.GridY, p.X, p.Y) > 0 { // not the cell it already stands on
			order = append(order, p)
		}
	}
	sort.Slice(order, func(i, j int) bool {
		return chebyshev(u.GridX, u.GridY, order[i].X, order[i].Y) <
			chebyshev(u.GridX, u.GridY, order[j].X, order[j].Y)
	})
	for _, p := range order {
		if g.OrderMove(u, p.X, p.Y) {
			dbg.Printf("ai: team %d BULLDOZER %s -> rubble (%d,%d)", t.team, u.Type.Tag(), p.X, p.Y)
			return
		}
	}
}

// ensureBulldozer has one idle Light plant build a Bulldozer (the obtain), so a team with
// rubble in the base area but no dozer can still reclaim its ground. Mirrors ensureRecon: one in
// flight satisfies the need (countBulldozers counts production), so it builds at most one.
// C++: TaskObtainUnits(BULLDOZER) when ground needs clearing.
func (t *taskBulldozer) ensureBulldozer() {
	g := t.ctx.Game
	if t.countBulldozers() > 0 {
		return // already fielding or building one
	}
	for _, f := range g.Units.Units() {
		if f.Team != t.team || f.Type != assets.LightPlt || f.Building || g.BuildOrderFor(f) != nil {
			continue
		}
		x, y, ok := g.FactoryDeployCell(f, flagLandUnit)
		if !ok {
			continue
		}
		unit := t.ctx.MakeUnit(t.team, assets.Bulldozr, flagLandUnit)
		if unit == nil {
			return
		}
		g.StartBuild(f, unit, x, y, aiBuildRate(g, f, unit))
		dbg.Printf("ai: team %d builds a BULLDOZER to clear rubble", t.team)
		return
	}
}

// countBulldozers counts the team's bulldozers, including any still in production.
func (t *taskBulldozer) countBulldozers() int {
	return buildingCount(t.ctx.Game, t.team, assets.Bulldozr)
}
