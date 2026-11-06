// randomstart_test.go : tests randomstart.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"math/rand"
	"path/filepath"
	"testing"

	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/landmass"
	"github.com/suglasp/max-go/internal/testutil"
	"github.com/suglasp/max-go/internal/world"
)

func TestFootprintsEdgeAdjacent(t *testing.T) {
	// A 2x2 mining station at (5,5)..(6,6); a 1x1 power gen at various cells.
	cases := []struct {
		x, y int
		want bool
		why  string
	}{
		{7, 5, true, "right edge"},
		{7, 6, true, "right edge lower"},
		{4, 5, true, "left edge"},
		{5, 4, true, "top edge"},
		{6, 7, true, "bottom edge"},
		{7, 7, false, "diagonal corner only"},
		{4, 4, false, "diagonal corner only"},
		{8, 5, false, "one-cell gap"},
		{5, 5, false, "overlapping"},
	}
	for _, c := range cases {
		got := footprintsEdgeAdjacent(c.x, c.y, 1, 5, 5, 2)
		if got != c.want {
			t.Errorf("edgeAdjacent 1x1@(%d,%d) vs 2x2@(5,5) = %v, want %v (%s)", c.x, c.y, got, c.want, c.why)
		}
	}
}

func TestAdjacentSitePicksEdgeNeighbour(t *testing.T) {
	placed := []placedBuilding{{x: 5, y: 5, n: 2}}
	x, y, ok := adjacentSite(placed, 1, func(int, int) bool { return true })
	if !ok {
		t.Fatal("no adjacent site found on open ground")
	}
	if !footprintsEdgeAdjacent(x, y, 1, 5, 5, 2) {
		t.Errorf("adjacentSite returned (%d,%d) which is not edge-adjacent to the 2x2", x, y)
	}
	// With nothing valid, it must report failure (caller falls back to nearestCell).
	if _, _, ok := adjacentSite(placed, 1, func(int, int) bool { return false }); ok {
		t.Error("adjacentSite should fail when no cell is valid")
	}
}

// TestLandingSpotPrefersInterior checks that landingSpot avoids the coast: on a
// 7x7 land island in open water, only the centre cell has full clearance, so it
// is chosen over any edge cell.
func TestLandingSpotPrefersInterior(t *testing.T) {
	const w, h = 11, 11
	surf := make([]uint8, w*h)
	var cells []game.Point
	for y := 2; y <= 8; y++ {
		for x := 2; x <= 8; x++ {
			surf[y*w+x] = world.SurfaceLand
			cells = append(cells, game.Point{X: x, Y: y})
		}
	}
	s := &Screen{wmap: &world.Map{Preview: world.Preview{W: w, H: h}, Surface: surf}}
	if got := s.landingSpot(cells, rand.New(rand.NewSource(1))); got.X != 5 || got.Y != 5 {
		t.Errorf("landingSpot = (%d,%d), want the interior (5,5)", got.X, got.Y)
	}
}

// TestStartRoomNeeded pins the land a team needs: the landing force (5 tiles of start buildings + 11 units) plus
// 5 percent of the map's land.
func TestStartRoomNeeded(t *testing.T) {
	if got := landingTiles(); got != 16 {
		t.Errorf("landingTiles() = %d, want 16", got)
	}
	if got := startRoomNeeded(2553); got != 16+127 {
		t.Errorf("startRoomNeeded(2553) = %d, want 143 (GREEN_5)", got)
	}
}

// TestStartSpotsHaveRoomOnRetailMaps lands 2 to 4 teams on every retail map with many seeds: each spot must sit on
// an island holding the needed land, unless the map has no such island at all.
func TestStartSpotsHaveRoomOnRetailMaps(t *testing.T) {
	dir := testutil.DataDir(t)
	files, _ := filepath.Glob(filepath.Join(dir, "*.WRL"))
	if len(files) == 0 {
		t.Skip("no .WRL maps in MAX_DATA_DIR")
	}
	for _, f := range files {
		m, err := world.LoadMap(f)
		if err != nil {
			t.Fatalf("%s: %v", filepath.Base(f), err)
		}
		s := &Screen{wmap: m}
		roomy := s.islandsWithRoom(landmass.Find(m))
		if len(roomy) == 0 {
			t.Errorf("%s: no island has room for a base", filepath.Base(f))
			continue
		}
		island := make(map[game.Point]bool)
		for _, c := range roomy {
			for _, p := range c.Cells {
				island[p] = true
			}
		}
		for teams := 2; teams <= 4; teams++ {
			for seed := int64(0); seed < 25; seed++ {
				for i, p := range s.chooseStartSpots(teams, rand.New(rand.NewSource(seed))) {
					if !island[p] {
						t.Errorf("%s teams=%d seed=%d: team %d lands at (%d,%d), off every island with room", filepath.Base(f), teams, seed, i, p.X, p.Y)
					}
				}
			}
		}
	}
}
