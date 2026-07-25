// terrain.go : M.A.X. tile surface and base posture analysis
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package ai

import (
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/world"
)

// terrainName labels a base's terrain posture for the debug trace.
func terrainName(coastal bool) string {
	if coastal {
		return "coastal"
	}

	return "landlocked"
}

// coastalScanRadius is the maximum distance from a team building to water for a base to count as coastal.
const coastalScanRadius = 6

// baseIsCoastal reports whether any stationary team building lies within coastalScanRadius of water. C++: naval build scoring weighs nearby water.
func baseIsCoastal(g *sim.Game, team int) bool {
	if g == nil || g.Grid == nil || g.Grid.Surf == nil {
		return false
	}

	for _, u := range g.Units.Units() {
		if u.Team != team || !u.Flags.Has(game.FlagStationary) {
			continue
		}

		for dy := -coastalScanRadius; dy <= coastalScanRadius; dy++ {
			for dx := -coastalScanRadius; dx <= coastalScanRadius; dx++ {
				x, y := u.GridX+dx, u.GridY+dy
				if x < 0 || y < 0 || x >= g.W || y >= g.H {
					continue
				}

				if g.Grid.Surf.SurfaceAt(x, y)&world.SurfaceWater != 0 {
					return true
				}
			}
		}
	}

	return false
}
