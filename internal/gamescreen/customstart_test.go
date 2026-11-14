// customstart_test.go : tests customstart.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"math/rand"
	"path/filepath"
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/testutil"
	"github.com/suglasp/max-go/internal/world"
)

// TestCustomStartComputerLandsClearOfTheHuman lands the human right on the spot the
// computer team preferred: the computer must land somewhere else, its Mining Station well
// clear of the human's (the computer spots were fixed before the human picked, N1).
func TestCustomStartComputerLandsClearOfTheHuman(t *testing.T) {
	for seed := int64(1); seed <= 3; seed++ {
		s, g := newTestScreen(t, 112)
		s.SetCustomOpponents(1)
		s.CustomStart(nil, nil, 100, seed)

		// The same first draw CustomStart made: team 1's preferred spot.
		preferred := s.chooseStartSpots(2, rand.New(rand.NewSource(seed)))[1]
		s.siteSelect.confirm(preferred.X, preferred.Y)

		station := map[int]game.Point{}

		for _, u := range g.Units.Units() {
			if u.Type == assets.MiningSt {
				station[int(u.Team)] = game.Point{X: u.GridX, Y: u.GridY}
			}
		}

		human, computer := station[0], station[1]
		if d := max(iabsInt(human.X-computer.X), iabsInt(human.Y-computer.Y)); d <= landingSafeRange-baseSlack {
			t.Errorf("seed %d: the computer station at (%d,%d) is %d cells from the human's at (%d,%d)",
				seed, computer.X, computer.Y, d, human.X, human.Y)
		}
	}
}

// baseSlack is how far a base's Mining Station may sit from its landing spot in the test above.
const baseSlack = 4

// TestPlaceComputerSpotsKeepsTheSafeRange puts the human on a computer team's preferred spot
// on an open map: every computer team lands more than landingSafeRange cells from the human
// and from the computer teams before it, on at least one axis.
func TestPlaceComputerSpotsKeepsTheSafeRange(t *testing.T) {
	s, _ := newTestScreen(t, 112)

	for teams := 2; teams <= 4; teams++ {
		for seed := int64(0); seed < 5; seed++ {
			rng := rand.New(rand.NewSource(seed))
			spot := s.chooseStartSpots(teams, rng)
			spot[0] = spot[1]
			s.placeComputerSpots(spot, rng)

			for team := 1; team < teams; team++ {
				if !landingClear(spot[team], spot[:team]) {
					t.Errorf("teams=%d seed=%d: team %d at (%d,%d) is %d cells from a team already down",
						teams, seed, team, spot[team].X, spot[team].Y, nearestTeamRange(spot[team], spot[:team]))
				}
			}
		}
	}
}

// TestPlaceComputerSpotsOnASmallMap lands four teams on a map too small to keep the safe range:
// the pick still returns, on land and off the human's spot.
func TestPlaceComputerSpotsOnASmallMap(t *testing.T) {
	s, _ := newTestScreen(t, 16)
	rng := rand.New(rand.NewSource(7))
	spot := s.chooseStartSpots(4, rng)
	spot[0] = game.Point{X: 8, Y: 8}
	s.placeComputerSpots(spot, rng)

	for team := 1; team < 4; team++ {
		p := spot[team]
		if s.wmap.SurfaceAt(p.X, p.Y)&(world.SurfaceLand|world.SurfaceCoast) == 0 {
			t.Errorf("team %d lands off land at (%d,%d)", team, p.X, p.Y)
		}

		if p == spot[0] {
			t.Errorf("team %d lands on the human's spot (%d,%d)", team, p.X, p.Y)
		}
	}
}

// TestPlaceComputerSpotsOnRetailMaps puts the human on a computer team's preferred spot on
// every retail map: each computer team lands on land, and clear of the teams already down
// whenever the map has a clear land cell at all.
func TestPlaceComputerSpotsOnRetailMaps(t *testing.T) {
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
		land := s.landCells()

		for teams := 2; teams <= 4; teams++ {
			for seed := int64(0); seed < 3; seed++ {
				rng := rand.New(rand.NewSource(seed))
				spot := s.chooseStartSpots(teams, rng)
				spot[0] = spot[1]
				s.placeComputerSpots(spot, rng)

				for team := 1; team < teams; team++ {
					p, placed := spot[team], spot[:team]

					if m.SurfaceAt(p.X, p.Y)&(world.SurfaceLand|world.SurfaceCoast) == 0 {
						t.Errorf("%s teams=%d seed=%d: team %d lands off land at (%d,%d)", filepath.Base(f), teams, seed, team, p.X, p.Y)
					}

					if !landingClear(p, placed) && len(clearCells(land, placed)) > 0 {
						t.Errorf("%s teams=%d seed=%d: team %d at (%d,%d) is %d cells from a team already down, a clear cell existed",
							filepath.Base(f), teams, seed, team, p.X, p.Y, nearestTeamRange(p, placed))
					}
				}
			}
		}
	}
}
