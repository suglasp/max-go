// saveload_test.go : tests the Save/Load screen's per slot plate art resolution.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package menus

import (
	"testing"

	"github.com/suglasp/max-go/internal/resfile"
	"github.com/suglasp/max-go/internal/sprites"
	"github.com/suglasp/max-go/internal/testutil"
)

// TestSlotArtTags verifies the tag naming: FILE<n>_UP/_DN for positions 0-8, and
// the underscore free FILE10UP/FILE10DN for position 9 (RES tags cap at 8
// characters). C++: ResourceID FILE1_UP + i (saveloadmenu.cpp).
func TestSlotArtTags(t *testing.T) {
	up, dn := slotArtTags(0)
	if up != "FILE1_UP" || dn != "FILE1_DN" {
		t.Errorf("pos 0 = %s/%s, want FILE1_UP/FILE1_DN", up, dn)
	}
	up, dn = slotArtTags(8)
	if up != "FILE9_UP" || dn != "FILE9_DN" {
		t.Errorf("pos 8 = %s/%s, want FILE9_UP/FILE9_DN", up, dn)
	}
	up, dn = slotArtTags(9)
	if up != "FILE10UP" || dn != "FILE10DN" {
		t.Errorf("pos 9 = %s/%s, want FILE10UP/FILE10DN", up, dn)
	}
}

// TestSlotArtDecodes verifies all ten per slot plate art pairs exist in the
// retail archives and decode: each page position draws its OWN pre rendered
// plate, so a single missing tag would kill the whole Save/Load screen.
func TestSlotArtDecodes(t *testing.T) {
	dir := testutil.DataDir(t)
	lib, err := resfile.OpenDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer lib.Close()

	for pos := 0; pos < slPerPage; pos++ {
		upTag, dnTag := slotArtTags(pos)
		for _, tag := range []string{upTag, dnTag} {
			data, err := lib.Read(tag)
			if err != nil {
				t.Fatalf("%s: %v", tag, err)
			}
			if _, err := sprites.DecodeSimple(data); err != nil {
				t.Errorf("%s: %v", tag, err)
			}
		}
	}
}
