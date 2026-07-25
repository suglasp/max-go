// task_turrets.go : M.A.X. AI fire stationary turrets at the nearest enemy in range
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import (
	"github.com/suglasp/max-go/internal/game"
)

// taskFireTurrets is the fire pass run each turn for stationary armed structures:
// GunTurrt, AntiAir, ArtyTrrt, AntiMssl and any other building with ATTRIB_ATTACK
// > 0. Each turn it picks the nearest targetable enemy already within attack range
// and fires. If no enemy is in range the turret holds; stationary units cannot
// advance. The task does not reschedule itself; TaskManager.BeginTurn queues it
// again each turn alongside taskAttackEnemies.
//
// C++ ported : aiplayer.cpp - AiPlayer::CheckAttacks (the fire loop for stationary units)
type taskFireTurrets struct {
	TaskBase
	ctx *aiContext
}

func newTaskFireTurrets(team int, ctx *aiContext) *taskFireTurrets {
	return &taskFireTurrets{
		TaskBase: newTaskBase(team, nil, 800), // same priority bracket as taskAttackEnemies
		ctx:      ctx,
	}
}

// BeginTurn fires each ready stationary armed unit at the nearest enemy in range.
// C++: AiPlayer::CheckAttacks iterating UnitsManager_StationaryUnits and calling
// AiAttack_EvaluateAttack, reduced to the case of firing when a target is in range
// This implementation omits reaction delays and damage-potential gating.
func (t *taskFireTurrets) BeginTurn() {
	g := t.ctx.Game
	if g == nil {
		return
	}

	for _, u := range g.Units.Units() {
		if !t.isReadyTurret(u) {
			continue
		}

		target := t.nearestInRangeEnemy(u)

		if target == nil {
			continue // no enemy in range: hold position
		}

		if g.CanAttack(u, target.GridX, target.GridY) {
			g.Attack(u, target.GridX, target.GridY)
		}
	}
}

// isReadyTurret reports whether u is one of this team's stationary armed units
// that has shots and ammo left this turn.
func (t *taskFireTurrets) isReadyTurret(u *game.UnitInfo) bool {
	if u.Team != t.team || u.Values == nil {
		return false
	}

	// Stationary structures have FlagStationary set; mobile units do not.
	if !u.Flags.Has(game.FlagStationary) {
		return false
	}

	if u.Values.Get(game.AttribAttack) <= 0 {
		return false // unarmed building: power plant, factory, etc.
	}

	if u.Shots <= 0 || u.Ammo <= 0 {
		return false // out of ammo this turn
	}

	return true
}

// nearestInRangeEnemy returns the nearest enemy that is already within the
// turret's attack range, or nil. Because turrets cannot move, we only fire if
// the target is reachable right now, do not pursue enemies out of range.
// C++: AiAttack_EvaluateAttack calls AiAttack_SelectTargetToAttack with the
// unit's ATTRIB_RANGE; we mirror that by asking g.CanAttack after confirming
// the target is the closest one within range.
func (t *taskFireTurrets) nearestInRangeEnemy(u *game.UnitInfo) *game.UnitInfo {
	g := t.ctx.Game
	var best *game.UnitInfo
	bestD := 1 << 30

	for _, e := range g.Units.Units() {
		if e.Team == u.Team || e.Team < 0 || e.Team >= game.TeamCount {
			continue
		}

		if e.GridX < 0 || e.GridY < 0 {
			continue
		}

		// Only pursue enemies this turret's weapon can engage by type.
		if !g.CanTargetUnit(u, e) {
			continue
		}

		// Only fire at an enemy the team has actually spotted; a turret never
		// shoots into the fog (it still sees anything genuinely inside its own
		// scan coverage). C++ ported : spottedunit.cpp - the AI fires only at
		// units in spotted_units.
		if g.Vis != nil && !g.Vis.VisibleToTeam(e, t.team) {
			continue
		}

		// Only consider enemies actually within firing range right now.
		if !g.CanAttack(u, e.GridX, e.GridY) {
			continue
		}

		if d := chebyshev(u.GridX, u.GridY, e.GridX, e.GridY); d < bestD {
			best, bestD = e, d
		}
	}

	return best
}
