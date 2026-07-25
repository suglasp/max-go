// groups_test.go : tests the numbered 1-9 unit control groups.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/saveload"
)

// TestUnitGroupStoresFullSelection: assigning a group records every selected unit's
// id, not just the leader.
func TestUnitGroupStoresFullSelection(t *testing.T) {
	s, g := newClickScreen(t)
	a := mkMobile(g, 0, 1, 3, 3)
	b := mkMobile(g, 0, 2, 4, 4)
	c := mkMobile(g, 0, 3, 5, 5)

	s.setSelectionSet([]*game.UnitInfo{a, b, c}, a)
	s.setUnitGroup(0)

	if got := s.unitGroups[0]; len(got) != 3 {
		t.Fatalf("group 1 should hold all 3 selected ids, got %v", got)
	}
}

// TestUnitGroupExclusiveMembership: assigning a unit to a group removes it from any
// other group it was in (a unit belongs to only one numbered group).
func TestUnitGroupExclusiveMembership(t *testing.T) {
	s, g := newClickScreen(t)
	a := mkMobile(g, 0, 1, 3, 3)
	b := mkMobile(g, 0, 2, 4, 4)

	// Group 9 holds both A and B.
	s.setSelectionSet([]*game.UnitInfo{a, b}, a)
	s.setUnitGroup(8)

	// Assign just A to group 1: A must leave group 9, B stays.
	s.setSelectionSet([]*game.UnitInfo{a}, a)
	s.setUnitGroup(0)

	if len(s.unitGroups[0]) != 1 || s.unitGroups[0][0] != a.ID {
		t.Fatalf("group 1 should hold only A, got %v", s.unitGroups[0])
	}

	for _, id := range s.unitGroups[8] {
		if id == a.ID {
			t.Fatalf("A should have left group 9 when assigned to group 1, group 9 = %v", s.unitGroups[8])
		}
	}

	if len(s.unitGroups[8]) != 1 || s.unitGroups[8][0] != b.ID {
		t.Errorf("group 9 should still hold B, got %v", s.unitGroups[8])
	}
}

// TestRecallSelectsWholeGroupSkipsDead: recalling reselects every surviving member
// and skips an id no longer in the registry (a destroyed unit).
func TestRecallSelectsWholeGroupSkipsDead(t *testing.T) {
	s, g := newClickScreen(t)
	a := mkMobile(g, 0, 1, 3, 3)
	b := mkMobile(g, 0, 2, 4, 4)

	s.unitGroups[2] = []uint16{a.ID, b.ID, 999} // 999 is a destroyed / never deployed member
	s.recallUnitGroup(2)

	set := s.selectionSet()
	if len(set) != 2 {
		t.Fatalf("recall should reselect the 2 survivors (skipping the dead id), got %d", len(set))
	}

	if s.selected == nil {
		t.Error("recall should leave a leader selected")
	}
}

// TestCaptureApplyUnitGroupsRoundTrip: capture stores only assigned slots and apply
// restores them, clearing anything the save did not carry (a pre v16 save clears all).
func TestCaptureApplyUnitGroupsRoundTrip(t *testing.T) {
	s, _ := newClickScreen(t)
	s.unitGroups[2] = []uint16{1, 2, 3}
	s.unitGroups[5] = []uint16{9}

	snap := s.captureUnitGroups()
	if len(snap) != 2 {
		t.Fatalf("capture should store only the 2 assigned slots, got %d", len(snap))
	}

	s2, _ := newClickScreen(t)
	s2.unitGroups[0] = []uint16{7} // a stale slot that a load must clear
	s2.applyUnitGroups(snap)

	if len(s2.unitGroups[0]) != 0 {
		t.Errorf("slot 1 should be cleared after a load, got %v", s2.unitGroups[0])
	}

	if len(s2.unitGroups[2]) != 3 || len(s2.unitGroups[5]) != 1 {
		t.Errorf("slots 3 and 6 should be restored, got %v and %v", s2.unitGroups[2], s2.unitGroups[5])
	}

	// A pre v16 save (nil) clears every slot; an out of range slot is ignored.
	s2.applyUnitGroups(nil)
	for i, ids := range s2.unitGroups {
		if len(ids) != 0 {
			t.Errorf("slot %d should be empty after a nil apply, got %v", i+1, ids)
		}
	}

	s2.applyUnitGroups([]saveload.UnitGroupSnapshot{{Slot: 99, UnitIDs: []uint16{1}}})
	for i, ids := range s2.unitGroups {
		if len(ids) != 0 {
			t.Errorf("an out of range slot must be ignored, slot %d = %v", i+1, ids)
		}
	}
}

// TestDisplayTogglesRideTheSave pins the fix for overlays that reset on every load: the nine
// panel toggles (Survey, Status, Colors, Hits, Ammo, Range, Scan, Grid, Names) live on the
// screen and appeared nowhere in the save, so loading a game switched every one of them off
// and you had to set your view up again. They now ride the save, like the camera bookmarks
// and the control groups.
func TestDisplayTogglesRideTheSave(t *testing.T) {
	s := &Screen{}
	s.showHits = true
	s.showAmmo = true
	s.grid = true
	s.showNames = true

	snap := s.captureDisplayToggles()

	// A fresh screen (the state a load starts from) picks the save's view back up.
	loaded := &Screen{}
	loaded.applyDisplayToggles(snap)

	if !loaded.showHits || !loaded.showAmmo || !loaded.grid || !loaded.showNames {
		t.Error("the overlays that were ON did not come back with the save")
	}

	if loaded.showRange || loaded.showScan || loaded.showColors || loaded.showStatus || loaded.surveyOverlay {
		t.Error("an overlay that was OFF came back on")
	}

	// An older save carries no toggles at all: the running screen keeps what it has.
	keep := &Screen{showRange: true}
	keep.applyDisplayToggles(nil)

	if !keep.showRange {
		t.Error("a pre v20 save wiped the running screen's toggles; it should leave them alone")
	}
}
