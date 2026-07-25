// task_commando.go : M.A.X. AI commanding.
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

// taskCommando drives the team's idle commandos to infiltrate the enemy: each turn
// it sends every free commando toward the highest value spotted enemy electronic
// target and, once adjacent, STEALS it (a mobile unit, captured onto our team) or
// DISABLES it (a building, or anything steal rejects, shut down for a few turns). A
// commando that has to cross ground first is driven adjacent and acts next turn. It
// does not reschedule itself; TaskManager queues it again each turn.
//
// This dedicated task sends commandos directly to high-value electronic targets. It uses normal pathing, targets no organic infantry or commandos,
// and leaves experience-scaled success and exposure handling to the simulation.
//
// C++ ported : aiplayer.cpp - DetermineAttack (the commando as a high value target
// attacker) + units_manager.cpp - GameManager_DisableUnit / StealUnit (the resolve).
type taskCommando struct {
	TaskBase
	ctx *aiContext
}

func newTaskCommando(team int, ctx *aiContext) *taskCommando {
	return &taskCommando{
		TaskBase: newTaskBase(team, nil, 750), // above the gun line attack task (800)
		ctx:      ctx,
	}
}

// BeginTurn infiltrates with each free commando: act if already adjacent to its
// target, otherwise drive adjacent. C++: the assignment of a commando to a high
// value target each turn, then the stealth action on contact.
func (t *taskCommando) BeginTurn() {
	g := t.ctx.Game

	if g == nil {
		return
	}

	for _, u := range g.Units.Units() {
		if !t.isReadyCommando(u) {
			continue
		}

		target := t.bestInfiltrationTarget(u)

		if target == nil {
			continue
		}

		tx, ty := believedCell(g, t.team, target)

		// Adjacent and eligible: steal a mobile unit (we GAIN it) when possible, else
		// disable (a building, or anything steal rejects). The seam spends the shot.
		if g.CanStealAt(u, tx, ty) {
			ok, exposed := g.Steal(u, tx, ty)
			dbg.Printf("ai: team %d COMMANDO steal %s at (%d,%d): ok=%v exposed=%v", t.team, target.Type.Tag(), tx, ty, ok, exposed)

			continue
		}

		if g.CanDisableAt(u, tx, ty) {
			ok, exposed := g.Disable(u, tx, ty)
			dbg.Printf("ai: team %d COMMANDO disable %s at (%d,%d): ok=%v exposed=%v", t.team, target.Type.Tag(), tx, ty, ok, exposed)

			continue
		}

		// Not in reach yet: close to a cell touching the target's footprint, and act
		// next turn (claiming the unit so the attack task leaves it alone).
		for _, c := range g.AdjacentApproachCells(u, tx, ty) {
			if g.OrderMove(u, c.X, c.Y) {
				dbg.Printf("ai: team %d COMMANDO approaches %s at (%d,%d)", t.team, target.Type.Tag(), tx, ty)

				break
			}
		}
	}
}

// isReadyCommando reports whether u is one of this team's commandos free to act this
// turn: it is a commando with a shot and movement budget left and no order in flight.
func (t *taskCommando) isReadyCommando(u *game.UnitInfo) bool {
	return sim.IsCommando(u) && u.Team == t.team && u.Shots > 0 && u.Speed > 0 && !t.ctx.Game.HasOrder(u)
}

// bestInfiltrationTarget returns the highest value SPOTTED enemy electronic unit for
// u to infiltrate, or nil. It ranks by (priority, distance to the believed cell):
// priority 0 = a key facility (Eco Sphere, mining, or plant - disabling these denies
// the enemy economy), priority 1 = any other electronic unit (a tank to steal). It
// skips organic units (cannot be infiltrated), airborne units (out of reach), and
// anything the team has not spotted (not omniscient, the same gate the attack task
// uses). C++: the high value target selection the original's DetermineAttack applies
// to a commando (weighted by IsKeyFacility), reduced to nearest of the best class.
func (t *taskCommando) bestInfiltrationTarget(u *game.UnitInfo) *game.UnitInfo {
	g := t.ctx.Game
	var best *game.UnitInfo
	bestPri := 2
	bestD := 1 << 30

	for _, e := range g.Units.Units() {
		if e.Team == u.Team || e.Team < 0 || e.Team >= game.TeamCount {
			continue
		}

		if e.GridX < 0 || e.GridY < 0 {
			continue
		}

		if !e.Flags.Has(game.FlagElectronicUnit) {
			continue // organic units cannot be disabled or stolen
		}

		if e.Flags.Has(game.FlagMobileAirUnit) && e.Flags.Has(game.FlagHovering) {
			continue // an airborne unit is out of a foot commando's reach
		}

		if !g.SpottedByTeam(e, t.team) {
			continue // never seen: the team cannot send a commando after it
		}

		tx, ty := believedCell(g, t.team, e)
		pri := 1
		if isKeyFacility(e.Type) {
			pri = 0
		}

		d := chebyshev(u.GridX, u.GridY, tx, ty)
		if pri < bestPri || (pri == bestPri && d < bestD) {
			best, bestPri, bestD = e, pri, d
		}
	}

	return best
}

// believedCell returns the cell the team believes enemy e occupies: its true cell
// while currently visible, otherwise the frozen last seen cell from the spotted
// memory (so a unit acting on a fogged enemy aims at where it was LAST seen, not its
// true position). Falls back to the live cell with no fog memory (a test harness with
// a nil Vis). It mirrors taskAttackEnemies.targetCell for the special unit tasks.
// C++: SpottedUnit::GetLastPosition.
func believedCell(g *sim.Game, team int, e *game.UnitInfo) (int, int) {
	if g.Vis != nil && !g.Vis.VisibleToTeam(e, team) {
		if x, y, ok := g.LastSeen(team, e); ok {
			return x, y
		}
	}

	return e.GridX, e.GridY
}
