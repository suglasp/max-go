// site_select_test.go : tests build site selection legality and hover feedback on the game screen.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/units"
)

// TestClearingBulldozerSelectableFromFootprint verifies the 2x2
// selection hit area bug: a bulldozer clearing a building pile is drawn centred on
// the 2x2 site but hashed only on its anchor cell, so clicking the other three site
// cells found no unit. clickCell / topUnitAt now fall back to the reserved footprint
// (siteUnitAt), so the dozer is selectable from any of the four cells.
func TestClearingBulldozerSelectableFromFootprint(t *testing.T) {
	const w, h = 16, 16
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLandSurf{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)

	// A 2x2 building destroyed leaves a large (2x2) rubble pile anchored at (5,5).
	bld := game.NewUnitInfo(assets.PowerStn, 0, 1, 0)
	bld.Flags = game.FlagStationary | game.FlagBuilding | game.FlagSelectable
	bld.Values = &game.UnitValues{}
	bld.Values.Set(game.AttribHits, 40)
	bld.InitState()
	g.PlaceBuilding(bld, 5, 5)
	g.DestroyUnit(bld)

	// A bulldozer parked on the pile starts clearing it (drawn centred on the 2x2).
	dozer := game.NewUnitInfo(assets.Bulldozr, 0, 2, 0)
	dozer.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	g.Units.Deploy(dozer, 5, 5)
	if !g.StartClearRubble(dozer) {
		t.Fatal("StartClearRubble returned false (no large pile under the dozer?)")
	}

	s := &Screen{game: g}
	// Each of the four footprint cells selects the clearing bulldozer.
	for _, c := range [][2]int{{5, 5}, {6, 5}, {5, 6}, {6, 6}} {
		s.selected = nil
		s.clickCell(c[0], c[1])
		if s.selected != dozer {
			t.Errorf("click (%d,%d) selected %v; want the clearing bulldozer", c[0], c[1], s.selected)
		}
	}
	// topUnitAt (cursor / right click status) resolves to it from an off anchor cell.
	if got := s.topUnitAt(6, 6); got != dozer {
		t.Errorf("topUnitAt(6,6) = %v; want the clearing bulldozer", got)
	}
	// A cell off the site selects nothing (no spurious selection).
	s.selected = nil
	s.clickCell(10, 10)
	if s.selected == dozer {
		t.Error("a click off the site must not select the bulldozer")
	}
}
