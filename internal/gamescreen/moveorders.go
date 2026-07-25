// moveorders.go : turns a player's move click into a routed order, mine warning included.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
)

// canTakeMoveOrder reports whether u is mobile and not in the transient build pose. u.Building does not mean the unit is a structure.
// C++: ORDER_MOVE is only issued for a unit with a movement flag.
func canTakeMoveOrder(u *game.UnitInfo) bool {
	return u != nil && isMobileUnit(u) && !u.Building
}

// orderMoveTo routes an active-team unit to a cell through sim.OrderMove, replacing any existing route and spending its turn movement.
func (s *Screen) orderMoveTo(u *game.UnitInfo, cellX, cellY int) {
	if !canTakeMoveOrder(u) { // choke point for every move caller (click, minimap, drive onto)
		return
	}

	if u.Team != s.game.State.ActiveTeam {
		return
	}

	// The player's selected unit shows a brief planning bulb before moving. Other movers begin immediately.
	if u == s.selected {
		s.armPlanningBulb(u, cellX, cellY)
		return
	}

	s.executeMove(u, cellX, cellY)
}

// executeMove issues OrderMove while movement remains, otherwise QueueMove for the next turn. A missing path produces no cue.
// C++: ORDER_MOVE / a 0-speed unit keeps its GroundPath and defers to next turn.
func (s *Screen) executeMove(u *game.UnitInfo, cellX, cellY int) {
	s.warnOnKnownMine(u, cellX, cellY)

	if u.Speed <= 0 {
		if s.game.QueueMove(u, cellX, cellY) {
			dbg.Printf("queue move %s (%d,%d) -> (%d,%d): no points this turn, resumes next", displayName(u.Type), u.GridX, u.GridY, cellX, cellY)
			s.showMessage("No movement points left - the unit will move next turn.")
		} else {
			s.playVoice(vsNoPath) // unreachable: same cue as an in range no path
			dbg.Printf("queue move %s (%d,%d) - no path to (%d,%d)", displayName(u.Type), u.GridX, u.GridY, cellX, cellY)
		}

		return
	}

	if s.game.OrderMove(u, cellX, cellY) {
		dbg.Printf("move %s (%d,%d) to (%d,%d)", displayName(u.Type), u.GridX, u.GridY, cellX, cellY)
	} else {
		s.playVoice(vsNoPath) // "No path to destination!"
		dbg.Printf("move %s (%d,%d) - not possible (%d,%d): no path", displayName(u.Type), u.GridX, u.GridY, cellX, cellY)
	}
}

// warnOnKnownMine reports when a player orders a unit onto a discovered mine without blocking the order. This port addition applies only to the player.
func (s *Screen) warnOnKnownMine(u *game.UnitInfo, cellX, cellY int) {
	if s.game.State == nil || u.Team != s.game.State.PlayerTeam {
		return
	}

	for _, other := range s.game.Units.Cells.At(cellX, cellY) {
		if sim.IsMine(other) && other.Team != u.Team && other.SpottedByTeam(u.Team) {
			s.showMessage("Warning: enemy mine on that cell.")
			return
		}
	}
}
