// planningbulb.go : implements the planning mode light bulb toggle.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

// planningbulb.go - the BULB "thinking" overlay: a brief lightbulb flashed over the
// player's own SELECTED unit while it plans the path it was just ordered to walk.
// Vanilla sequence (live DOS check): the bulb appears (the unit "thinks", standing
// still), the bulb goes, THEN the unit moves - so the move is DEFERRED behind the
// bulb's dwell, never drawn while the unit drives off. The original showed the BULB
// only while the path was being computed; on a 486 that took visible time, but the
// port's pathfinding is synchronous (instant), so a short fixed dwell recreates the
// beat. C++ ported : drawmap.cpp - DrawMap_RenderWaitBulb (the BULB drawn for a player
// unit in ORDER_STATE_NEW_ORDER / MOVE_GETTING_PATH with no path yet).

import (
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/platform"
)

// bulbDwellMS is how long the "thinking" bulb shows before the deferred move starts -
// a brief flash, kept short because the move waits for it. Tunable feel.
const bulbDwellMS = 150

// armPlanningBulb shows the BULB over the selected unit and stashes the move it was
// just ordered to make, to start once the bulb clears (tickPlanningBulb). Only the
// player's own SELECTED unit shows the bulb; the caller routes any other mover
// straight to executeMove.
func (s *Screen) armPlanningBulb(u *game.UnitInfo, cellX, cellY int) {
	// Defensive: only the selected unit shows the bulb (the caller already checks).
	if u == nil || u != s.selected {
		return
	}

	s.effects.bulbUnit = u
	s.effects.bulbDue = platform.Ticks() + bulbDwellMS
	s.effects.bulbMoveX, s.effects.bulbMoveY = cellX, cellY
}

// cancelPlanningBulb drops a pending deferred move and its bulb - used when the player
// issues a different order to the unit before the dwell elapses (so the stale move
// does not fire after).
func (s *Screen) cancelPlanningBulb() {
	s.effects.bulbUnit = nil
}

// tickPlanningBulb starts the deferred move once the bulb has shown for its dwell, then
// clears the bulb - the unit visibly thinks, the bulb goes, THEN it moves (vanilla
// sequence). The move is committed when the player clicked, so it fires regardless of a
// later selection change; a unit that died during the dwell drops its move.
func (s *Screen) tickPlanningBulb() {
	// Nothing pending, or still "thinking".
	if s.effects.bulbUnit == nil || platform.Ticks() < s.effects.bulbDue {
		return
	}

	u := s.effects.bulbUnit
	cellX, cellY := s.effects.bulbMoveX, s.effects.bulbMoveY
	s.effects.bulbUnit = nil // the bulb goes...

	// ...then the unit moves, unless it was destroyed (removed from play) during the
	// dwell - a destroyed unit is unregistered from the unit hash.
	if s.game.Units.ByID.Get(u.ID) == u {
		s.executeMove(u, cellX, cellY)
	}
}

// drawPlanningBulb blits the BULB over the selected unit while its dwell is live (the
// unit is still standing still, so it anchors cleanly on the unit's cell). The move
// itself fires from tickPlanningBulb, so the bulb is gone before the unit drives off.
// C++ ported : drawmap.cpp - DrawMap_RenderWaitBulb.
func (s *Screen) drawPlanningBulb(view *gfx.Buffer) {
	// Nothing to draw without the sprite or an armed unit.
	if s.effects.bulbFrame == nil || s.effects.bulbUnit == nil {
		return
	}

	// Draw only while the dwell is live and the unit is still the selection (a unit
	// removed mid dwell is cleared by tickPlanningBulb before the next draw).
	if platform.Ticks() >= s.effects.bulbDue || s.effects.bulbUnit != s.selected {
		return
	}

	// Centre the bulb over the unit's cell, nudged up so it reads as a bubble above the
	// unit rather than hidden behind its sprite. Tracks the map zoom like the markers.
	z := s.r.Zoom()
	u := s.effects.bulbUnit
	sx := u.GridX*z + z/2 - s.camera.scrollX
	sy := u.GridY*z + z/4 - s.camera.scrollY
	s.effects.bulbFrame.DrawScaledCentered(view, sx, sy, z, 64, nil)
}
