// task_mines.go : M.A.X. AI laying mines arround base
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import (
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
)

// taskMines seeds defensive mines on the approaches to the team's base: each turn it
// drives every idle mine layer (with mines in store) to a cell in a defensive ring
// around the base, biased toward the side facing the nearest spotted enemy, and lays
// a mine there. It walks the ring laying one mine per fresh cell until the layer runs
// dry. It does not reschedule itself; TaskManager queues it again each turn.
//
// Mine fields are computed as a defensive ring around the base facing the nearest threat. Empty layers hold position, and the near-base ring avoids
// the need for a separate retreat pass.
//
// C++ ported : taskplacemines.cpp - TaskPlaceMines::Execute / BeginTurn.
type taskMines struct {
	TaskBase
	ctx  *aiContext
	laid map[[2]int]bool // laid records the cells this task has already mined, so the ring scan never targets the same one again
}

const (
	// mineInnerRadius keeps mines off the base footprint itself; mineDefenseRadius is
	// the outer edge of the defensive ring on the approaches. Interpreted values (the
	// original reads the field from the InfoMap): a band a few cells out from the
	// base centre.
	mineInnerRadius   = 3
	mineDefenseRadius = 9
)

func newTaskMines(team int, ctx *aiContext) *taskMines {
	return &taskMines{
		TaskBase: newTaskBase(team, nil, taskPriorityFrontier), // mine laying shares the frontier tier
		ctx:      ctx,
	}
}

// BeginTurn lays a mine with each idle layer standing on a fresh ring cell, or drives
// it to the nearest one. C++: TaskPlaceMines::Execute over the team's mine layers.
func (t *taskMines) BeginTurn() {
	g := t.ctx.Game
	if g == nil {
		return
	}

	bx, by, ok := teamBaseCenter(g, t.team)
	if !ok {
		return // no base to ring with mines
	}

	ex, ey, hasThreat := t.nearestSpottedEnemy(bx, by)
	for _, u := range g.Units.Units() {
		if !t.isReadyLayer(u) {
			continue
		}

		// Standing on a fresh defensive cell: lay here (faithful - the layer mines its
		// own cell). CanLayMine also guards store + no existing mine of its type.
		if t.isMineSite(u.GridX, u.GridY, bx, by, ex, ey, hasThreat) && g.CanLayMine(u) {
			if g.LayMine(u) {
				t.markLaid(u.GridX, u.GridY)
				dbg.Printf("ai: team %d MINELAYR lays mine at (%d,%d)", t.team, u.GridX, u.GridY)
			}

			continue
		}

		// Otherwise drive to the nearest fresh ring cell.
		if x, y, found := t.nearestMineSite(u, bx, by, ex, ey, hasThreat); found {
			if g.OrderMove(u, x, y) {
				dbg.Printf("ai: team %d MINELAYR moves to mine (%d,%d)", t.team, x, y)
			}
		}
	}
}

// isReadyLayer reports whether u is one of this team's mine layers free to act this
// turn with mines left to lay.
func (t *taskMines) isReadyLayer(u *game.UnitInfo) bool {
	return sim.IsMineLayer(u) && u.Team == t.team && u.Speed > 0 && u.Storage > 0 && !t.ctx.Game.HasOrder(u)
}

// isMineSite reports whether (x, y) is a cell the team wants mined: inside the
// defensive ring around the base centre, on the half facing the threat (when one is
// known), and not already mined by this task. Terrain/enterability is checked by the
// caller for a drive target (the layer's own cell is trivially valid terrain).
func (t *taskMines) isMineSite(x, y, bx, by, ex, ey int, hasThreat bool) bool {
	if t.laid[[2]int{x, y}] {
		return false
	}

	d := sim.SquaredDist(x, y, bx, by)
	if d < mineInnerRadius*mineInnerRadius || d > mineDefenseRadius*mineDefenseRadius {
		return false
	}

	if hasThreat {
		// Keep to the side of the base that faces the threat: the dot product of the
		// cell offset and the threat offset (both from the base centre) is not negative.
		if (x-bx)*(ex-bx)+(y-by)*(ey-by) < 0 {
			return false
		}
	}

	return true
}

// nearestMineSite returns the nearest enterable, not yet mined ring cell to u (never
// u's own cell, so the layer always makes real progress), or found=false. The ring is
// enumerated by the existing zoneWalker around the base centre, like
// taskRetreat.fleeToSafety enumerates the flee disc.
func (t *taskMines) nearestMineSite(u *game.UnitInfo, bx, by, ex, ey int, hasThreat bool) (int, int, bool) {
	g := t.ctx.Game
	bestX, bestY, bestD, found := 0, 0, 1<<30, false
	zw := newZoneWalker(game.Point{X: bx, Y: by}, mineDefenseRadius, g.W, g.H)

	for {
		x, y := zw.x, zw.y

		if (x != u.GridX || y != u.GridY) && t.isMineSite(x, y, bx, by, ex, ey, hasThreat) && g.CanEnter(u, x, y) {
			if d := sim.SquaredDist(u.GridX, u.GridY, x, y); d < bestD {
				bestX, bestY, bestD, found = x, y, d, true
			}
		}

		if !zw.findNext() {
			break
		}
	}

	return bestX, bestY, found
}

// nearestSpottedEnemy returns the believed cell of the enemy nearest the base centre
// that the team has spotted, used as the threat direction the mine ring faces. With
// no enemy in sight it returns hasThreat=false (mines may then ring any side).
func (t *taskMines) nearestSpottedEnemy(bx, by int) (int, int, bool) {
	g := t.ctx.Game
	ex, ey, bestD, found := 0, 0, 1<<30, false

	for _, e := range g.Units.Units() {
		if e.Team == t.team || e.Team < 0 || e.Team >= game.TeamCount {
			continue
		}

		if e.GridX < 0 || e.GridY < 0 || !g.SpottedByTeam(e, t.team) {
			continue
		}

		tx, ty := believedCell(g, t.team, e)
		if d := sim.SquaredDist(bx, by, tx, ty); d < bestD {
			ex, ey, bestD, found = tx, ty, d, true
		}
	}

	return ex, ey, found
}

// markLaid records a freshly mined cell so the ring scan skips it afterwards.
func (t *taskMines) markLaid(x, y int) {
	if t.laid == nil {
		t.laid = make(map[[2]int]bool)
	}

	t.laid[[2]int{x, y}] = true
}

// teamBaseCenter returns the centroid of the team's buildings, the anchor of its
// defensive mine ring, or ok=false when the team owns no building. It counts
// stationary SELECTABLE units (buildings) so a laid mine - stationary but not
// selectable - never drags the centre as the ring fills.
func teamBaseCenter(g *sim.Game, team int) (int, int, bool) {
	sumX, sumY, n := 0, 0, 0

	for _, b := range g.Units.Units() {
		if b.Team != team || b.GridX < 0 || b.GridY < 0 {
			continue
		}

		if !b.Flags.Has(game.FlagStationary) || !b.Flags.Has(game.FlagSelectable) {
			continue
		}

		sumX += b.GridX
		sumY += b.GridY
		n++
	}

	if n == 0 {
		return 0, 0, false
	}

	return sumX / n, sumY / n, true
}
