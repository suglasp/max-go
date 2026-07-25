// airunload.go : implements the air transport unload animation.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"github.com/suglasp/max-go/internal/game"
)

// An AIR transport descends in place, drops its unit directly below, then rises again instead of using the land or sea unload animation.
// It toggles FlagHovering to use the AirAlt tween. It does not use LandAir because unloads may touch down anywhere.
// C++ ported : units_manager.cpp - UnitsManager_ProgressUnloading + unitinfo.cpp - UnitInfo::Land / Take / ProcessUnloading.

// airUnloadJob records one air transport unload. The transport descends, drops the unit at AirAlt 0, then rises again.
type airUnloadJob struct {
	transport *game.UnitInfo
	unit      *game.UnitInfo
	dropped   bool
}

// beginAirUnload starts an air transport descending, dropping unit, then rising again.
// Clearing FlagHovering starts the AirAlt tween and SfxLand. The unit remains aboard until touchdown.
func (s *Screen) beginAirUnload(transport, unit *game.UnitInfo) {
	if s.effects.airUnload != nil {
		return // one unload animation at a time
	}

	s.effects.airUnload = &airUnloadJob{transport: transport, unit: unit}
	transport.Flags &^= game.FlagHovering // start descending (the AirAlt tween + SfxLand follow)
	s.setSelected(transport)
	s.game.RefreshVisibility()
}

/* // note: unused code in Go port
// cancelAirUnload drops a pending air unload (the unit stays aboard).
func (s *Screen) cancelAirUnload() {
	s.effects.airUnload = nil
}
*/

// tickAirUnload advances an air transport unload each idle frame.
// It descends, drops and grows in the unit, rises, then clears the job at flying altitude.
func (s *Screen) tickAirUnload() {
	job := s.effects.airUnload

	if job == nil {
		return
	}

	t := job.transport

	// The transport was destroyed mid animation: abandon (the unit stays carried).
	if s.game.Units.ByID.Get(t.ID) != t {
		s.effects.airUnload = nil
		return
	}

	if !job.dropped {
		// Still descending: the AirAlt tween animates the shadow + plays SfxLand.
		if t.AirAlt > 0 {
			return
		}

		// Touched down: the cue for setting down, then drop the unit beneath and start rising.
		s.playSfxForUnit(t, game.SfxPowerEnd)

		if s.game.DropUnloadedUnit(t, job.unit) {
			s.showMessage(displayName(job.unit.Type) + " unloaded.")
		} else {
			s.showMessage("Cannot unload here - no room beneath the transport.")
		}

		job.dropped = true
		t.Flags |= game.FlagHovering // rise again (the AirAlt tween + SfxTake follow)
		s.game.RefreshVisibility()
		return
	}

	// Risen back to flying altitude: the unload is complete.
	if t.AirAlt >= game.AirAltMax {
		s.effects.airUnload = nil
	}
}
