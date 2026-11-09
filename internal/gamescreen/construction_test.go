// construction_test.go : tests construction.go.
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

// TestConstructionWaterVariant drives constructionOnWater - the decision behind the
// raw water construction overlay (tape frame 1, no slab/cones). Raw water/coast reads
// as water; a platform/bridge under the cell makes it read as land (so a build on
// finished platforms keeps the land slab+cones); plain land is land.
func TestConstructionWaterVariant(t *testing.T) {
	const w, h = 16, 16
	m := flatWorld(w, h)

	m.Surface[5*w+5] = world.SurfaceWater
	m.Surface[6*w+6] = world.SurfaceCoast

	p := &game.StartParams{}
	p.Teams[0].Type = game.TeamPlayer
	g := sim.New(p, m)
	s := &Screen{game: g, wmap: m, r: drawmap.New(m)}

	if !s.constructionOnWater(5, 5) {
		t.Error("raw water cell should use the water tape variant")
	}

	if !s.constructionOnWater(6, 6) {
		t.Error("raw coast cell should use the water tape variant")
	}

	if s.constructionOnWater(2, 2) {
		t.Error("plain land cell should use the land slab/cones variant")
	}

	// A water platform on the water cell is the foundation - the cell now reads as land,
	// so the construction overlay falls back to the land slab+cones.
	plat := game.NewUnitInfo(assets.WtrPltfm, 0, 5, 5)
	plat.Flags = game.FlagStationary | game.FlagSelectable
	g.Units.Deploy(plat, 5, 5)

	if s.constructionOnWater(5, 5) {
		t.Error("a water cell carrying a platform should read as land (slab+cones)")
	}
}
