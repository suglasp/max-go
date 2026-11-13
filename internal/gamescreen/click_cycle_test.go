// click_cycle_test.go : tests left click stacked unit cycling on the game screen.
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

// TestClickCycleStackedUnits verifies that a unit
// parked on a connector ("collector") could not be told apart from the building
// under it. Clicking a cell the selected unit already sits on now cycles to the
// NEXT stacked occupant (the original's Access_GetSelectableUnit) and wraps back.
func TestClickCycleStackedUnits(t *testing.T) {
	s, g := newTestScreen(t, 16)
	// An engineer (mobile) parked on a connector (stationary, selectable), same cell.
	eng := mkUnit(g, assets.Engineer, 0, 1, 5, 5, game.FlagMobileLandUnit|game.FlagSelectable)
	conn := mkUnit(g, assets.Cnct4W, 0, 2, 5, 5, game.FlagStationary|game.FlagSelectable)

	s.selected = eng
	s.clickCell(5, 5)
	if s.selected != conn {
		t.Fatalf("click should cycle to the connector under the engineer, got %v", s.selected)
	}
	s.clickCell(5, 5)
	if s.selected != eng {
		t.Errorf("a second click should cycle back to the engineer, got %v", s.selected)
	}
}

// TestClickMoveOntoConnectorAtZeroMoves verifies that clicking the
// connector's cell to position the engineer there selected the connector instead
// of moving the engineer. A drivable structure now takes a MOVE (queued at 0
// moves), not a select - matching the GO cursor.
func TestClickMoveOntoConnectorAtZeroMoves(t *testing.T) {
	s, g := newTestScreen(t, 16)
	eng := mkUnit(g, assets.Engineer, 0, 1, 4, 4, game.FlagMobileLandUnit|game.FlagSelectable)
	eng.Speed = 0 // out of moves (just built the connector)
	mkUnit(g, assets.Cnct4W, 0, 2, 5, 4, game.FlagStationary|game.FlagSelectable)

	s.selected = eng
	s.clickCell(5, 4)
	s.effects.bulbDue = 0
	s.tickPlanningBulb() // flush the deferred move behind the planning bulb
	if s.selected != eng {
		t.Fatalf("clicking the connector should move the engineer, not select the connector; selected=%v", s.selected)
	}
	found := false
	for _, pr := range g.PendingRoutes() {
		if pr.Unit == eng {
			found = true
		}
	}
	if !found {
		t.Errorf("the engineer's 0-move move onto the connector should be queued, PendingRoutes=%d", len(g.PendingRoutes()))
	}
}

// TestBuildingSelectedClickTerrainNoMove verifies that
// selecting a STRUCTURE then clicking terrain drew the green/blue move path arrows
// and said "no path to destination". A stationary building (Speed 0, no movement
// flag, u.Building==false because that bool is the build POSE state of a mobile
// builder) must never pathfind - the click does nothing.
func TestBuildingSelectedClickTerrainNoMove(t *testing.T) {
	s, g := newTestScreen(t, 16)
	// A power station: a real structure (stationary, no movement flag).
	bldg := mkUnit(g, assets.PowerStn, 0, 1, 5, 5, game.FlagStationary|game.FlagBuilding|game.FlagSelectable)
	if bldg.Building {
		t.Fatal("a freshly deployed structure should not be in a build pose")
	}

	s.selected = bldg
	s.clickCell(10, 10) // empty terrain
	if n := len(g.PendingRoutes()); n != 0 {
		t.Errorf("clicking terrain with a building selected queued a route (n=%d); a structure must not move", n)
	}
	if n := len(g.MoveOrders()); n != 0 {
		t.Errorf("clicking terrain with a building selected started a move order (n=%d)", n)
	}

	// A real mobile unit out of moves still queues its route (the path with no move is
	// not broken by the building guard).
	tank := mkUnit(g, assets.Tank, 0, 2, 4, 4, game.FlagMobileLandUnit|game.FlagSelectable)
	tank.Speed = 0
	s.selected = tank
	s.clickCell(9, 4)
	s.effects.bulbDue = 0
	s.tickPlanningBulb() // flush the deferred move behind the planning bulb
	if len(g.PendingRoutes()) == 0 {
		t.Error("a mobile 0-move unit should still queue its route after the building-guard fix")
	}
}

// TestAirUnitClicksBuildingSelectsIt verifies that with
// an air unit (Fighter/Bomber/AWAC) selected, clicking a building issued a MOVE order
// to the plane instead of selecting the building. An air unit returns CanEnter true on
// every cell (it flies over everything), so it hit the drive onto structure guard; the
// guard now excludes air units, so a click on a building cell selects the building.
func TestAirUnitClicksBuildingSelectsIt(t *testing.T) {
	s, g := newTestScreen(t, 16)
	plane := mkUnit(g, assets.Fighter, 0, 1, 3, 3, game.FlagMobileAirUnit|game.FlagHovering|game.FlagSelectable)
	bldg := mkUnit(g, assets.PowerStn, 0, 2, 7, 7, game.FlagStationary|game.FlagBuilding|game.FlagSelectable)

	s.selected = plane
	s.clickCell(7, 7) // the building's cell
	s.effects.bulbDue = 0
	s.tickPlanningBulb() // flush any deferred move behind the planning bulb
	if s.selected != bldg {
		t.Fatalf("an air unit clicking a building should select the building, got %v", s.selected)
	}
	if n := len(g.PendingRoutes()); n != 0 {
		t.Errorf("an air-unit building click queued a route (n=%d); it must select, not move", n)
	}
	if n := len(g.MoveOrders()); n != 0 {
		t.Errorf("an air-unit building click started a move order (n=%d)", n)
	}
}

// TestPopupTargetsBuildReadyConstructorNotHiddenBuilding verifies that a long press targets the parked constructor rather than the completed structure beneath it.
// The constructor must receive the popup so the player can drive it off the site.
func TestPopupTargetsBuildReadyConstructorNotHiddenBuilding(t *testing.T) {
	g, c, _, _ := finishedConstructor(t)
	g.State.ActiveTeam = 0
	s := &Screen{game: g}

	// The parked build ready constructor is hashed across its reserved build site
	// footprint (it keeps its build pose), not at its old GridX/GridY.
	got := s.popupTargetAt(c.BuildSiteX, c.BuildSiteY)
	if got != c {
		t.Fatalf("popupTargetAt = %v, want the build-ready constructor", got)
	}
	if br := g.BuildReadyInfo(c); br != nil && got == br.Building {
		t.Error("the long-press popup targeted the HIDDEN building, not the parked constructor")
	}
}

// TestPopupTargetSkipsHiddenPrefersSelected covers the rest of the popup resolution:
// a hidden/unexpanded structure is never the target, and the already selected unit
// wins over another mobile unit sharing the cell (deterministic, matching a click).
func TestPopupTargetSkipsHiddenPrefersSelected(t *testing.T) {
	s, g := newTestScreen(t, 16)
	// A hidden (unexpanded) building + a mobile unit on the same cell: the popup
	// targets the mobile unit, never the hidden structure.
	hidden := mkUnit(g, assets.PowerStn, 0, 1, 5, 5, game.FlagStationary|game.FlagBuilding|game.FlagSelectable)
	hidden.HiddenUntilExpand = true
	tank := mkUnit(g, assets.Tank, 0, 2, 5, 5, game.FlagMobileLandUnit|game.FlagSelectable)
	if got := s.popupTargetAt(5, 5); got != tank {
		t.Fatalf("popupTargetAt = %v, want the mobile tank (hidden building skipped)", got)
	}
	// Two mobile units on a cell: the already selected one is the popup target.
	other := mkUnit(g, assets.Tank, 0, 3, 5, 5, game.FlagMobileLandUnit|game.FlagSelectable)
	s.selected = other
	if got := s.popupTargetAt(5, 5); got != other {
		t.Errorf("popupTargetAt = %v, want the already-selected unit", got)
	}
}

// TestRightClickMapDeselect verifies the rule: a right click on
// empty terrain/sea clears the selection, while a right click over a unit keeps it
// (it opens the Status screen instead).
func TestRightClickMapDeselect(t *testing.T) {
	s, g := newTestScreen(t, 16)
	u := mkUnit(g, assets.Tank, 0, 1, 3, 3, game.FlagMobileLandUnit|game.FlagSelectable)

	s.selected = u
	s.rightClickMap(8, 8) // empty cell
	if s.selected != nil {
		t.Errorf("right-click on empty terrain should deselect, got %v", s.selected)
	}

	s.selected = u
	s.rightClickMap(3, 3) // over the unit
	if s.selected != u {
		t.Errorf("right-click over a unit should keep the selection (status), got %v", s.selected)
	}
}
