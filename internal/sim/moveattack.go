// moveattack.go : drives an attacker into weapon range and fires.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : unitinfo.cpp - OrderMoveToAttack execution
package sim

import (
	"github.com/suglasp/max-go/internal/combat"
	"github.com/suglasp/max-go/internal/game"
)

// OrderMoveToAttack issues a combined move then fire order against the enemy at
// (cellX, cellY): if the target is already in weapon range the unit fires now,
// otherwise it drives to a standoff cell within range and fires on arrival,
// pursuing the enemy across turns if it relocates. It reports whether the order
// was accepted (a valid out of range enemy with a mobile, armed attacker, or an
// immediate in range shot). The far target cursor (PTR_FTRG) maps to this click.
//
// C++ ported : game_manager.cpp - GameManager_SetUnitOrder (ORDER_MOVE_TO_ATTACK)
// + units_manager.cpp - UnitsManager_PursueEnemy.
func (g *Game) OrderMoveToAttack(attacker *game.UnitInfo, cellX, cellY int) bool {
	if attacker == nil || attacker.Values == nil {
		return false
	}
	target := g.topTarget(attacker, cellX, cellY)
	if target == nil || !combat.CanTarget(attacker, target, g.Grid.Surf) {
		return false
	}
	// In range already: fire on the spot (this also covers the in range enemy click,
	// which vanilla routes through ORDER_MOVE_TO_ATTACK with ORDER_STATE_EXECUTING_ORDER).
	if combat.CanAttack(attacker, target, g.Grid.Surf) {
		g.Attack(attacker, cellX, cellY)
		return true
	}
	// Out of range: only a mobile, armed attacker can close the distance.
	if !g.CanFarTarget(attacker, cellX, cellY) {
		return false
	}
	attacker.AttackTarget = target
	attacker.SetOrder(game.OrderMoveToAttack)
	g.advancePursuit(attacker) // start moving now (or queue if out of movement this turn)
	return true
}

// advancePursuit is one step of a move to attack order (port of PursueEnemy): it
// fires when the target is in range, otherwise paths again toward the target's current
// position. It runs on the initial order, on each move completion (arrival), and at
// the start of the unit's team turn (resume across turns). It reports whether a fresh
// move was started. A spent, dead, gone or unreachable target ends the pursuit.
//
// C++ ported : units_manager.cpp - UnitsManager_PursueEnemy.
func (g *Game) advancePursuit(unit *game.UnitInfo) bool {
	target := unit.AttackTarget
	if target == nil {
		return false
	}
	// The target died or left the map: drop the pursuit.
	if target.Hits <= 0 || target.GridX < 0 || target.GridY < 0 {
		unit.AttackTarget = nil
		return false
	}
	// Out of ammo for good (not just shots spent this turn): cannot fire even after
	// closing, so give up. A reload is the player's job. C++: PursueEnemy ammo>0 gate.
	if unit.Ammo <= 0 {
		unit.AttackTarget = nil
		return false
	}
	// In weapon range with a shot ready: fire and finish the order.
	if combat.CanAttack(unit, target, g.Grid.Surf) {
		g.Attack(unit, target.GridX, target.GridY)
		unit.AttackTarget = nil
		return false
	}
	// In range but out of shots this turn (the move/fire budget was spent driving here):
	// hold the target; next turn's refill lets it fire. C++: the shared budget stall.
	if combat.InRange(unit, target.GridX, target.GridY, unit.Values.Get(game.AttribRange)) {
		return false
	}
	// Still out of range: drive to the nearest reachable firing cell (a standoff at
	// weapon range), falling back to closing to an adjacent cell.
	rng := unit.Values.Get(game.AttribRange)
	if rng < 1 {
		rng = 1
	}
	for _, c := range g.FiringApproachCells(unit, target.GridX, target.GridY, rng) {
		if g.orderMoveTo(unit, c.X, c.Y) {
			return true
		}
	}
	for _, c := range g.AdjacentApproachCells(unit, target.GridX, target.GridY) {
		if g.orderMoveTo(unit, c.X, c.Y) {
			return true
		}
	}
	return false // no reachable approach: stay put, keep the target for a later turn
}

// SetMoveDelay holds a unit's fresh move order still for a few animation frames before it
// pulls out. A group move staggers its members this way, so a column leaves in order instead
// of the whole formation lurching off on the same frame. It costs no movement budget, and a
// fast (headless) move ignores it. No op if the unit is not moving.
func (g *Game) SetMoveDelay(u *game.UnitInfo, ticks int) {
	if u == nil || ticks <= 0 {
		return
	}

	for _, o := range g.orders {
		if o.Unit == u {
			o.Delay = ticks

			return
		}
	}
}

// AttackOnArrival queues a shot for the moment unit's CURRENT drive ends: the unit fires
// when it PARKS, during its animated move, instead of at the next turn's planning pass.
// The caller keeps its own routing, which is why this exists next to OrderMoveToAttack -
// the computer opponent routes around the threat map, and handing it to advancePursuit
// would throw that away. A dead target, a spent shot budget or a target still out of
// range simply drops the shot; nothing here can fire early or fire twice.
//
// C++ ported : units_manager.cpp - the ORDER_MOVE_TO_ATTACK arrival shot.
func (g *Game) AttackOnArrival(unit, target *game.UnitInfo) {
	if unit == nil || target == nil {
		return
	}

	if g.arrivalShots == nil {
		g.arrivalShots = make(map[*game.UnitInfo]*game.UnitInfo)
	}

	g.arrivalShots[unit] = target
}

// fireArrivalShot takes the shot a unit queued with AttackOnArrival, now that its drive is
// over. Attack itself re checks range, shots, ammo and the target, so a unit that spent its
// whole budget driving simply does not fire (it shoots next turn, as before).
func (g *Game) fireArrivalShot(unit *game.UnitInfo) {
	target, queued := g.arrivalShots[unit]
	if !queued {
		return
	}

	delete(g.arrivalShots, unit)

	if unit.Hits <= 0 || target.Hits <= 0 {
		return // one of them died on the way in
	}

	g.Attack(unit, target.GridX, target.GridY)
}

// clearArrivalShots drops team's queued arrival shots at its turn start: a shot that was
// never taken belongs to the turn that planned it, and the team re plans from scratch now.
func (g *Game) clearArrivalShots(team int) {
	for u := range g.arrivalShots {
		if u.Team == team {
			delete(g.arrivalShots, u)
		}
	}
}

// resumePursuit runs advancePursuit again for each of team's units still pursuing a
// target, so a chase that did not reach weapon range last turn continues now that
// movement and shots have refilled. Called from BeginTeamTurn after resumePending.
func (g *Game) resumePursuit(team int) {
	for _, u := range g.Units.Units() {
		if u.Team == team && u.AttackTarget != nil {
			g.advancePursuit(u)
		}
	}
}

// dropPursuitOf clears the AttackTarget of every unit pursuing victim, called when
// victim is removed so no unit keeps a dangling pursuit pointer.
func (g *Game) dropPursuitOf(victim *game.UnitInfo) {
	for _, u := range g.Units.Units() {
		if u.AttackTarget == victim {
			u.AttackTarget = nil
		}
	}
}

// ApproachCell is a candidate move destination ranked by squared distance from the
// unit. Shared by sim combat and the AI attack/retreat tasks (ai imports sim, so it
// folds onto this one copy rather than carrying its own).
type ApproachCell struct {
	X, Y, D int
}

// FiringApproachCells lists the enterable cells from which unit could fire on a
// target at (tx, ty) - the cells inside the circular weapon range disc (dx^2 +
// dy^2 <= range^2, matching combat.InRange) - nearest to unit first, so the first
// that paths is the maximum standoff (least movement spent). Shared by the player's
// combined order and the AI attack task. C++ ported : aiattack.cpp -
// AiAttack_ChooseSiteForAttacker / units_manager.cpp - UnitsManager_GetAttackPosition.
func (g *Game) FiringApproachCells(unit *game.UnitInfo, tx, ty, rng int) []ApproachCell {
	var cands []ApproachCell
	for dy := -rng; dy <= rng; dy++ {
		for dx := -rng; dx <= rng; dx++ {
			if dx == 0 && dy == 0 {
				continue // the target's own cell
			}
			if dx*dx+dy*dy > rng*rng {
				continue // outside the circular weapon range disc
			}
			x, y := tx+dx, ty+dy
			if !g.CanEnter(unit, x, y) {
				continue
			}
			cands = append(cands, ApproachCell{x, y, SquaredDist(unit.GridX, unit.GridY, x, y)})
		}
	}
	SortApproach(cands)
	return cands
}

// AdjacentApproachCells lists the enterable cells touching (tx, ty), nearest to unit
// first - the close to contact fallback when no standoff firing cell paths.
func (g *Game) AdjacentApproachCells(unit *game.UnitInfo, tx, ty int) []ApproachCell {
	var cands []ApproachCell
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			x, y := tx+dx, ty+dy
			if !g.CanEnter(unit, x, y) {
				continue
			}
			cands = append(cands, ApproachCell{x, y, SquaredDist(unit.GridX, unit.GridY, x, y)})
		}
	}
	SortApproach(cands)
	return cands
}

// SortApproach orders candidates by ascending squared distance (insertion sort; the
// candidate lists are short).
func SortApproach(cands []ApproachCell) {
	for i := 1; i < len(cands); i++ {
		for j := i; j > 0 && cands[j].D < cands[j-1].D; j-- {
			cands[j], cands[j-1] = cands[j-1], cands[j]
		}
	}
}

// SquaredDist is the squared straight line grid distance, the metric used to rank
// approach cells. C++: Access_GetSquaredDistance.
func SquaredDist(ax, ay, bx, by int) int {
	dx, dy := ax-bx, ay-by
	return dx*dx + dy*dy
}
