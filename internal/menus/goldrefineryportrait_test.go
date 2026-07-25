// goldrefineryportrait_test.go : tests the Gold Refinery portrait mapping.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package menus

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/resfile"
	"github.com/suglasp/max-go/internal/sprites"
	"github.com/suglasp/max-go/internal/testutil"
)

// TestGoldRefineryPortrait verifies the Gold Refinery portrait mapping: COMMTWR (the
// legacy "Communication Tower" enum, reused as the Gold Refinery) maps to the
// misleadingly named "P_TRANSP" image (the actual photo of the building with
// the tower and dish, NOT a transparent placeholder), and that image decodes from the real archive.
func TestGoldRefineryPortrait(t *testing.T) {
	if got := unitPortrait[assets.CommTwr]; got != "P_TRANSP" {
		t.Fatalf("unitPortrait[CommTwr] = %q, want P_TRANSP (the Gold Refinery photo)", got)
	}

	dir := testutil.DataDir(t)
	lib, err := resfile.OpenDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer lib.Close()

	data, err := lib.Read("P_TRANSP")
	if err != nil {
		t.Fatalf("P_TRANSP missing from the archive: %v", err)
	}
	if _, err := sprites.DecodeBig(data); err != nil {
		t.Errorf("P_TRANSP did not decode as a BIG image: %v", err)
	}
}
