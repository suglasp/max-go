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

// surveyAllBut marks every cell of team 0 surveyed except the listed ones.
func surveyAllBut(g *Game, open ...game.Point) {
	isOpen := make(map[game.Point]bool, len(open))
	for _, p := range open {
		isOpen[p] = true
	}
	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			if !isOpen[game.Point{X: x, Y: y}] {
				g.Resources.MarkSurveyed(x, y, 0)
			}
		}
	}
}

// surveyDestination is where the surveyor's move order ends, or (-1,-1).
func surveyDestination(g *Game, u *game.UnitInfo) game.Point {
	for _, o := range g.MoveOrders() {
		if o.Unit == u && len(o.Steps) > 0 {
			return o.Steps[len(o.Steps)-1].To
		}
	}
	return game.Point{X: -1, Y: -1}
}

// TestSurveyMapsTheBaseFirst checks that a Surveyor maps the ground around its base
// before a nearer cell out in the open, and heads out once the base is mapped.
func TestSurveyMapsTheBaseFirst(t *testing.T) {
	g := newSurveyGame(t, 40, 10)
	radar := game.NewUnitInfo(assets.Radar, 0, 2, 0)
	radar.Flags = game.FlagStationary | game.FlagSelectable
	g.Units.Deploy(radar, 2, 5)

	near, base := game.Point{X: 22, Y: 5}, game.Point{X: 8, Y: 5} // 2 cells away / 12 cells away but 6 from the Radar
	surveyAllBut(g, near, base)
	sur := newSurveyor(g, 20, 5)

	if !g.NearestUnsurveyedMove(sur, 0) {
		t.Fatal("NearestUnsurveyedMove should issue a move")
	}
	if got := surveyDestination(g, sur); got != base {
		t.Fatalf("surveyor heads to %v, want the base cell %v before the nearer open cell", got, base)
	}

	// Base mapped: it heads out to the nearest cell anywhere.
	g.CancelOrder(sur)
	g.Resources.MarkSurveyed(base.X, base.Y, 0)
	if !g.NearestUnsurveyedMove(sur, 0) {
		t.Fatal("NearestUnsurveyedMove should issue a move once the base is mapped")
	}
	if got := surveyDestination(g, sur); got != near {
		t.Errorf("surveyor heads to %v, want the nearest open cell %v", got, near)
	}
}

// TestSurveyBaseBeforeTheFirstBuilding checks that before any building stands the base
// is the landing group (the team's other units).
func TestSurveyBaseBeforeTheFirstBuilding(t *testing.T) {
	g := newSurveyGame(t, 40, 10)
	addTank(g, 2, 6, 2, 5)

	near, base := game.Point{X: 22, Y: 5}, game.Point{X: 8, Y: 5}
	surveyAllBut(g, near, base)
	sur := newSurveyor(g, 20, 5)

	if !g.NearestUnsurveyedMove(sur, 0) {
		t.Fatal("NearestUnsurveyedMove should issue a move")
	}
	if got := surveyDestination(g, sur); got != base {
		t.Errorf("surveyor heads to %v, want %v near the landing group", got, base)
	}
}
