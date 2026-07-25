// connectorchain_test.go : tests connector auto chain placement and network adjacency rules.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/drawmap"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/world"
)

// TestLayConnectorChainClosesLinkAroundWater reproduces the -random2 detached shipyard
// bug: a water inlet sits on the straight line between the dock and the base, so the old
// L shaped connector run clipped the water, left a gap and never merged the shipyard into
// the base complex. The BFS path must route around the inlet over land and close the
// link, so the shipyard and the base mining station end up in ONE complex (it powers).
func TestLayConnectorChainClosesLinkAroundWater(t *testing.T) {
	const w, h = 24, 24

	m := &world.Map{Surface: make([]uint8, w*h)}
	m.W, m.H = w, h

	for i := range m.Surface {
		m.Surface[i] = world.SurfaceLand
	}

	// A vertical water inlet down column 10, leaving a land crossing only on the bottom
	// two rows - the straight dock->base path (along row 10) is blocked; a detour exists.
	for y := 0; y < h-2; y++ {
		m.Surface[y*w+10] = world.SurfaceWater
	}

	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.New(p, m)
	s := &Screen{game: g, wmap: m, r: drawmap.New(m)}

	bld := game.FlagStationary | game.FlagBuilding | game.FlagRequiresSlab | game.FlagSelectable
	taken := map[game.Point]bool{}

	// Base mining station on the left of the inlet; shipyard on the right.
	s.placeStartBuilding(0, assets.MiningSt, bld, 4, 10)
	markFootprint(taken, 4, 10, 2)
	placed := []placedBuilding{{x: 4, y: 10, n: 2}}
	s.placeStartBuilding(0, assets.Shipyard, bld, 14, 10)
	markFootprint(taken, 14, 10, 2)

	s.layConnectorChain(0, 14, 10, placed, taken)

	// The shipyard's complex must also contain the base mining station.
	var ship, mine *game.UnitInfo

	for _, u := range s.game.Units.Units() {
		switch u.Type {
		case assets.Shipyard:
			ship = u

		case assets.MiningSt:
			mine = u
		}
	}

	if ship == nil || mine == nil {
		t.Fatalf("missing units: shipyard=%v mine=%v", ship != nil, mine != nil)
	}

	for _, c := range s.game.Complexes {
		hasShip, hasMine := false, false
		for _, u := range c.Members {
			if u == ship {
				hasShip = true
			}

			if u == mine {
				hasMine = true
			}
		}

		if hasShip {
			if !hasMine {
				t.Fatal("connector chain did not close the link: shipyard and base mine are in different complexes")
			}

			return // linked: shipyard and mine share a complex
		}
	}

	t.Fatal("shipyard is in no complex at all")
}
