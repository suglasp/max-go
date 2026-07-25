// airload.go : implements the air transport load animation.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// An AIR transport flies over its unit, descends, loads it, then rises again instead of using the land or sea pickup animation.
// It mirrors airunload.go and toggles FlagHovering to use the AirAlt altitude tween and its landing and takeoff sounds.
package gamescreen

import (
	"github.com/suglasp/max-go/internal/game"
)

// airLoadJob records one air transport load. The transport flies over the unit, descends, loads it, then rises.
// descending starts over the unit cell. loaded starts after the unit boards.
// armX/armY remember where the unit stood when the pick up was ordered: a unit
// that drives off abandons the job instead of dragging the transport on an
// endless chase (klei only loads a settled unit at the transport's own cell).
type airLoadJob struct {
	transport  *game.UnitInfo
	unit       *game.UnitInfo
	armX, armY int
	descending bool
	loaded     bool
}

// beginAirLoad starts an air transport flying over, descending, loading, and rising with unit.
// It sends the transport to the unit first when needed. Selecting it lets selection-gated AirAlt cues play.
func (s *Screen) beginAirLoad(transport, unit *game.UnitInfo) {
	if s.effects.airLoad != nil {
		return // one air load animation at a time
	}

	s.effects.airLoad = &airLoadJob{transport: transport, unit: unit, armX: unit.GridX, armY: unit.GridY}

	// Fly over the unit's cell unless the transport already hovers above it (air
	// units fly over everything, so the unit's cell is always reachable).
	if transport.GridX != unit.GridX || transport.GridY != unit.GridY {
		s.game.OrderMove(transport, unit.GridX, unit.GridY)
	}

	s.setSelected(transport)
}

/* // note: unused code in Go port
// cancelAirLoad drops a pending air load (the unit stays on the map).
func (s *Screen) cancelAirLoad() {
	s.effects.airLoad = nil
}
*/

// tickAirLoad advances an air transport load each idle frame.
// It waits for the transport, descends, loads the unit, rises, then clears the job at flying altitude.
func (s *Screen) tickAirLoad() {
	job := s.effects.airLoad

	if job == nil {
		return
	}

	t := job.transport

	// The transport was destroyed mid animation: abandon (the unit stays on the map).
	if s.game.Units.ByID.Get(t.ID) != t {
		s.effects.airLoad = nil
		return
	}

	// The unit to load was lost before pickup: abandon.
	if !job.loaded && s.game.Units.ByID.Get(job.unit.ID) != job.unit {
		s.effects.airLoad = nil
		return
	}

	// The unit drove off its arming cell: abandon the chase (the player re-issues
	// the load; klei only grabs a settled unit at the transport's own cell).
	if !job.loaded && (job.unit.GridX != job.armX || job.unit.GridY != job.armY) {
		s.effects.airLoad = nil
		return
	}

	if !job.descending {
		// Arrived over the unit's cell: start descending (the AirAlt tween + SfxLand follow).
		if t.GridX == job.unit.GridX && t.GridY == job.unit.GridY {
			t.Flags &^= game.FlagHovering
			job.descending = true
			return
		}

		// Still flying / queued to fly: wait it out. isMoving = moving this frame;
		// HasPendingRoute = ran out of moves but a route is shelved to resume next turn.
		if s.isMoving(t) || s.game.HasPendingRoute(t) {
			return
		}

		// A transport can stop short with leftover Speed and no shelved route because over-budget shelving occurs only at Speed == 0.
		// Reissue the reachable flight whenever movement budget is available; otherwise wait for the next turn's refill.
		if t.Speed > 0 {
			s.game.OrderMove(t, job.unit.GridX, job.unit.GridY)
		}

		return
	}

	if !job.loaded {
		// Still descending: the AirAlt tween animates the shadow + plays SfxLand.
		if t.AirAlt > 0 {
			return
		}

		// Touched down over the unit: the pack up cue, then take it aboard and rise.
		s.playSfxForUnit(t, game.SfxShrink)

		if s.game.PickUpUnit(t, job.unit) {
			s.showMessage(displayName(job.unit.Type) + " loaded.")
		} else {
			s.showInfo("Cannot load - the transport has no room for that unit.")
		}

		job.loaded = true
		t.Flags |= game.FlagHovering // rise again (the AirAlt tween + SfxTake follow)
		s.game.RefreshVisibility()
		return
	}

	// Risen back to flying altitude: the load is complete.
	if t.AirAlt >= game.AirAltMax {
		s.effects.airLoad = nil
	}
}
