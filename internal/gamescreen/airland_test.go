// airland_test.go : tests the air unit landing pad click flow in gamescreen.go.
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

// TestAirUnitClicksLandingPadMovesOntoIt verifies that a
// flying Bomber could not be sent to a landing pad - clicking the pad SELECTED it
// instead of moving the plane onto it, so the plane never reached a pad and the popup
// never offered Land. A click on a FRIENDLY landing pad with an air unit selected must
// now issue a move ONTO the pad (keeping the plane selected); once over the pad CanLand
// is true so the popup offers Land. A building that is not a pad still selects (a plane
// inspects it, it cannot land there).
func TestAirUnitClicksLandingPadMovesOntoIt(t *testing.T) {
	s, g := newClickScreen(t)

	// A player Bomber flying at (5,5) with move budget, and a player landing pad at (8,5).
	bomber := mkUnit(g, assets.Bomber, 1, 5, 5, game.FlagMobileAirUnit|game.FlagHovering|game.FlagSelectable)
	bomber.Hits = 10
	bomber.Speed = 16
	bomber.Values.Set(game.AttribSpeed, 16)
	pad := mkUnit(g, assets.LandPad, 2, 8, 5, game.FlagStationary|game.FlagSelectable)
	pad.Hits = 10

	s.selected = bomber

	// Click the pad: the plane is ordered onto it, NOT deselected for the pad.
	s.clickCell(8, 5)
	if s.selected != bomber {
		t.Fatalf("clicking the pad selected %v, want the bomber still selected (a move, not a select)", s.selected)
	}
	// The selected unit's move is deferred behind the "thinking" bulb dwell; flush it.
	s.effects.bulbDue = 0
	s.tickPlanningBulb()
	if !g.HasOrder(bomber) {
		t.Fatal("clicking the pad did not issue a move order for the air unit")
	}

	// Resolve the move: the bomber sits over the pad and can now land.
	g.ResolveMovement()
	if bomber.GridX != 8 || bomber.GridY != 5 {
		t.Fatalf("bomber at (%d,%d) after the move, want (8,5) over the pad", bomber.GridX, bomber.GridY)
	}
	if !g.CanLand(bomber) {
		t.Error("an air unit over a friendly pad should be able to land (popup Land)")
	}
}

// TestAirUnitClicksOtherBuildingSelectsIt: a plane clicking a friendly building that is
// NOT a pad still SELECTS it (the plane flies above and inspects it; it cannot set down there).
func TestAirUnitClicksOtherBuildingSelectsIt(t *testing.T) {
	s, g := newClickScreen(t)

	bomber := mkUnit(g, assets.Bomber, 1, 5, 5, game.FlagMobileAirUnit|game.FlagHovering|game.FlagSelectable)
	bomber.Hits = 10
	bomber.Speed = 16
	bomber.Values.Set(game.AttribSpeed, 16)
	// A building that is not a pad (a mining station) at (8,5).
	bld := mkUnit(g, assets.MiningSt, 2, 8, 5, game.FlagBuilding|game.FlagStationary|game.FlagSelectable)
	bld.Hits = 10

	s.selected = bomber
	s.clickCell(8, 5)
	if s.selected != bld {
		t.Fatalf("clicking a non-pad building selected %v, want the building", s.selected)
	}
	if g.HasOrder(bomber) {
		t.Error("clicking a non-pad building must not move the plane onto it")
	}
}

// TestAirUnitAutoLandsOnPadArrival: an air unit that comes to rest ON its friendly
// landing pad sets down by itself (vanilla lands a plane by moving it onto the pad -
// there is no Land popup). The arrival tick (tickMovementSfx) clears Hovering.
func TestAirUnitAutoLandsOnPadArrival(t *testing.T) {
	s, g := newClickScreen(t)
	s.fogTeam = 0
	s.sound.sfxFn = func(string, string, float64) {}

	pad := mkUnit(g, assets.LandPad, 2, 8, 5, game.FlagStationary|game.FlagSelectable)
	pad.Hits = 10
	bomber := mkUnit(g, assets.Bomber, 1, 8, 5, game.FlagMobileAirUnit|game.FlagHovering|game.FlagSelectable)
	bomber.Hits = 10

	// Simulate the bomber having just driven onto the pad: it is in the drive sound set
	// with no remaining move order, so the arrival tick lands it automatically.
	s.sound.driveSfx = map[*game.UnitInfo]int{bomber: 1}
	s.sound.driveSfxType = map[*game.UnitInfo]game.SfxType{bomber: game.SfxDrive}

	s.tickMovementSfx()
	if bomber.Flags.Has(game.FlagHovering) {
		t.Error("an air unit arriving on a friendly pad should auto-land (clear Hovering)")
	}
}
