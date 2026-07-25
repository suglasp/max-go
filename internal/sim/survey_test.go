// survey_test.go : tests survey.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// Note: Tests for NearestUnsurveyedMove (sim/survey.go).
package sim

import (
	"testing"

	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/units"
)

// newSurveyGame builds a minimal game with a resource map so NearestUnsurveyedMove has
// something to work with.
func newSurveyGame(t *testing.T, w, h int) *Game {
	t.Helper()
	mgr := units.NewManager()
	grid := &access.Grid{W: w, H: h, Surf: allLand{}, Units: mgr.Cells}
	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := NewWithGrid(game.NewGameState(p), mgr, grid, w, h)
	g.Resources = economy.NewResourceMap(w, h)
	return g
}

// newSurveyor deploys a Surveyor unit for team 0 at the given cell.
func newSurveyor(g *Game, gx, gy int) *game.UnitInfo {
	u := game.NewUnitInfo(assets.Surveyor, 0, 1, 0)
	u.Flags = game.FlagMobileLandUnit | game.FlagSelectable
	u.Values = &game.UnitValues{}
	u.Values.Set(game.AttribSpeed, 6)
	u.Speed = 6
	g.Units.Deploy(u, gx, gy)
	return u
}

// TestNearestUnsurveyedMoveTargetsUnsurveyed confirms that a Surveyor with an
// unsurveyed cell nearby gets a move order toward it.
func TestNearestUnsurveyedMoveTargetsUnsurveyed(t *testing.T) {
	g := newSurveyGame(t, 16, 16)
	// Mark most cells surveyed except a small region at the far corner.
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			// Leave (14,14) unsurveyed so the surveyor has somewhere to go.
			if x != 14 || y != 14 {
				g.Resources.MarkSurveyed(x, y, 0)
			}
		}
	}
	sur := newSurveyor(g, 0, 0)
	// NearestUnsurveyedMove should issue a move order for the surveyor.
	if !g.NearestUnsurveyedMove(sur, 0) {
		t.Fatal("NearestUnsurveyedMove should return true when an unsurveyed cell exists")
	}
	if !g.HasOrder(sur) {
		t.Error("the surveyor should have a move order after NearestUnsurveyedMove")
	}
}

// TestNearestUnsurveyedMoveReturnsFalseWhenFullySurveyed confirms the return value
// when every reachable cell is already surveyed (nothing left to reveal).
func TestNearestUnsurveyedMoveReturnsFalseWhenFullySurveyed(t *testing.T) {
	g := newSurveyGame(t, 8, 8)
	// Mark every cell surveyed for team 0.
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			g.Resources.MarkSurveyed(x, y, 0)
		}
	}
	sur := newSurveyor(g, 0, 0)
	// Nothing to survey: should return false and issue no order.
	if g.NearestUnsurveyedMove(sur, 0) {
		t.Error("NearestUnsurveyedMove should return false when all cells are surveyed")
	}
	if g.HasOrder(sur) {
		t.Error("no move order should be issued when nothing is left to survey")
	}
}

// TestNearestUnsurveyedMoveNoResourceMapReturnsFalse confirms the nil resource map
// guard (headless harness without a resource map).
func TestNearestUnsurveyedMoveNoResourceMapReturnsFalse(t *testing.T) {
	g := newGame(t, 8, 8) // no resource map
	sur := newSurveyor(g, 0, 0)
	if g.NearestUnsurveyedMove(sur, 0) {
		t.Error("NearestUnsurveyedMove should return false with no resource map")
	}
}
