// bridge_test.go : tests bridge.go.
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

// drawIndexOf returns u's position in a draw order slice, or -1.
func drawIndexOf(order []*game.UnitInfo, u *game.UnitInfo) int {
	for i, v := range order {
		if v == u {
			return i
		}
	}
	return -1
}

// runBridgeAnim runs the raise/lower animation to completion, calling the step
// directly, not throttled, so the test does not depend on wall clock timing.
func runBridgeAnim(s *Screen) {
	for i := 0; i < game.BridgeRaiseMax+2; i++ {
		s.stepBridges()
	}
}

// TestPassesUnderBridge verifies which units go UNDER a bridge (raising it) versus
// OVER it. Per the original the trigger is !(MOBILE_SEA_UNIT); verified in vanilla, the
// Personnel Carrier (a true dual flag amphibian) and pure sea units go under, while
// the Constructor and Engineer (land builders that can drive onto water, but not
// MOBILE_SEA_UNIT in vanilla) cross over, like a pure land unit.
func TestPassesUnderBridge(t *testing.T) {
	mk := func(ut assets.UnitType, flags game.Flags) *game.UnitInfo {
		u := game.NewUnitInfo(ut, 0, 1, 0)
		u.Flags = flags
		return u
	}
	land := game.FlagMobileLandUnit
	sea := game.FlagMobileSeaUnit
	amph := game.FlagMobileLandUnit | game.FlagMobileSeaUnit
	cases := []struct {
		name string
		u    *game.UnitInfo
		want bool
	}{
		{"submarine (sea)", mk(assets.Submarne, sea), true},
		{"fastboat (sea)", mk(assets.FastBoat, sea), true},
		{"personnel carrier (dual flag)", mk(assets.ClnTrans, amph), true},
		// The Constructor/Engineer/Scout carry only MOBILE_LAND_UNIT now (like the Surveyor),
		// so they cross over the bridge, not under.
		{"constructor (land builder)", mk(assets.Constrct, land), false},
		{"engineer (land builder)", mk(assets.Engineer, land), false},
		{"scout (land recon)", mk(assets.Scout, land), false},
		{"tank (land)", mk(assets.Tank, land), false},
	}
	for _, c := range cases {
		if got := passesUnderBridge(c.u); got != c.want {
			t.Errorf("%s: passesUnderBridge = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestFlatBridgeDrawsUnderApproachingUnit verifies that a flat bridge remains below a land unit while it approaches and while it occupies the bridge cell.
// Although the bridge carries FlagStationary rather than FlagGroundCover, it belongs to the ground-cover draw layer.
func TestFlatBridgeDrawsUnderApproachingUnit(t *testing.T) {
	const w, h = 8, 8
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLandSurf{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)
	s := &Screen{game: g}

	bridge := game.NewUnitInfo(assets.Bridge, 0, 1, 0) // flat (BridgeRaise 0)
	bridge.Flags = game.FlagStationary | game.FlagSelectable
	mgr.Deploy(bridge, 3, 4)

	scout := game.NewUnitInfo(assets.Scout, 0, 2, 0)
	scout.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	mgr.Deploy(scout, 3, 3)

	// Approaching from a nearer row (lower GridY) and standing on the bridge cell:
	// the flat bridge must draw first (under) in both cases - lower index = drawn earlier.
	for _, pos := range [][2]int{{3, 3}, {3, 4}} {
		mgr.Move(scout, pos[0], pos[1])
		order := s.unitDrawOrder()
		if bi, si := drawIndexOf(order, bridge), drawIndexOf(order, scout); bi > si {
			t.Errorf("scout at (%d,%d): flat bridge draw index %d should be before the scout at %d (bridge under)", pos[0], pos[1], bi, si)
		}
	}
}

// TestWaterPlatformDrawsUnderUnit verifies that a water platform, as the floor over water, draws
// draw UNDER every unit - including a platform in a NEARER row (higher GridY) than a
// constructor standing a row behind it, which would otherwise clip the constructor's
// lower half. C++: UnitInfo::GetDrawLayer puts WTRPLTFM (layer 2) below units (layer 6).
func TestWaterPlatformDrawsUnderUnit(t *testing.T) {
	const w, h = 8, 8
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLandSurf{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)
	s := &Screen{game: g}

	constructor := game.NewUnitInfo(assets.Constrct, 0, 1, 0)
	constructor.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	mgr.Deploy(constructor, 3, 3)

	// A platform on the constructor's own cell AND one in the nearer row in front of it
	// (GridY+1): both must draw before (under) the constructor.
	platSame := game.NewUnitInfo(assets.WtrPltfm, 0, 2, 0)
	platSame.Flags = game.FlagStationary | game.FlagSelectable
	mgr.Deploy(platSame, 3, 3)

	platFront := game.NewUnitInfo(assets.WtrPltfm, 0, 3, 0)
	platFront.Flags = game.FlagStationary | game.FlagSelectable
	mgr.Deploy(platFront, 3, 4)

	order := s.unitDrawOrder()
	ci := drawIndexOf(order, constructor)
	for _, plat := range []*game.UnitInfo{platSame, platFront} {
		if pi := drawIndexOf(order, plat); pi > ci {
			t.Errorf("platform at (%d,%d) draw index %d should be before the constructor at %d (platform under)",
				plat.GridX, plat.GridY, pi, ci)
		}
	}
}

// TestRaisedBridgeDrawsOverSeaUnit verifies the elevated bridge z order: a bridge
// raised over a sea unit draws OVER it (the unit passes under the raised bridge).
// The raise and lower steps are pinned by TestUpdateBridgesRaisesToMaxAndLowersBack.
func TestRaisedBridgeDrawsOverSeaUnit(t *testing.T) {
	const w, h = 8, 8
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLandSurf{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.NewWithGrid(game.NewGameState(p), mgr, grid, w, h)
	s := &Screen{game: g}

	bridge := game.NewUnitInfo(assets.Bridge, 0, 1, 0)
	bridge.Flags = game.FlagStationary | game.FlagSelectable
	mgr.Deploy(bridge, 3, 3)

	boat := game.NewUnitInfo(assets.FastBoat, 0, 2, 0)
	boat.Flags = game.FlagMobileSeaUnit | game.FlagSelectable
	mgr.Deploy(boat, 3, 3) // same cell as the bridge: it passes under

	// With a sea unit on the cell the bridge raises and reports elevated.
	runBridgeAnim(s)
	/* // note: unused code in Go port
	// TestUpdateBridgesRaisesToMaxAndLowersBack drives the live updateBridges to the cap
	if bridge.BridgeRaise != game.BridgeRaiseMax {
		t.Errorf("bridge raise under a sea unit = %d, want %d (fully up)", bridge.BridgeRaise, game.BridgeRaiseMax)
	}
	*/
	if !isElevatedBridge(bridge) {
		t.Fatal("bridge should report elevated while raised")
	}

	// Z order: the raised bridge draws AFTER (over) the boat, both when the boat
	// shares its cell AND when the boat is APPROACHING from a nearer row (higher
	// GridY, in front) - the raised bridge is a top layer, so the boat never
	// flickers over it then flips under.
	for _, boatCell := range [][2]int{{3, 3}, {3, 5}} {
		mgr.Move(boat, boatCell[0], boatCell[1])
		order := s.unitDrawOrder()
		if bi, boi := drawIndexOf(order, bridge), drawIndexOf(order, boat); bi < boi {
			t.Errorf("boat at (%d,%d): raised bridge draw index %d should be after the boat at %d (over it)", boatCell[0], boatCell[1], bi, boi)
		}
	}
	/* // note: unused code in Go port
	// TestUpdateBridgesRaisesToMaxAndLowersBack pins the lower phase
	mgr.Move(boat, 3, 3) // restore the boat onto the cell for the lower phase

	// The boat leaves: the bridge lowers back to flat (ground cover under crossers).
	g.Units.Remove(boat)
	runBridgeAnim(s)
	if bridge.BridgeRaise != 0 {
		t.Errorf("bridge raise with no sea unit = %d, want 0 (flat)", bridge.BridgeRaise)
	}
	if isElevatedBridge(bridge) {
		t.Error("bridge should not be elevated once flat")
	}
	*/
}
