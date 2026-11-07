// resourceoverlay.go : implements the resource map overlay.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"fmt"

	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/vis"
)

// surveyScanRadius is the reveal radius a resource surveying unit sweeps around
// itself each frame as it moves: 1 cell, i.e. its own tile plus the 8 neighbours
// (a 3x3, 9 tile block). The original calls Survey_SurveyArea(unit, 1) from
// Access_UpdateMapStatus on every position update, so the reveal grows one 3x3
// stamp per cell visited as the unit drives, never a wide instant sweep.
// C++ ported : access.cpp - Access_UpdateMapStatus / Survey_SurveyArea(unit, 1)
const surveyScanRadius = 1

// isResourceSurveyUnit reports whether moving this unit type reveals the
// resource map around itself.
//
// C++ ported : access.cpp - Access_UpdateMapStatus
func isResourceSurveyUnit(t assets.UnitType) bool {
	return t == assets.Surveyor || t == assets.MineLayr || t == assets.SeaMnLyr || t == assets.Commando
}

// surveyTick reveals resource deposits around every resource surveying unit at
// its current cell, so the survey overlay fills in as the unit drives over the
// map. MarkSurveyed is idempotent, so running it each frame is cheap and keeps
// the reveal live.
//
// C++ ported : access.cpp - Access_UpdateMapStatus
func (s *Screen) surveyTick() {
	if s.game.Resources == nil {
		return
	}

	for _, u := range s.game.Units.Units() {
		if isResourceSurveyUnit(u.Type) {
			s.game.Resources.Survey(u.GridX, u.GridY, surveyScanRadius, u.Team)
		}
	}
}

// isReconUnit reports whether a unit's whole job is revealing the map; the
// scanner, the AWAC, and the radar building. Selecting one shows its scan range
// ring, the way selecting a surveyor shows the resource overlay.
func isReconUnit(t assets.UnitType) bool {
	return t == assets.Scanner || t == assets.Awac || t == assets.Radar
}

// drawScanRange outlines the selected recon unit's scan radius (yellow ring,
// unit at the centre); a visual of what it reveals. C++: GameManager_DrawCircle
// with ATTRIB_SCAN, COLOR_YELLOW (shown for the selected unit under the scan
// display); here it shows on selecting a recon unit, no toggle needed.
func (s *Screen) drawScanRange(view *gfx.Buffer) {
	u := s.selected

	if u == nil || u.Values == nil || !isReconUnit(u.Type) {
		return
	}

	scan := u.Values.Get(game.AttribScan)

	if scan <= 0 {
		return
	}

	const tile = 64
	z := s.r.Zoom()
	cellX := u.X*z/tile - s.camera.scrollX
	cellY := u.Y*z/tile - s.camera.scrollY
	view.DrawCircleClip(cellX, cellY, scan*z, gfx.ColorYellow) // radius = scan tiles -> screen px
}

// resourceOverlayVisible reports whether the survey overlay should draw: the M
// debug reveal (maxSurvey) is on, the SURVEY panel toggle is on, or a surveyor
// of the player team is selected. The maxSurvey debug additionally bypasses the
// surveyed cell gate inside drawResourceOverlay; the SURVEY toggle does not, so
// it only reveals deposits the team has actually surveyed.
// C++: GameManager_RenderSurveyIndicator (DisplayButtonSurvey || surveyor selected).
func (s *Screen) resourceOverlayVisible() bool {
	if s.maxSurvey || s.surveyOverlay {
		return true
	}

	u := s.selected
	return u != nil && u.Type == assets.Surveyor
}

// drawResourceOverlay paints the resource markers (GOLDMK/FUELMK/RAWMSK tiles,
// frame = min(amount,16)) over each cell holding a deposit the player has
// surveyed, or every deposit under the maxSurvey debug. The marker is centred on
// the cell and scaled to the zoom. Z order: above the units, under the fog.
//
// C++: DrawMap_RenderSurveyDisplay / Survey_RenderMarker (drawmap.cpp / survey.cpp).
func (s *Screen) drawResourceOverlay(view *gfx.Buffer) {
	if s.game.Resources == nil || !s.resourceOverlayVisible() {
		return
	}

	z := s.r.Zoom()
	team := s.game.State.PlayerTeam
	tx0 := max(0, min(s.camera.scrollX/z, s.wmap.W-1))
	ty0 := max(0, min(s.camera.scrollY/z, s.wmap.H-1))
	tx1 := max(0, min((s.camera.scrollX+view.W-1)/z, s.wmap.W-1))
	ty1 := max(0, min((s.camera.scrollY+view.H-1)/z, s.wmap.H-1))

	for ty := ty0; ty <= ty1; ty++ {
		for tx := tx0; tx <= tx1; tx++ {
			if !s.maxSurvey && !s.game.Resources.IsSurveyed(tx, ty, team) {
				continue
			}

			t, amount := s.game.Resources.DepositAt(tx, ty)
			var tag string

			switch {
			case t != economy.CargoNone:
				// The cell's resource type at its amount: GOLDMK<n> / FUELMK<n> /
				// RAWMSK<n>, including the n=0 empty circle for a tapered to zero
				// region cell. C++: Survey_RenderMarker (marker_big + amount).
				tag = resourceTileTag(t, amount)
			case z > 16:
				// A cell with no resource type at all still gets the raw empty circle
				// at the larger zooms, marking the tile as surveyed (the small <=16
				// markers skip empties). C++: Survey_RenderMarker default raw.
				tag = resourceTileTag(economy.CargoRaw, 0) // RAWMSK0
			default:
				continue
			}

			tile := s.resourceTile(tag, z)

			if tile == nil {
				continue
			}

			// Centre the (zoom scaled) marker on the cell, matching the original's
			// decode at the cell centre.
			cellX := tx*z - s.camera.scrollX + z/2
			cellY := ty*z - s.camera.scrollY + z/2
			blitTransClipped(view, tile, cellX-tile.W/2, cellY-tile.H/2)
		}
	}
}

// resourceTileTag is the RES marker tag for a deposit type at the given amount
// (0..16): the "big" markers used at the normal play zooms. Raw is RAWMSK (not
// RAWMK: the tag many search for and miss). C++: Survey_RenderMarker (marker_big).
func resourceTileTag(t economy.CargoType, amount int) string {
	if amount < 0 {
		amount = 0
	}

	if amount > 16 {
		amount = 16
	}

	switch t {
	case economy.CargoGold:
		return fmt.Sprintf("GOLDMK%d", amount)
	case economy.CargoFuel:
		return fmt.Sprintf("FUELMK%d", amount)
	default:
		return fmt.Sprintf("RAWMSK%d", amount)
	}
}

// resourceTile returns the marker image for tag scaled to a z pixel cell, caching
// both the native image (loaded once; nil when absent) and the per zoom scaled
// copy. A native marker is sized for a 64px cell, so it scales by z/64.
func (s *Screen) resourceTile(tag string, z int) *gfx.Buffer {
	if s.mapArt.resTileScaled == nil {
		s.mapArt.resTileScaled = map[string]*gfx.Buffer{}
		s.mapArt.resTileNative = map[string]*gfx.Buffer{}
	}

	key := fmt.Sprintf("%s@%d", tag, z)

	if b, ok := s.mapArt.resTileScaled[key]; ok {
		return b
	}

	native, seen := s.mapArt.resTileNative[tag]

	if !seen {
		native = loadSimpleArt(s.lib, tag) // may be nil if the tag is absent

		if native != nil {
			// MAX overlaps an image's transparent_color onto its first pixel, so
			// the marker's transparent (fuchsia/blue) areas are keyed off At(0,0)
			// to 0; without this they blit as solid blue. C++: ImageSimpleHeader
			// transparent_color.
			native.KeyToZero(native.At(0, 0))
		}

		s.mapArt.resTileNative[tag] = native
	}

	var scaled *gfx.Buffer

	switch {
	case native == nil:
		scaled = nil
	case z >= 64:
		scaled = native
	default:
		w, h := native.W*z/64, native.H*z/64

		if w < 1 {
			w = 1
		}

		if h < 1 {
			h = 1
		}

		scaled = gfx.New(w, h)
		scaled.Scale(native)
	}

	s.mapArt.resTileScaled[key] = scaled
	return scaled
}

// blitTransClipped copies src into dst at (x, y) with colour 0 transparent,
// clipping to dst's bounds (and offsetting into src for negative x/y).
func blitTransClipped(dst, src *gfx.Buffer, x, y int) {
	srcX, srcY := 0, 0

	if x < 0 {
		srcX, x = -x, 0
	}

	if y < 0 {
		srcY, y = -y, 0
	}

	w := min(src.W-srcX, dst.W-x)
	h := min(src.H-srcY, dst.H-y)

	if w <= 0 || h <= 0 {
		return
	}

	dst.Sub(x, y, w, h).TransCopy(src.Sub(srcX, srcY, w, h))
}

// drawAccessOverlay outlines, in red, every visible cell the selected unit
// cannot enter: a live read of internal/access against the map terrain
// and the deployed units. C++: the AccessMap debug visualisation.
func (s *Screen) drawAccessOverlay(view *gfx.Buffer) {
	u := s.selected
	d := access.UnitDesc{Type: u.Type, Flags: u.Flags, LandType: access.LandTypeForUnit(u.Type, u.Flags)}
	z := s.r.Zoom()
	tx0 := max(0, min(s.camera.scrollX/z, s.wmap.W-1))
	ty0 := max(0, min(s.camera.scrollY/z, s.wmap.H-1))
	tx1 := max(0, min((s.camera.scrollX+view.W-1)/z, s.wmap.W-1))
	ty1 := max(0, min((s.camera.scrollY+view.H-1)/z, s.wmap.H-1))

	for ty := ty0; ty <= ty1; ty++ {
		for tx := tx0; tx <= tx1; tx++ {
			if s.game.Grid.IsAccessible(d, u.Team, tx, ty, access.SameClassBlocks).Passable() {
				continue
			}

			startX := max(0, min(tx*z-s.camera.scrollX, view.W-1))
			startY := max(0, min(ty*z-s.camera.scrollY, view.H-1))
			endX := max(0, min(tx*z-s.camera.scrollX+z-1, view.W-1))
			endY := max(0, min(ty*z-s.camera.scrollY+z-1, view.H-1))
			view.DrawBox(startX, startY, endX, endY, gfx.ColorRed)
		}
	}
}

// drawFog paints the player team fog of war: never scanned cells are blacked
// out, explored but unscanned cells dithered. Recomputes the team's heat map
// from the live unit set first. C++: Access_UpdateMinimapFogOfWar.
func (s *Screen) drawFog(view *gfx.Buffer) {
	s.game.RefreshVisibility()
	m := s.game.Vis.Team(s.fogTeam)

	if m == nil {
		return
	}

	z := s.r.Zoom()
	tx0 := max(0, min(s.camera.scrollX/z, s.wmap.W-1))
	ty0 := max(0, min(s.camera.scrollY/z, s.wmap.H-1))
	tx1 := max(0, min((s.camera.scrollX+view.W-1)/z, s.wmap.W-1))
	ty1 := max(0, min((s.camera.scrollY+view.H-1)/z, s.wmap.H-1))

	for ty := ty0; ty <= ty1; ty++ {
		for tx := tx0; tx <= tx1; tx++ {
			fog := m.FogAt(tx, ty)

			if fog == vis.FogVisible {
				continue
			}

			startX := max(0, min(tx*z-s.camera.scrollX, view.W-1))
			startY := max(0, min(ty*z-s.camera.scrollY, view.H-1))
			endX := max(0, min(tx*z-s.camera.scrollX+z-1, view.W-1))
			endY := max(0, min(ty*z-s.camera.scrollY+z-1, view.H-1))

			if fog == vis.FogUnseen {
				view.Sub(startX, startY, endX-startX+1, endY-startY+1).Fill(gfx.ColorBlack)
			} else {
				ditherFill(view, startX, startY, endX, endY, gfx.ColorBlack)
			}
		}
	}
}

// ditherFill blacks out every other pixel in a checkerboard, darkening the
// terrain underneath without fully hiding it (explored cell look).
func ditherFill(view *gfx.Buffer, startX, startY, endX, endY int, color uint8) {
	for y := startY; y <= endY; y++ {
		for x := startX + (y+startX)&1; x <= endX; x += 2 {
			view.Set(x, y, color)
		}
	}
}
