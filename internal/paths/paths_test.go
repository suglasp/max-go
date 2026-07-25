// paths_test.go : testing unit for paths.go
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package paths

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/testutil"
	"github.com/suglasp/max-go/internal/world"
)

// uniform builds a CostFunc that returns the same cost for every cell.
func uniform(c int) CostFunc { return func(x, y int) int { return c } }

// walls builds a CostFunc over a clear field with a set of blocked cells.
func walls(blocked map[game.Point]bool) CostFunc {
	return func(x, y int) int {
		if blocked[game.Point{X: x, Y: y}] {
			return 0
		}
		return int(access.Clear)
	}
}

// colSurface is a synthetic surface: a single column at waterX is open water,
// everything else is land. Used to test amphibious routing across a water gap.
type colSurface struct{ waterX int }

func (c colSurface) SurfaceAt(x, y int) uint8 {
	if x == c.waterX {
		return world.SurfaceWater
	}
	return world.SurfaceLand
}

// TestAmphibiousRoutesAcrossWater checks that an amphibious unit (both movement
// flags) finds a path across a one cell water column between two land cells,
// while a tank that is purely land cannot.
func TestAmphibiousRoutesAcrossWater(t *testing.T) {
	const w, h = 3, 3
	g := &access.Grid{W: w, H: h, Surf: colSurface{waterX: 1}}
	start, dest := game.Point{X: 0, Y: 1}, game.Point{X: 2, Y: 1}

	amFlags := game.FlagMobileLandUnit | game.FlagMobileSeaUnit
	amphib := access.UnitDesc{Type: assets.Scout, Flags: amFlags, LandType: access.LandTypeFor(amFlags)}
	if r, ok := Find(GridCost(g, amphib, 0, access.NoModifiers), w, h, start, dest, 0); !ok {
		t.Error("amphibious unit should route across the water column")
	} else if r.Cost <= 2*int(access.Clear) {
		t.Errorf("amphibious route cost = %d, want it to reflect the costlier water step", r.Cost)
	}

	tank := access.UnitDesc{Type: assets.Tank, Flags: game.FlagMobileLandUnit, LandType: access.LandTypeFor(game.FlagMobileLandUnit)}
	if _, ok := Find(GridCost(g, tank, 0, access.NoModifiers), w, h, start, dest, 0); ok {
		t.Error("a pure-land tank must not cross the water column")
	}
}

// TestSurveyorRoutesAcrossWaterCheap checks that the land-flagged Surveyor crosses water through its land_type at Clear cost, below a dual-flag Scout's amphibious cost.
//
// C++ ported : access.cpp - Access_IsAccessible / accessmap.cpp (the
// "unit_type != SURVEYOR" exclusion from the +8 water penalty) + the Surveyor's
// land_type entry (units.cpp - ParseLandType).
func TestSurveyorRoutesAcrossWaterCheap(t *testing.T) {
	const w, h = 3, 3
	g := &access.Grid{W: w, H: h, Surf: colSurface{waterX: 1}}
	start, dest := game.Point{X: 0, Y: 1}, game.Point{X: 2, Y: 1}

	landFlags := game.FlagMobileLandUnit
	surveyor := access.UnitDesc{Type: assets.Surveyor, Flags: landFlags, LandType: access.LandTypeForUnit(assets.Surveyor, landFlags)}
	survRoute, ok := Find(GridCost(g, surveyor, 0, access.NoModifiers), w, h, start, dest, 0)
	if !ok {
		t.Fatal("Surveyor should route across the water column")
	}

	amFlags := game.FlagMobileLandUnit | game.FlagMobileSeaUnit
	amphib := access.UnitDesc{Type: assets.Scout, Flags: amFlags, LandType: access.LandTypeFor(amFlags)}
	amphibRoute, ok := Find(GridCost(g, amphib, 0, access.NoModifiers), w, h, start, dest, 0)
	if !ok {
		t.Fatal("dual-flag Scout should route across the water column")
	}

	if survRoute.Cost >= amphibRoute.Cost {
		t.Errorf("Surveyor route cost = %d, want strictly less than the dual-flag Scout's %d (Surveyor crosses water at normal cost, not the +8 amphibious penalty)", survRoute.Cost, amphibRoute.Cost)
	}
}

func TestFindStraightLine(t *testing.T) {
	r, ok := Find(uniform(int(access.Clear)), 8, 8, game.Point{X: 1, Y: 1}, game.Point{X: 4, Y: 1}, 0)
	if !ok {
		t.Fatal("expected a path")
	}
	if len(r.Steps) != 3 {
		t.Fatalf("steps = %d, want 3", len(r.Steps))
	}
	if r.Cost != 3*int(access.Clear) {
		t.Errorf("cost = %d, want %d", r.Cost, 3*int(access.Clear))
	}
	for _, s := range r.Steps {
		if s.Dir != 2 { // due east
			t.Errorf("step dir = %d, want 2 (E)", s.Dir)
		}
	}
}

func TestFindDiagonalCheaperThanOrthogonal(t *testing.T) {
	// (0,0) -> (3,3): the diagonal route is 3 steps at (4*3)/2=6 each = 18,
	// cheaper than 6 orthogonal steps at 4 each = 24.
	r, ok := Find(uniform(int(access.Clear)), 8, 8, game.Point{}, game.Point{X: 3, Y: 3}, 0)
	if !ok {
		t.Fatal("expected a path")
	}
	if len(r.Steps) != 3 {
		t.Fatalf("steps = %d, want 3 diagonal", len(r.Steps))
	}
	want := 3 * ((int(access.Clear) * 3) / 2)
	if r.Cost != want {
		t.Errorf("cost = %d, want %d", r.Cost, want)
	}
	for _, s := range r.Steps {
		if s.Dir != 3 { // SE
			t.Errorf("step dir = %d, want 3 (SE)", s.Dir)
		}
	}
}

func TestFindRoutesAroundWall(t *testing.T) {
	// A vertical wall at x=2 for y in 0..3, with a gap at y=4.
	blocked := map[game.Point]bool{}
	for y := 0; y < 4; y++ {
		blocked[game.Point{X: 2, Y: y}] = true
	}
	r, ok := Find(walls(blocked), 8, 8, game.Point{X: 1, Y: 1}, game.Point{X: 3, Y: 1}, 0)
	if !ok {
		t.Fatal("expected a path around the wall")
	}
	for _, s := range r.Steps {
		if blocked[s.To] {
			t.Errorf("path crosses a wall at %v", s.To)
		}
	}
	// The last step must land on the destination.
	if last := r.Steps[len(r.Steps)-1].To; !last.Equal(game.Point{X: 3, Y: 1}) {
		t.Errorf("path ends at %v, want (3,1)", last)
	}
}

func TestFindNoPathWhenWalledOff(t *testing.T) {
	// Fully enclose the destination.
	blocked := map[game.Point]bool{
		{X: 4, Y: 3}: true, {X: 5, Y: 3}: true, {X: 6, Y: 3}: true,
		{X: 4, Y: 4}: true, {X: 6, Y: 4}: true,
		{X: 4, Y: 5}: true, {X: 5, Y: 5}: true, {X: 6, Y: 5}: true,
	}
	if _, ok := Find(walls(blocked), 8, 8, game.Point{X: 0, Y: 0}, game.Point{X: 5, Y: 4}, 0); ok {
		t.Error("expected no path into a walled-off cell")
	}
}

func TestFindRejectsBlockedDestAndOffGrid(t *testing.T) {
	if _, ok := Find(uniform(0), 8, 8, game.Point{X: 1, Y: 1}, game.Point{X: 4, Y: 4}, 0); ok {
		t.Error("blocked destination should not be reachable")
	}
	if _, ok := Find(uniform(int(access.Clear)), 8, 8, game.Point{X: -1, Y: 0}, game.Point{X: 4, Y: 4}, 0); ok {
		t.Error("off-grid start should fail")
	}
}

func TestFindSameCellEmptyPath(t *testing.T) {
	r, ok := Find(uniform(int(access.Clear)), 8, 8, game.Point{X: 2, Y: 2}, game.Point{X: 2, Y: 2}, 0)
	if !ok || len(r.Steps) != 0 || r.Cost != 0 {
		t.Errorf("same-cell path = %+v, %v; want empty zero-cost", r, ok)
	}
}

func TestFindMaxCostRejection(t *testing.T) {
	// A 4-step orthogonal route costs 16; cap below that rejects it.
	if _, ok := Find(uniform(int(access.Clear)), 8, 8, game.Point{X: 0, Y: 0}, game.Point{X: 4, Y: 0}, 10); ok {
		t.Error("path exceeding maxCost should be rejected")
	}
	if _, ok := Find(uniform(int(access.Clear)), 8, 8, game.Point{X: 0, Y: 0}, game.Point{X: 4, Y: 0}, 16); !ok {
		t.Error("path exactly at maxCost should be accepted")
	}
}

// TestFindOnRealMap routes a land unit across a retail world map and checks
// every step is passable for land and the chain is contiguous.
func TestFindOnRealMap(t *testing.T) {
	dir := testutil.DataDir(t)
	m, err := world.LoadMap(filepath.Join(dir, "GREEN_1.WRL"))
	if err != nil {
		t.Skipf("GREEN_1 unavailable: %v", err)
	}

	g := &access.Grid{W: m.W, H: m.H, Surf: m}
	land := access.UnitDesc{Type: assets.Tank, Flags: game.FlagMobileLandUnit, LandType: world.SurfaceLand}
	cost := GridCost(g, land, 0, access.NoModifiers)

	// Find two land cells reasonably far apart.
	var start, dest game.Point
	found := 0
	for y := 0; y < m.H && found < 2; y += 7 {
		for x := 0; x < m.W && found < 2; x += 7 {
			if cost(x, y) > 0 {
				if found == 0 {
					start = game.Point{X: x, Y: y}
				} else {
					dest = game.Point{X: x, Y: y}
				}
				found++
			}
		}
	}
	if found < 2 {
		t.Skip("not enough land cells to route between")
	}

	r, ok := Find(cost, m.W, m.H, start, dest, 0)
	if !ok {
		t.Skipf("no land route between %v and %v (islands)", start, dest)
	}
	prev := start
	for i, s := range r.Steps {
		dx := s.To.X - prev.X
		dy := s.To.Y - prev.Y
		if dx < -1 || dx > 1 || dy < -1 || dy > 1 || (dx == 0 && dy == 0) {
			t.Fatalf("step %d not adjacent: %v -> %v", i, prev, s.To)
		}
		if cost(s.To.X, s.To.Y) == 0 {
			t.Fatalf("step %d enters impassable cell %v", i, s.To)
		}
		prev = s.To
	}
	if !prev.Equal(dest) {
		t.Errorf("path ends at %v, want %v", prev, dest)
	}
}

func TestMain(m *testing.M) { os.Exit(m.Run()) }
