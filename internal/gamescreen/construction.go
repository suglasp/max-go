// construction.go : implements the construction animation.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : unitinfo.cpp - building materialise animation
package gamescreen

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/build"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/world"
)

// drawConstruction paints each active build site with slabs, hazard tape, and warning cones. Building art covers 2x2 footprints; the smaller variants cover one cell.
//
// C++: the ORDER_BUILD site overlay (LRGSLAB/LRGTAPE/... ground covers laid
// while a unit is under construction, units_manager.cpp).
func (s *Screen) drawConstruction(view *gfx.Buffer) {
	z := s.r.Zoom()

	for _, o := range s.game.BuildOrders() {
		if o.Product == nil {
			continue
		}

		// Factories build mobile units internally, so only stationary and ground-cover products receive a construction overlay.
		if !o.Product.Flags.Has(game.FlagStationary) && !o.Product.Flags.Has(game.FlagGroundCover) {
			continue
		}

		fp := build.Footprint(o.Product.Flags)
		slab, tape, cones := assets.SmlSlab, assets.SmlTape, assets.SmlCones

		if o.Product.Flags.Has(game.FlagBuilding) {
			slab, tape, cones = assets.LrgSlab, assets.LrgTape, assets.LrgCones
		}

		// Builds on raw water or coast use water tape without a slab or cones. Finished platforms use the normal land overlay.
		// C++ ported : unitinfo.cpp - UnitInfo::DeployConstructionSiteUtilities (++unit_angle
		// and no cones on SURFACE_TYPE_WATER | SURFACE_TYPE_COAST).
		if s.constructionOnWater(o.GridX, o.GridY) {
			s.sprites.DrawEffect(view, tape, 1, o.GridX, o.GridY, fp, z, s.camera.scrollX, s.camera.scrollY)
			continue
		}

		// Draw the land slab first, then hazard tape and cones.
		s.sprites.DrawEffect(view, slab, 0, o.GridX, o.GridY, fp, z, s.camera.scrollX, s.camera.scrollY)
		s.sprites.DrawEffect(view, tape, 0, o.GridX, o.GridY, fp, z, s.camera.scrollX, s.camera.scrollY)
		s.sprites.DrawEffect(view, cones, 0, o.GridX, o.GridY, fp, z, s.camera.scrollX, s.camera.scrollY)
	}
}

// constructionOnWater reports whether a build site is on raw water or coast without a platform or bridge. These ground covers make the site use the land overlay.
func (s *Screen) constructionOnWater(gx, gy int) bool {
	if s.wmap.SurfaceAt(gx, gy)&(world.SurfaceWater|world.SurfaceCoast) == 0 {
		return false
	}

	for _, u := range s.game.Units.Cells.At(gx, gy) {
		if u.Type == assets.WtrPltfm || u.Type == assets.Bridge {
			return false
		}
	}

	return true
}
