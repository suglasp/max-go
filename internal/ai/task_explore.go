// task_explore.go : M.A.X. AI exploring of the map/world.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// taskExplore sends a team's idle scan capable mobile units toward the nearest
// UNEXPLORED cell each turn, so a computer team with nothing to attack discovers
// the map and grows its fog memory instead of standing still. It runs after the
// attack/turret roots (higher priority number = scheduled last), so it only ever
// moves units those passes left without an order. It does not reschedule itself;
// TaskManager.BeginTurn queues it again each turn.
//
// This implementation drives idle mobile units already owned by the team. It approximates TaskSearchDestination's spiral with an outward ring scan.
//
// C++ ported : taskexplore.cpp - TaskExplore
type taskExplore struct {
	TaskBase
	ctx     *aiContext
	reserve map[*game.UnitInfo]bool // guards assigned to defend the base this turn
	escort  map[*game.UnitInfo]bool // guards assigned to escort another unit this turn
	// tm is this turn's team threat map, built lazily for the first ready explorer. A
	// recon unit does not scout into a cell under enemy fire and routes around defended
	// ground, so a scanner or scout stops driving blindly into a hostile base.
	tm *threatMap
}

func newTaskExplore(team int, ctx *aiContext) *taskExplore {
	return &taskExplore{
		TaskBase: newTaskBase(team, nil, 900), // below the attack/turret roots (800): drives only idle units
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

	// Skip home defence guards and assigned escorts so they are not pulled off their
	// post / charge to scout.
	t.reserve = reservedGuards(g, t.team)
	t.escort = escortGuards(g, t.team)
	t.tm = nil // built lazily on the first ready explorer (buildThreatMap is O(enemies x range^2))
	for _, u := range g.Units.Units() {
		if !t.isReadyExplorer(u) {
			continue
		}

		if t.tm == nil {
			t.tm = buildThreatMap(g, t.team)
		}

		t.nearestUnexploredMove(u)
	}
}

// isReadyExplorer reports whether u is one of this team's idle scan capable
// mobile units free to scout this turn: mobile (not a building), not a
// constructor (constructors build, not wander), has scan range to reveal fog,
// has movement budget left, and carries no order yet. C++: TaskExplore::Execute
// gate (IsReadyForOrders && speed) plus IsUnitUsable's scan class selection.
func (t *taskExplore) isReadyExplorer(u *game.UnitInfo) bool {
	if u.Team != t.team || u.Values == nil {
		return false
	}

	if !isMobileUnit(u.Flags) || u.Flags.Has(game.FlagConstructorUnit) {
		return false
	}

	if u.Type == assets.Surveyor {
		return false // the surveyor's job is the resource survey (taskSurvey), not fog explore
	}

	if unitDamaged(u) {
		return false // a damaged unit is retreating to repair, not scouting into danger
	}

	if t.reserve[u] {
		return false // a home defence guard holds its post, it does not scout
	}

	if t.escort[u] {
		return false // an assigned escort shadows its charge, it does not scout
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
