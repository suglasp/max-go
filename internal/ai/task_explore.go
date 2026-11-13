// task_explore.go : M.A.X. AI exploring of the map/world.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
)

// taskExplore sends a team's idle RECON units toward the nearest UNEXPLORED cell
// each turn, so a computer team discovers the map and grows its fog memory. Only the
// recon types in exploreCaps explore, each up to its cap; tanks and the other combat
// units stay with the army (a trip to a known enemy camp is the attack task's job).
// A recon unit that comes under fire stops exploring: it fights back when an enemy is
// in range, else falls back toward the base. It does not reschedule itself;
// TaskManager.BeginTurn queues it again each turn.
//
// This implementation approximates TaskSearchDestination's spiral with an outward ring scan.
//
// C++ ported : taskexplore.cpp - TaskExplore (IsUnitUsable's recon types and counts)
type taskExplore struct {
	TaskBase
	ctx *aiContext

	// tm is this turn's team threat map, built lazily for the first ready explorer. A
	// recon unit does not scout into a cell under enemy fire and routes around defended
	// ground, so a scanner or scout stops driving blindly into a hostile base.
	tm *threatMap
}

// exploreCaps lists the unit types that explore and how many of each a team sends at
// once. Scout and Fast Boat use a cap of 3, AWAC uses 1, and Scanner uses 1 for its
// mobile radar. Submarine and Commando are left out because they have other roles.
var exploreCaps = map[assets.UnitType]int{
	assets.Scout:    3,
	assets.FastBoat: 3,
	assets.Awac:     1,
	assets.Scanner:  1,
}

func newTaskExplore(team int, ctx *aiContext) *taskExplore {
	return &taskExplore{
		TaskBase: newTaskBase(team, nil, taskPriorityExplore),
		ctx:      ctx,
	}
}

// BeginTurn sends each ready idle explorer toward the nearest unexplored cell.
// C++: TaskExplore::Execute calling FindDestination for each of its units.
func (t *taskExplore) BeginTurn() {
	g := t.ctx.Game

	if g == nil || g.Vis == nil {
		return
	}

	t.tm = nil // built lazily on the first ready explorer (buildThreatMap is O(enemies x range^2))
	sent := map[assets.UnitType]int{}
	for _, u := range g.Units.Units() {
		// A recon type still under its cap this pass.
		if sent[u.Type] >= exploreCaps[u.Type] || !t.isReadyExplorer(u) {
			continue
		}

		if t.tm == nil {
			t.tm = buildThreatMap(g, t.team)
		}

		// A scout that acts is this task's for the turn and counts toward its cap.
		if t.act(u) {
			t.claim(u)
			sent[u.Type]++
		}
	}
}

// act gives one ready explorer its move: under fire it defends itself, firing back when
// an enemy is in range and its health holds, else falling back; otherwise it heads for
// the nearest fog. Returns whether it acted.
func (t *taskExplore) act(u *game.UnitInfo) bool {
	// Under fire: the explore stops and the scout defends itself.
	if t.tm.at(u.GridX, u.GridY) > 0 {
		// Low on health: no fight, straight back.
		if lowHealth(u) {
			return t.fallBack(u)
		}

		if e := engageableEnemy(t.ctx.Game, t.team, u); e != nil {
			ex, ey := believedCell(t.ctx.Game, t.team, e)
			t.ctx.Game.Attack(u, ex, ey)
			dbg.Printf("ai: team %d scout %s under fire fights back at (%d,%d)", t.team, u.Type.Tag(), ex, ey)

			return true
		}

		return t.fallBack(u)
	}

	return t.nearestUnexploredMove(u)
}

// fallBack drives a scout under fire back toward the base centre, routed around the
// defended ground. It holds (returns false) when the team has no base. C++ ported :
// ai.cpp - Task_RetreatIfNecessary (reduced: home instead of the safest cell).
func (t *taskExplore) fallBack(u *game.UnitInfo) bool {
	g := t.ctx.Game
	bx, by, ok := teamBaseCenter(g, t.team)
	if !ok {
		return false
	}

	// The centre itself when free, else the first free cell beside it (a building
	// usually stands on the centre).
	pen := dangerPenalty(t.tm)
	if g.OrderMoveAvoiding(u, bx, by, pen) {
		dbg.Printf("ai: team %d scout %s under fire falls back to (%d,%d)", t.team, u.Type.Tag(), bx, by)

		return true
	}

	for _, c := range g.AdjacentApproachCells(u, bx, by) {
		if g.OrderMoveAvoiding(u, c.X, c.Y, pen) {
			dbg.Printf("ai: team %d scout %s under fire falls back to (%d,%d)", t.team, u.Type.Tag(), c.X, c.Y)

			return true
		}
	}

	return false
}

// isReadyExplorer reports whether u is one of this team's idle recon units free to
// scout this turn: a type in exploreCaps, mobile (not a building), not a constructor
// (constructors build, not wander), has scan range to reveal fog, has movement budget
// left, and carries no order yet. C++: TaskExplore::Execute gate (IsReadyForOrders &&
// speed) plus IsUnitUsable's recon type selection.
func (t *taskExplore) isReadyExplorer(u *game.UnitInfo) bool {
	if u.Team != t.team || u.Values == nil {
		return false
	}

	if exploreCaps[u.Type] == 0 {
		return false // not a recon type: a combat unit stays with the army
	}

	if !isMobileUnit(u.Flags) || u.Flags.Has(game.FlagConstructorUnit) {
		return false
	}

	if u.Type == assets.Surveyor {
		return false // the surveyor's job is the resource survey (taskSurvey), not fog explore
	}

	if !t.mayClaim(u) {
		return false // a damaged unit, a home guard or an escort belongs to a more urgent task
	}

	if u.Values.Get(game.AttribScan) <= 0 {
		return false // no scan: nothing to reveal by moving
	}

	if u.Speed <= 0 {
		return false // no movement budget left this turn
	}

	return !t.ctx.Game.HasOrder(u)
}

// nearestUnexploredMove orders u toward the nearest reachable cell its team has
// never scanned, returning whether a move was issued. It rings outward from the
// unit (the spiral search in miniature): the unit's own scanned bubble is already
// explored, so the first hit is the fog frontier, which for a scout is the
// nearest worthwhile destination. CanEnter doubles as the surface test (a land
// scout will not target water), covering the original's IsVisited GetSurfaceType
// check. Returns false when the whole reachable map is explored (nothing to scout).
//
// C++ ported : taskabstractsearch.cpp - TaskAbstractSearch::FindDestination
// (reduced: the spiral TaskSearchDestination is approximated by a ring scan; no
// danger retreat or automatic repair gating yet).
func (t *taskExplore) nearestUnexploredMove(u *game.UnitInfo) bool {
	g := t.ctx.Game
	fog := g.Vis.Team(t.team)

	if fog == nil {
		return false
	}

	maxR := g.W
	if g.H > maxR {
		maxR = g.H
	}

	// Route around defended ground; the destinations below are already screened to be
	// threat free, so this only bends the PATH away from any covered cell it crosses.
	pen := dangerPenalty(t.tm)

	for r := 1; r <= maxR; r++ {
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if dx > -r && dx < r && dy > -r && dy < r {
					continue // ring perimeter only (inner cells handled by smaller r)
				}

				x, y := u.GridX+dx, u.GridY+dy
				if x < 0 || y < 0 || x >= g.W || y >= g.H {
					continue
				}

				if fog.Explored(x, y) {
					continue // already seen by this team
				}

				if t.tm.at(x, y) > 0 {
					continue // ground under enemy fire: a fragile recon unit stays out
				}

				if !g.CanEnter(u, x, y) {
					continue // wrong surface / blocked (the IsVisited surface gate)
				}

				if g.OrderMoveAvoiding(u, x, y, pen) {
					return true
				}
			}
		}
	}

	return false
}
