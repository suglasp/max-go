// rubble_water_test.go : tests rubble suppression on water/coast terrain.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package sim

import (
	"testing"

	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/units"
	"github.com/suglasp/max-go/internal/world"
)

// waterAt is a terrain surface where the listed cells are water and the rest land.
type waterAt map[[2]int]bool

func (w waterAt) SurfaceAt(x, y int) uint8 {
	if w[[2]int{x, y}] {
		return world.SurfaceWater
	}
	return world.SurfaceLand
}

// newGameSurf builds a sim Game over a custom terrain surface (for water tests).
func newGameSurf(w, h int, surf access.SurfaceSource) *Game {
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: surf, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	return NewWithGrid(game.NewGameState(p), mgr, grid, w, h)
}

// TestSeaUnitOnWaterLeavesNoRubble: a sea unit destroyed on water sinks - it leaves
// no debris pile (the original + MaxR both gate rubble on land terrain).
func TestSeaUnitOnWaterLeavesNoRubble(t *testing.T) {
	g := newGameSurf(12, 12, waterAt{{5, 5}: true})
	ship := game.NewUnitInfo(assets.Corvette, 0, 1, 0)
	ship.Flags = game.FlagMobileSeaUnit | game.FlagSelectable
	ship.Values = &game.UnitValues{}
	g.Units.Deploy(ship, 5, 5)

	g.DestroyUnit(ship)
	if _, ok := g.RubbleAt(5, 5); ok {
		t.Error("a sea unit destroyed on water must leave no rubble (it sinks)")
	}
}

// TestLandUnitOnLandLeavesRubble verifies that a land unit destroyed on land leaves a small pile.
func TestLandUnitOnLandLeavesRubble(t *testing.T) {
	g := newGameSurf(12, 12, waterAt{}) // all land
	tank := game.NewUnitInfo(assets.Tank, 0, 1, 0)
	tank.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	tank.Values = &game.UnitValues{}
	g.Units.Deploy(tank, 5, 5)

	g.DestroyUnit(tank)
	if large, ok := g.RubbleAt(5, 5); !ok || large {
		t.Errorf("a land unit on land should leave a small pile (large=%v ok=%v)", large, ok)
	}
}

// TestBuildingTouchingWaterLeavesNoRubble: a 2x2 building with any cell over water
// is not fully land covered, so the original leaves no pile (all or nothing, no per cell split).
func TestBuildingTouchingWaterLeavesNoRubble(t *testing.T) {
	g := newGameSurf(12, 12, waterAt{{6, 6}: true}) // the SE corner of the footprint
	b := addBuilding(g, 1, 5, 5)                    // PowerStn 2x2 over (5,5)..(6,6)

	g.DestroyUnit(b)
	if _, ok := g.RubbleAt(5, 5); ok {
		t.Error("a building straddling water must leave no rubble (klei all-or-nothing)")
	}
}

// TestBuildingFullyOnLandLeavesLargeRubble verifies that a building fully on land leaves large rubble.
func TestBuildingFullyOnLandLeavesLargeRubble(t *testing.T) {
	g := newGameSurf(12, 12, waterAt{}) // all land
	b := addBuilding(g, 1, 5, 5)

	g.DestroyUnit(b)
	if large, ok := g.RubbleAt(5, 5); !ok || !large {
		t.Errorf("a building on land should leave a large pile (large=%v ok=%v)", large, ok)
	}
}

// TestSoldierLeavesNoRubble: commando / infantry leave no wreck even on land.
func TestSoldierLeavesNoRubble(t *testing.T) {
	g := newGameSurf(12, 12, waterAt{}) // all land
	c := game.NewUnitInfo(assets.Commando, 0, 1, 0)
	c.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	c.Values = &game.UnitValues{}
	g.Units.Deploy(c, 5, 5)

	g.DestroyUnit(c)
	if _, ok := g.RubbleAt(5, 5); ok {
		t.Error("a soldier (commando) leaves no wreck")
	}
}
