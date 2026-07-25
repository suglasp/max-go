// buildcancel_test.go : tests a constructor returning from a stopped 2x2 build pose.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package sim

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// TestCancelBuildSlidesConstructorBack verifies that stopping a 2x2 build arms Leaving, returns the constructor to its corner cell, and collapses its hash footprint to 1x1.
func TestCancelBuildSlidesConstructorBack(t *testing.T) {
	g := newGame(t, 24, 24)
	const cornerX, cornerY = 6, 6
	const tile = 64

	// A constructor sits on the top left corner of the 2x2 site and starts a building.
	con := game.NewUnitInfo(assets.Constrct, 0, 1, 0)
	con.Flags = game.FlagMobileLandUnit | game.FlagConstructorUnit | game.FlagSelectable
	con.Values = &game.UnitValues{}
	con.InitState()
	con.Storage = 200
	g.Units.Deploy(con, cornerX, cornerY)

	depot := game.NewUnitInfo(assets.Depot, 0, 2, 0)
	depot.Flags = game.FlagBuilding | game.FlagStationary | game.FlagRequiresSlab | game.FlagSelectable
	depot.Values = &game.UnitValues{}
	depot.Values.Set(game.AttribTurns, 5) // long enough that it stays in the pose
	depot.Values.Set(game.AttribHits, 10)
	g.StartBuild(con, depot, cornerX, cornerY, 1)

	if !con.Building || con.BuildFootprint != 2 {
		t.Fatalf("constructor not in a 2x2 build pose: Building=%v fp=%d", con.Building, con.BuildFootprint)
	}
	poseAngle := con.Angle

	// Drive the enter slide so the builder is parked off the grid at the footprint centre.
	for i := 0; i < 64 && !con.AdvanceBuildSlide(8); i++ {
	}
	centreX := cornerX*tile + 2*tile/2
	if con.X != centreX {
		t.Fatalf("enter slide did not reach the footprint centre: X=%d want %d", con.X, centreX)
	}
	// While building, the whole 2x2 site is reserved (the far corner is occupied).
	if len(g.Units.Cells.At(cornerX+1, cornerY+1)) == 0 {
		t.Fatal("2x2 build site not reserved at the far corner before Stop")
	}

	g.CancelBuild(con)

	// The pose is cleared and the leave slide is armed back to the own cell centre.
	if con.Building {
		t.Error("constructor still in the build pose after Stop")
	}
	if !con.Leaving {
		t.Error("constructor is not sliding back (Leaving=false) - it would be stranded at the centre")
	}
	wantX, wantY := cornerX*tile+tile/2, cornerY*tile+tile/2
	if con.BuildToX != wantX || con.BuildToY != wantY {
		t.Errorf("leave target = (%d,%d); want the own-cell centre (%d,%d)", con.BuildToX, con.BuildToY, wantX, wantY)
	}
	// A 2x2 builder turns to face the outward direction (driving back out), nose first.
	if want := (poseAngle + 4) & 7; con.Angle != want {
		t.Errorf("leave angle = %d; want the outward flip %d", con.Angle, want)
	}
	// The hash reservation collapsed from the 2x2 to the single corner cell.
	if got := len(g.Units.Cells.At(cornerX+1, cornerY+1)); got != 0 {
		t.Errorf("far corner still reserved after Stop: %d occupants (hash did not collapse)", got)
	}
	if len(g.Units.Cells.At(cornerX, cornerY)) == 0 {
		t.Error("constructor not bucketed at its own corner cell after Stop")
	}

	// Driving the leave slide to completion lands the unit aligned to the grid on its own cell.
	for i := 0; i < 64 && !con.AdvanceBuildSlide(8); i++ {
	}
	if con.X != wantX || con.Y != wantY {
		t.Errorf("after the leave slide the constructor is at (%d,%d); want grid-aligned (%d,%d)", con.X, con.Y, wantX, wantY)
	}
}
