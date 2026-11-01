// benchslabs_test.go : testing unit for the bench building placement (slabs)
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

// TestBenchBuildingsStandOnSlabs checks that every bench building stands on its slab. Connectors, bridges, and
// water platforms lay none.
func TestBenchBuildingsStandOnSlabs(t *testing.T) {
	for _, tc := range []struct {
		name   string
		deploy func(s *Screen)
	}{
		{"missile", (*Screen).DeployMissileBench},
		{"air", (*Screen).DeployAirBench},
		{"commando", (*Screen).DeployCommandoBench},
		{"headrotate", (*Screen).DeployHeadRotateBench},
		{"aliens", (*Screen).DeployAliensBench},
		{"apcunit", (*Screen).DeployAPCs},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &world.Map{Surface: make([]uint8, 112*112)}
			m.W, m.H = 112, 112
			for i := range m.Surface {
				m.Surface[i] = world.SurfaceLand
			}
			p := &game.StartParams{}
			p.Teams[0].Type = game.TeamPlayer
			g := sim.New(p, m)
			s := &Screen{game: g, wmap: m, r: drawmap.New(m)}
			tc.deploy(s)

			buildings := 0
			for _, u := range g.Units.Units() {
				if !u.Flags.Has(game.FlagStationary) || u.Flags.Has(game.FlagGroundCover) {
					continue
				}
				if u.Type == assets.Cnct4W || u.Type == assets.Bridge || u.Type == assets.WtrPltfm {
					continue
				}
				buildings++
				slab := false
				for _, e := range g.Units.Cells.At(u.GridX, u.GridY) {
					slab = slab || e.Type == assets.SmlSlab || e.Type == assets.LrgSlab
				}
				if !slab {
					t.Errorf("%v at (%d,%d) has no slab", u.Type, u.GridX, u.GridY)
				}
			}
			if buildings == 0 {
				t.Fatal("the bench laid out no building")
			}
		})
	}
}
