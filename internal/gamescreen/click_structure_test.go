// click_structure_test.go : tests structure/building click selection precedence on the game screen.
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

// clickTestRoad deploys a selectable, passable Road structure (a terrain structure
// a unit can drive onto) at (gx, gy).
func clickTestRoad(team, id, gx, gy int, mgr *units.Manager) *game.UnitInfo {
	road := game.NewUnitInfo(assets.Road, team, id, 0)
	road.Flags = game.FlagStationary | game.FlagSelectable
	road.Values = &game.UnitValues{}
	road.InitState()
	mgr.Deploy(road, gx, gy)
	return road
}

// TestClickUnitOnStructureWins verifies ticket bug #2/#3: a mobile unit parked on a
// passable structure wins the click over the structure; a cell with only a structure, the
// selected unit can drive onto moves there (mirrors the move cursor) instead of
// selecting the structure; and a cell with only a structure and nothing selected still
// selects the structure.
func TestClickUnitOnStructureWins(t *testing.T) {
	const w, h = 24, 24
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLandSurf{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	p.Teams[1].Type = game.TeamComputer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)
	s := &Screen{game: g}

	// (1) A unit standing on a road: clicking the cell selects the unit, not the road.
	tank := clickTestUnit(0, 1, 5, 5, mgr)
	road := clickTestRoad(0, 2, 5, 5, mgr)
	s.selected = nil
	s.clickCell(5, 5)
	if s.selected != tank {
		t.Errorf("unit-on-road click selected %v; want the unit", s.selected)
	}
	_ = road

	// (2) A cell with only a road the selected unit can drive onto: it moves there (keeps
	// the unit selected) rather than selecting the road.
	roadFar := clickTestRoad(0, 3, 8, 8, mgr)
	s.selected = tank
	s.clickCell(8, 8)
	if s.selected != tank {
		t.Errorf("driving onto a road selected %v; want the unit to stay selected and move", s.selected)
	}
	if s.selected == roadFar {
		t.Error("driving onto a road must not select the road")
	}

	// (3) A cell with only a road and nothing selected: the road is selected.
	s.selected = nil
	s.clickCell(8, 8)
	if s.selected != roadFar {
		t.Errorf("clicking a lone road with no selection selected %v; want the road", s.selected)
	}
}

// TestClickBuildingOnPlatformWins verifies that a
// building raised on water platforms could not be selected - the click picked the
// lower ID platform tile under it. A real structure (the building) must win the click
// over the FLOOR structure (the water platform) it stands on; a lone platform with
// nothing selected is still selectable.
func TestClickBuildingOnPlatformWins(t *testing.T) {
	const w, h = 16, 16
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLandSurf{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)
	s := &Screen{game: g}

	// A water platform (lower id = the floor) and a building (higher id) on one cell.
	plat := game.NewUnitInfo(assets.WtrPltfm, 0, 1, 0)
	plat.Flags = game.FlagStationary | game.FlagSelectable
	mgr.Deploy(plat, 6, 6)

	bld := game.NewUnitInfo(assets.PowerStn, 0, 2, 0)
	bld.Flags = game.FlagStationary | game.FlagSelectable | game.FlagBuilding
	mgr.Deploy(bld, 6, 6)

	s.selected = nil
	s.clickCell(6, 6)
	if s.selected != bld {
		t.Errorf("building-on-platform click selected %v; want the building", s.selected)
	}

	// A lone platform (no building) is still selectable with nothing selected.
	lonePlat := game.NewUnitInfo(assets.WtrPltfm, 0, 3, 0)
	lonePlat.Flags = game.FlagStationary | game.FlagSelectable
	mgr.Deploy(lonePlat, 9, 9)
	s.selected = nil
	s.clickCell(9, 9)
	if s.selected != lonePlat {
		t.Errorf("lone platform click selected %v; want the platform", s.selected)
	}
}
