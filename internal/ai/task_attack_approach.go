// task_attack_approach.go : drives an attacker into a stand off firing position and
// routes it around defended ground.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : aiattack.cpp - AiAttack_ChooseSiteForAttacker + TaskMove
package ai

import (
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
)

// advanceToward moves u to a reachable stand off cell from which it can fire on (tx, ty)
// at its weapon range, so a long range unit kites instead of walking into melee. It
// issues the first move that paths (preferring the firing cell nearest u = maximum stand
// off), falling back to an adjacent cell when no in range cell is reachable. Already in
// range: it holds (firing is the caller's job). C++: TaskMove + AiAttack_ChooseSiteForAttacker.
func (t *taskAttackEnemies) advanceToward(u *game.UnitInfo, tx, ty int) bool {
	g := t.ctx.Game
	rng := u.Values.Get(game.AttribRange)

	if rng < 1 {
		rng = 1
	}

	// Already within weapon range of the believed cell: hold and let the caller
	// fire. C++: the distance3 <= range*range "site = current cell" early exit.
	if sq := sim.SquaredDist(u.GridX, u.GridY, tx, ty); sq <= rng*rng {
		return false
	}

	cells := g.FiringApproachCells(u, tx, ty, rng)

	// With a threat map, prefer the SAFEST reachable firing cell (least incoming
	// enemy fire), with ties broken by nearest - the unit stands off where it takes
	// the least return fire instead of walking into a kill zone. C++:
	// AiAttack_ChooseSiteForAttacker scores sites by damage_potential.
	if t.tm != nil {
		sortApproachSafest(cells, t.tm)
	}

	// Route to the chosen stand off / approach cell with the threat map as a danger
	// penalty, so the path bends AROUND defended ground instead of driving straight
	// through the kill zone to reach a safe firing cell (nil before the team commits,
	// which is the plain terrain route).
	pen := dangerPenalty(t.tm)

	for _, c := range cells {
		if g.OrderMoveAvoiding(u, c.X, c.Y, pen) {
			return true
		}
	}

	// No reachable firing cell: close to an adjacent cell as before.
	for _, c := range g.AdjacentApproachCells(u, tx, ty) {
		if g.OrderMoveAvoiding(u, c.X, c.Y, pen) {
			return true
		}
	}

	return false
}

// dangerRouteWeight scales the threat map into the route penalty for OrderMoveAvoiding:
// a cell's incoming firepower times this weight adds to its terrain cost during A*. Sized
// near a normal land step (access.Clear = 4), so one covering enemy adds ~one cell of
// detour and a kill zone strongly repels, without making a reachable target unreachable.
// A package var so a test can pin the weight.
var dangerRouteWeight = 4

// dangerPenalty builds the per cell route penalty from a threat map for
// Game.OrderMoveAvoiding: the summed incoming enemy firepower at a cell times
// dangerRouteWeight. It returns nil for a nil threat map (before the team commits, or
// a bare test harness), so the move falls back to the plain terrain route.
func dangerPenalty(tm *threatMap) func(x, y int) int {
	if tm == nil {
		return nil
	}

	return func(x, y int) int {
		return tm.at(x, y) * dangerRouteWeight
	}
}

// The approach cell search (ApproachCell, FiringApproachCells, AdjacentApproachCells,
// SortApproach, SquaredDist) lives in internal/sim now, shared with sim combat; the AI
// calls it through t.ctx.Game. Only the threat aware ordering below is AI specific.

// sortApproachSafest orders candidates by ascending threat (least incoming enemy
// fire at the cell), then ascending travel cost (insertion sort; lists are short).
func sortApproachSafest(cands []sim.ApproachCell, tm *threatMap) {
	for i := 1; i < len(cands); i++ {
		for j := i; j > 0; j-- {
			tj, tp := tm.at(cands[j].X, cands[j].Y), tm.at(cands[j-1].X, cands[j-1].Y)

			if tj < tp || (tj == tp && cands[j].D < cands[j-1].D) {
				cands[j], cands[j-1] = cands[j-1], cands[j]
			} else {
				break
			}
		}
	}
}
