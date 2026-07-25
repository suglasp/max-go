// repairshop_test.go : tests service-building repair and upgrade queues.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package menus

import "testing"

// TestRepairSlotGeom locks the per slot layout arithmetic to repairshopmenu.cpp:
// depot/dock/barracks fill a 3x2 grid, the hangar a 2x2 grid of wider wells.
func TestRepairSlotGeom(t *testing.T) {
	// Depot (6 slots, 3 columns): slot 0, slot 3 (next row), slot 5 (last).
	depot := []struct {
		p                             int
		ulx, uly, wellW, statX, statY int
	}{
		{0, 17, 9, 128, 20, 149},
		{3, 17, 244, 128, 20, 384},
		{5, 327, 244, 128, 330, 384},
	}
	for _, c := range depot {
		ulx, uly, w, sx, sy := rsSlotGeom(false, c.p)
		if ulx != c.ulx || uly != c.uly || w != c.wellW || sx != c.statX || sy != c.statY {
			t.Errorf("depot slot %d = (%d,%d,w%d,stat %d,%d), want (%d,%d,w%d,stat %d,%d)",
				c.p, ulx, uly, w, sx, sy, c.ulx, c.uly, c.wellW, c.statX, c.statY)
		}
	}
	// Hangar (4 slots, 2 columns, 200-wide wells): slot 0 and slot 3.
	hangar := []struct {
		p                             int
		ulx, uly, wellW, statX, statY int
	}{
		{0, 18, 9, 200, 56, 149},
		{3, 244, 244, 200, 282, 384},
	}
	for _, c := range hangar {
		ulx, uly, w, sx, sy := rsSlotGeom(true, c.p)
		if ulx != c.ulx || uly != c.uly || w != c.wellW || sx != c.statX || sy != c.statY {
			t.Errorf("hangar slot %d = (%d,%d,w%d,stat %d,%d), want (%d,%d,w%d,stat %d,%d)",
				c.p, ulx, uly, w, sx, sy, c.ulx, c.uly, c.wellW, c.statX, c.statY)
		}
	}
}

// TestRepairSlotButtonDecode verifies the slot/action encoding the run loop
// decodes from a per slot button value (rsSlotBase + p*4 + action).
func TestRepairSlotButtonDecode(t *testing.T) {
	for p := 0; p < 6; p++ {
		for _, a := range []int{rsAct, rsRel, rsRep, rsUpg} {
			key := rsSlotBase + p*4 + a
			gotP := (key - rsSlotBase) / 4
			gotA := (key - rsSlotBase) % 4
			if gotP != p || gotA != a {
				t.Errorf("decode key %d = (p%d,a%d), want (p%d,a%d)", key, gotP, gotA, p, a)
			}
		}
	}
	// The fixed right column values must not collide with the slot range.
	if rsActivateAll < rsSlotBase+6*4 {
		t.Errorf("rsActivateAll %d overlaps the slot value range [%d,%d)", rsActivateAll, rsSlotBase, rsSlotBase+6*4)
	}
}
