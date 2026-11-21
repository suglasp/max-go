// movefinished.go : answers a computer unit that a halt stopped mid turn.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ reference: taskmove.cpp, TaskMove::Execute; unitinfo.cpp, UnitInfo::MoveFinished.
package ai

import (
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
)

// MoveFinished lets a computer unit that an enemy sighting halted act in the same turn: it fires on an enemy in
// range, else drives on along its held route with the speed it has left.
//
// Reductions: the reference also weighs a reserve shot caution rule and re-checks the next waypoint of the task; here
// the held route is the task's planned path and the unit drives on to its destination.
//
// C++ ported : task.cpp - Task_RemindMoveFinished (UnitInfo::MoveFinished runs the unit's TaskMove::Execute again)
func (a *AI) MoveFinished(g *sim.Game, u *game.UnitInfo) {
	// Nothing to answer for a missing unit, a human team, a silenced AI or an AI that has not planned yet.
	if u == nil || u.Team < 0 || u.Team >= game.TeamCount || g.State.Teams[u.Team].Type != game.TeamComputer {
		return
	}

	if a.disabled || !a.inited {
		return
	}

	// A dead, building or disabled unit cannot act.
	if u.Hits <= 0 || u.Building || g.IsDisabled(u) {
		return
	}

	// An enemy in range: fire first and keep the held route, as the fire or move rule of the attack task.
	if e := engageableEnemy(g, u.Team, u); e != nil {
		ex, ey := believedCell(g, u.Team, e)

		if _, fired := g.Attack(u, ex, ey); fired {
			dbg.Logf(dbg.AI, "team %d %s at (%d,%d) fires after its halt at %s (%d,%d)", u.Team, u.Type.Tag(), u.GridX, u.GridY, e.Type.Tag(), ex, ey)

			return
		}
	}

	// No shot taken: drive on toward the held destination while moves are left.
	if g.HeldRoute(u) != nil && u.Speed > 0 {
		if g.ContinueHeldRoute(u) {
			dbg.Logf(dbg.AI, "team %d %s at (%d,%d) drives on after its halt", u.Team, u.Type.Tag(), u.GridX, u.GridY)
		}
	}
}
