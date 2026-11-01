// groups.go : implements the unit group system.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

// groups.go - C&C-style unit control groups. Alt+1..9 assigns the current
// selection to a group slot; the bare digit 1..9 recalls it (selects the whole
// roster and centres the camera on the leader). The slots hold unit ids in a
// []uint16 so a group carries a full multi unit selection. Assigned slots ride the
// save (v16). M.A.X. itself has no control groups; this is a quality of life
// addition modelled on C&C.

import (
	"fmt"

	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/saveload"
)

// setUnitGroup assigns the whole current selection to control group n (0..8).
// Membership is EXCLUSIVE: a unit belongs to only one numbered group, so the
// assigned units are first dropped from every other slot (classic single group
// behaviour).
func (s *Screen) setUnitGroup(n int) {
	if n < 0 || n >= len(s.unitGroups) {
		return
	}

	sel := s.selectionSet()
	if len(sel) == 0 {
		return
	}

	ids := make([]uint16, 0, len(sel))
	assigned := make(map[uint16]bool, len(sel))

	for _, u := range sel {
		ids = append(ids, u.ID)
		assigned[u.ID] = true
	}

	// Exclusive membership: remove the assigned units from any OTHER slot they were in.
	for i := range s.unitGroups {
		if i == n {
			continue
		}

		kept := s.unitGroups[i][:0]

		for _, id := range s.unitGroups[i] {
			if !assigned[id] {
				kept = append(kept, id)
			}
		}

		if len(kept) == 0 {
			s.unitGroups[i] = nil
		} else {
			s.unitGroups[i] = kept
		}
	}

	s.unitGroups[n] = ids

	if len(ids) == 1 {
		s.showMessage(fmt.Sprintf("Group %d set.", n+1))
	} else {
		s.showMessage(fmt.Sprintf("Group %d set (%d units).", n+1, len(ids)))
	}
}

// recallUnitGroup selects every surviving member of control group n (0..8) and
// centres the view on the leader (the first survivor). Destroyed members (gone
// from the registry) and ground cover are skipped; an empty or wiped out group
// does nothing.
func (s *Screen) recallUnitGroup(n int) {
	if n < 0 || n >= len(s.unitGroups) {
		return
	}

	var members []*game.UnitInfo

	for _, id := range s.unitGroups[n] {
		u := s.game.Units.ByID.Get(id)

		if u == nil || u.Flags.Has(game.FlagGroundCover) {
			continue
		}

		members = append(members, u)
	}

	if len(members) == 0 {
		return
	}

	s.setSelectionSet(members, members[0])
	s.centerOnSelected()
}

// captureUnitGroups copies the assigned control groups into save snapshot form;
// empty slots are skipped. Runs after Capture (the groups live on the screen, not
// the sim). C++: none - a port quality of life addition (see the camera bookmarks).
func (s *Screen) captureUnitGroups() []saveload.UnitGroupSnapshot {
	var out []saveload.UnitGroupSnapshot

	for i, ids := range s.unitGroups {
		if len(ids) == 0 {
			continue
		}

		out = append(out, saveload.UnitGroupSnapshot{Slot: i, UnitIDs: append([]uint16(nil), ids...)})
	}

	return out
}

// applyUnitGroups replaces the control groups with the ones a save carries. Slots
// the save lacks come back empty (a pre v16 save clears them all).
func (s *Screen) applyUnitGroups(groups []saveload.UnitGroupSnapshot) {
	s.unitGroups = [9][]uint16{}

	for _, g := range groups {
		if g.Slot < 0 || g.Slot >= len(s.unitGroups) {
			continue
		}

		s.unitGroups[g.Slot] = append([]uint16(nil), g.UnitIDs...)
	}
}

// captureDisplayToggles copies the nine display toggles into save snapshot form. They live
// on the screen, not the sim, so this runs after Capture, like the bookmarks and the groups.
// C++: saveload.cpp writes the display settings into the save file.
func (s *Screen) captureDisplayToggles() *saveload.DisplaySnapshot {
	return &saveload.DisplaySnapshot{
		Survey: s.surveyOverlay,
		Status: s.showStatus,
		Colors: s.showColors,
		Hits:   s.showHits,
		Ammo:   s.showAmmo,
		Range:  s.showRange,
		Scan:   s.showScan,
		Grid:   s.grid,
		Names:  s.showNames,
	}
}

// applyDisplayToggles restores the overlays a save was played with. A pre v20 save carries
// none, and then the toggles simply keep whatever the running screen already has - which is
// exactly what a load used to do to every save.
func (s *Screen) applyDisplayToggles(d *saveload.DisplaySnapshot) {
	if d == nil {
		return
	}

	s.surveyOverlay = d.Survey
	s.showStatus = d.Status
	s.showColors = d.Colors
	s.showHits = d.Hits
	s.showAmmo = d.Ammo
	s.showRange = d.Range
	s.showScan = d.Scan
	s.grid = d.Grid
	s.showNames = d.Names
}

// captureTurnClock records the seconds still on the running turn's clock. Another screen
// layer value: the sim knows whose turn it is, not how much of it is left. 0 means an
// untimed turn (or one already run out), which a load then leaves alone.
func (s *Screen) captureTurnClock() int {
	if s.turnFlow.turnSeconds <= 0 {
		return 0
	}

	return s.turnRemaining()
}

// applyTurnClock resumes a part spent turn. The remainder becomes THIS turn's budget and
// the clock restarts against it; the next turn takes the full budget back in
// goLiveForHuman. A pre v22 save carries 0 and keeps the old behaviour: a full fresh turn,
// which is a quiet time refund for anyone who saves when the clock runs low.
func (s *Screen) applyTurnClock(secondsLeft int) {
	if secondsLeft <= 0 || s.turnFlow.turnSeconds <= 0 {
		return
	}

	s.turnFlow.turnSeconds = secondsLeft
	s.turnFlow.turnStart = platform.Ticks()
	s.turnFlow.timeWarnedTurn = turnID{} // a loaded turn warns again on its remainder (armed again at the load turn start)
}
