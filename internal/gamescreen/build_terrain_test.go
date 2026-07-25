// build_terrain_test.go : tests constructor build site terrain legality checks on the game screen.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// TestBuildDescTerrainSpecialFlags verifies the build flow flag matrix for the terrain
// specials: a Road is drivable GROUND COVER (no slab, no stationary structure flag),
// the Connector / Bridge / Water Platform are structures with no slab, and a normal
// building still lays a slab.
func TestBuildDescTerrainSpecialFlags(t *testing.T) {
	s := &Screen{}
	cases := []struct {
		t               assets.UnitType
		large           bool
		wantGroundCover bool
		wantSlab        bool
	}{
		{assets.Road, false, true, false},      // road: ground cover, no slab
		{assets.Cnct4W, false, false, false},   // connector: structure, no slab
		{assets.Bridge, false, false, false},   // bridge: structure, no slab
		{assets.WtrPltfm, false, false, false}, // water platform: structure, no slab
		{assets.PowGen, false, false, true},    // a normal 1x1 building lays a slab
		{assets.PowerStn, true, false, true},   // a normal 2x2 building lays a slab
	}
	for _, c := range cases {
		s.buildUI.buildRoster = []buildableType{{c.t, c.large}}
		s.buildUI.buildIdx = 1
		d, ok := s.buildDesc()
		if !ok {
			t.Errorf("%v: buildDesc returned ok=false", c.t)
			continue
		}
		if gc := d.Flags.Has(game.FlagGroundCover); gc != c.wantGroundCover {
			t.Errorf("%v: ground cover = %v, want %v", c.t, gc, c.wantGroundCover)
		}
		if sl := d.Flags.Has(game.FlagRequiresSlab); sl != c.wantSlab {
			t.Errorf("%v: requires slab = %v, want %v", c.t, sl, c.wantSlab)
		}
		// A road is never a selectable stationary structure.
		if c.t == assets.Road && (d.Flags.Has(game.FlagStationary) || d.Flags.Has(game.FlagSelectable)) {
			t.Errorf("road flags = %#x; must be ground cover only (no stationary/selectable)", d.Flags)
		}
	}
}
