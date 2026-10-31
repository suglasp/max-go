// minimap.go : implements the minimap renderer.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : game_manager.cpp - minimap drawing and click navigation
package gamescreen

import (
	"sort"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/drawmap"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/platform"
)

// minimapState holds minimap toggles, controls, drawing buffers, and drag recenter state.
type minimapState struct {
	minimap2x, minimapTnt        bool        // minimap 2X zoom / TNT (combat units) toggle state
	mmX2Off, mmX2On              *gfx.Buffer // 2X button off/on face
	mmTntOff, mmTntOn            *gfx.Buffer // TNT button off/on face
	zoomTrack, zoomKnob          *gfx.Buffer // zoom slider track (ZOOMPNL1) / knob (ZOOMPTR)
	minimapDots                  [4]uint8    // per team dot colour
	minimapBuf                   *gfx.Buffer // reused minimap scratch buffer (avoids per frame allocation)
	minimapDrag                  bool        // freezes the 2X overview origin for a left drag recenter (no feedback drift)
	minimapDragOX, minimapDragOY int         // frozen source origin captured on the press edge
}

// dotColor returns a team's minimap dot colour. Teams outside the configured range use yellow. C++:
// HASH_TEAM_* -> COLOR_* mapping.
func (s *Screen) dotColor(team int) uint8 {
	if team >= 0 && team < len(s.minimap.minimapDots) && s.minimap.minimapDots[team] != 0 {
		return s.minimap.minimapDots[team]
	}

	return gfx.ColorYellow
}

// isCombatUnit reports whether a unit has an attack value and passes the TNT minimap filter.
//
// C++: flags & (HAS_FIRING_SPRITE | FIRES_MISSILES) (drawmap.cpp).
func isCombatUnit(u *game.UnitInfo) bool {
	return u.Values != nil && u.Values.Get(game.AttribAttack) > 0
}

// isDefenseBuilding reports whether u is a stationary armed structure. It gates the Sentry popup toggle and
// default Sentry deploy behaviour.
func isDefenseBuilding(u *game.UnitInfo) bool {
	return u != nil && u.Flags.Has(game.FlagStationary) && isCombatUnit(u)
}

// drawMinimap renders terrain, visible team-coloured unit dots, and the viewport box. It applies 2X zoom when enabled.
func (s *Screen) drawMinimap(buf *gfx.Buffer) {
	mmW, mmH := viewSize(Minimap)

	// MinimapTerrain rewrites the reused scratch buffer every frame.
	if s.minimap.minimapBuf == nil {
		s.minimap.minimapBuf = gfx.New(mmW, mmH)
	}

	tmp := s.minimap.minimapBuf
	s.r.MinimapTerrain(tmp)
	s.drawMinimapUnits(tmp, mmW, mmH)

	dst := buf.Sub(Minimap.ULX, Minimap.ULY, mmW, mmH)

	if s.minimap.minimap2x {
		// Zoom mode shows no viewport box (it would cover most of the view).
		s.minimapZoom2x(dst, tmp, mmW, mmH)
	} else {
		vw, vh := viewSize(MainMap)
		s.r.MinimapBox(tmp, s.camera.scrollX, s.camera.scrollY, vw, vh, gfx.ColorRed)
		dst.Copy(tmp)
	}
}

// drawMinimapUnits stamps visible units as team-coloured footprint blocks. It uses the human fog view so
// computer turns cannot reveal their units; disabling fog reveals every unit.
func (s *Screen) drawMinimapUnits(dst *gfx.Buffer, mmW, mmH int) {
	viewer := s.fogTeam // the human's view, same team the main map fog uses
	view := s.game.Vis.Team(viewer)
	// Draw a stable order to prevent overlapping colours flickering. Buildings come first, then mobile units,
	// with IDs breaking ties.
	order := s.game.Units.Units()
	sort.SliceStable(order, func(i, j int) bool {
		mi, mj := isMobileUnit(order[i]), isMobileUnit(order[j])

		if mi != mj {
			return !mi // stationary before mobile
		}

		return order[i].ID < order[j].ID
	})

	for _, u := range order {
		if u.Flags.Has(game.FlagGroundCover) {
			continue // slabs / foundations are terrain, not units
		}

		if u.Type == assets.Waldo || u.Type == assets.Harvster {
			continue // hidden gag props: never blip on the minimap, the surprise is finding them on the map itself
		}

		if u.HiddenUntilExpand {
			continue // a just finished structure, still hidden behind its parked builder
		}

		// Fog off (F2 debug) reveals every blip; otherwise own units always show and
		// other teams only where the human is currently scanning.
		visible := !s.fogShow || u.Team == viewer || (view != nil && view.Complete(u.GridX, u.GridY) > 0)

		if !visible {
			continue
		}

		if s.minimap.minimapTnt && !isCombatUnit(u) {
			continue
		}

		s.stampUnit(dst, u, mmW, mmH)
	}
}

// stampUnit fills a 2x2 cell blip on the minimap with the unit's team colour, scaled to minimap pixels, at
// least one pixel. In the 2X minimap view a unit that is not a building is one cell. A 2x2 blip at the right
// or bottom edge shifts inward to stay on the map. No outline, pure team colour pixels.
// C++ ported : drawmap.cpp - DrawMap_RenderMiniMapUnitList (klei 23/08/26; was a 2x2 blip for buildings only)
func (s *Screen) stampUnit(dst *gfx.Buffer, u *game.UnitInfo, mmW, mmH int) {
	cells := minimapBlipCells(u, s.minimap.minimap2x)
	gridX, gridY := u.GridX, u.GridY

	if cells == 2 {
		gridX = min(gridX, s.wmap.W-2)
		gridY = min(gridY, s.wmap.H-2)
	}

	startX := gridX * mmW / s.wmap.W
	startY := gridY * mmH / s.wmap.H
	endX := (gridX + cells) * mmW / s.wmap.W
	endY := (gridY + cells) * mmH / s.wmap.H

	if endX <= startX {
		endX = startX + 1
	}

	if endY <= startY {
		endY = startY + 1
	}

	col := s.dotColor(u.Team)

	for y := startY; y < endY && y < dst.H; y++ {
		if y < 0 {
			continue
		}

		for x := startX; x < endX && x < dst.W; x++ {
			if x >= 0 {
				dst.Set(x, y, col)
			}
		}
	}
}

// minimapZoom2x copies the half of src centred on the viewport, nearest scaled 2x,
// into dst (so the overview shows the area around the current viewport at
// double resolution).
//
// C++: the GameManager_DisplayButtonMinimap2x branch (game_manager.cpp).
func (s *Screen) minimapZoom2x(dst, src *gfx.Buffer, mmW, mmH int) {
	ox, oy := s.minimap2xOrigin(mmW, mmH)

	for dy := 0; dy < mmH; dy++ {
		srcY := oy + dy/2

		for dx := 0; dx < mmW; dx++ {
			dst.Set(dx, dy, src.At(ox+dx/2, srcY))
		}
	}
}

// minimap2xOrigin returns the top left SOURCE minimap pixel of the 2X overview
// region, centred on the current viewport. It is shared by the zoom renderer
// (minimapZoom2x) and the click->cell inverse (minimapCellAt) so a click maps
// back to exactly the cell drawn under it.
func (s *Screen) minimap2xOrigin(mmW, mmH int) (int, int) {
	vw, vh := viewSize(MainMap)
	mw, mh := s.r.MapPixels()
	cellX := (s.camera.scrollX + vw/2) * mmW / mw
	cellY := (s.camera.scrollY + vh/2) * mmH / mh
	ox := clamp(cellX-mmW/4, 0, mmW-mmW/2)
	oy := clamp(cellY-mmH/4, 0, mmH-mmH/2)
	return ox, oy
}

// minimapCellAt maps a minimap click to a map cell and inverts the 2X overview transform when needed. It
// returns false outside the minimap.
func (s *Screen) minimapCellAt(mx, my int) (int, int, bool) {
	if !s.minimap.minimap2x {
		return minimapCell(mx, my, s.wmap.W, s.wmap.H)
	}

	mmW, mmH := viewSize(Minimap)
	ox, oy := s.minimap2xOrigin(mmW, mmH)
	return s.minimapCellAtOrigin(mx, my, ox, oy)
}

// minimapCellAtOrigin maps a 2X click with a supplied overview origin. Drag recenter freezes that origin at
// press time so the target cell remains stable.
func (s *Screen) minimapCellAtOrigin(mx, my, ox, oy int) (int, int, bool) {
	if mx < Minimap.ULX || mx > Minimap.LRX || my < Minimap.ULY || my > Minimap.LRY {
		return 0, 0, false // not on the minimap
	}

	mmW, mmH := viewSize(Minimap)
	// Each dst pixel covers half a source pixel; fold the click back to the
	// source minimap, then map that source pixel to a map cell.
	srcX := ox + (mx-Minimap.ULX)/2
	srcY := oy + (my-Minimap.ULY)/2
	cellX := srcX * s.wmap.W / mmW
	cellY := srcY * s.wmap.H / mmH
	return clamp(cellX, 0, s.wmap.W-1), clamp(cellY, 0, s.wmap.H-1), true
}

// drawMinimapButtons draws the 2X and TNT toggle buttons (pressed art when
// active).
func (s *Screen) drawMinimapButtons(buf *gfx.Buffer) {
	blitArt(buf, onOff(s.minimap.minimap2x, s.minimap.mmX2On, s.minimap.mmX2Off), MinimapX2.ULX, MinimapX2.ULY)
	blitArt(buf, onOff(s.minimap.minimapTnt, s.minimap.mmTntOn, s.minimap.mmTntOff), MinimapTnt.ULX, MinimapTnt.ULY)
}

// drawZoomSlider draws the zoom track and its knob at the current zoom.
func (s *Screen) drawZoomSlider(buf *gfx.Buffer) {
	blitArt(buf, s.minimap.zoomTrack, ZoomTrack.ULX, ZoomTrack.ULY)

	if s.minimap.zoomKnob == nil {
		return
	}

	// Centre the knob on its offset and clamp it inside the track, so it never
	// overshoots the right edge (which the track redraw can't erase, leaving a
	// half knob ghost).
	kw := s.minimap.zoomKnob.W
	x := clamp(ZoomTrack.ULX+zoomKnobOffset(s.r.Zoom())-kw/2, ZoomTrack.ULX, ZoomTrack.LRX-kw+1)
	blitArtKeyed(buf, s.minimap.zoomKnob, x, ZoomTrack.ULY+1)
}

// zoomKnobOffset is the knob's x within the track for a zoom level: full
// zoom (MaxZoom) sits at the left, minimum zoom at the right.
//
// C++: GameManager_UpdateZoomSlider (game_manager.cpp).
func zoomKnobOffset(zoom int) int {
	const knobW = 28
	span := (ZoomTrack.LRX - ZoomTrack.ULX) - knobW

	if span < 0 {
		span = 0
	}

	return span*(drawmap.MaxZoom-zoom)/(drawmap.MaxZoom-drawmap.MinZoom) + knobW/2
}

// zoomFromSliderX maps a click x on the slider to a zoom level
// (MinZoom..MaxZoom); curZoom is returned when the track is degenerate.
//
// C++: the WINDOW_ZOOM_SLIDER_BUTTON case (game_manager.cpp).
func zoomFromSliderX(mx, curZoom int) int {
	span := (ZoomSlider.LRX - ZoomSlider.ULX) - 30

	if span <= 0 {
		return curZoom
	}

	x := clamp(mx-(ZoomSlider.ULX+14), 0, span)
	return drawmap.MaxZoom - x*(drawmap.MaxZoom-drawmap.MinZoom)/span
}

// handleMinimapZoomControls processes clicks on the 2X / TNT buttons and the
// zoom +/- buttons and slider. Returns true when it consumed the input.
func (s *Screen) handleMinimapZoomControls(mouse platform.MouseState) bool {
	if s.turnFlow.handover != -1 || s.gameOver {
		return false
	}

	// Holding the left button on the track sets the zoom continuously (drag).
	if mouse.Buttons&platform.MousePressLeft != 0 && inRect(ZoomSlider, mouse.X, mouse.Y) {
		s.zoomAt(zoomFromSliderX(mouse.X, s.r.Zoom()))
		return true
	}

	if mouse.Buttons&platform.MouseReleaseLeft != 0 {
		switch {
		case inRect(MinimapX2, mouse.X, mouse.Y):
			s.playToggleSfx() // 2X clicks like the square overlay toggles (IAMMO0 = ihits0.wav)
			s.minimap.minimap2x = !s.minimap.minimap2x
			return true
		case inRect(MinimapTnt, mouse.X, mouse.Y):
			s.playToggleSfx() // TNT minimap flic toggle, the same square button click as 2X
			s.minimap.minimapTnt = !s.minimap.minimapTnt
			return true
		case inRect(ZoomPlus, mouse.X, mouse.Y):
			// The zoom buttons were the last in game controls with no click at all. The
			// original gives every control button the same menu blip through
			// Button::PlaySound (MENU_GUI_ITEM_DEF sfx = MENUOP).
			s.playMenuBarSfx()
			s.zoomAt(s.r.Zoom() + 10)

			return true
		case inRect(ZoomMinus, mouse.X, mouse.Y):
			s.playMenuBarSfx()
			s.zoomAt(s.r.Zoom() - 10)

			return true
		}
	}

	return false
}

// minimapOrder sends a selected active mobile unit to a right-clicked minimap cell. It uses orderMoveTo and
// the 2X-aware minimap mapping; clicks inside the minimap are always consumed.
//
// C++ ported : game_manager.cpp - WINDOW_MINIMAP right release sets ORDER_MOVE to
// the minimap cell when GameManager_GetWindowCursor is CURSOR_UNIT_GO / CURSOR_WAY.
func (s *Screen) minimapOrder(mx, my int) bool {
	cellX, cellY, ok := s.minimapCellAt(mx, my)

	if !ok {
		return false // not on the minimap: let the map's right click handle it
	}

	// A selected, active team, MOBILE unit moves to the cell if it can enter it;
	// a structure (or a builder mid build pose) never pathfinds, and the right click
	// is consumed silently (C++: does nothing when the cursor there is not a move cursor).
	if sel := s.selected; canTakeMoveOrder(sel) &&
		sel.Team == s.game.State.ActiveTeam && s.game.CanEnter(sel, cellX, cellY) {
		s.orderMoveTo(sel, cellX, cellY)
	}

	return true
}

// onOff picks the on or off art by a toggle state.
func onOff(on bool, a, b *gfx.Buffer) *gfx.Buffer {
	if on {
		return a
	}

	return b
}

// blitArt copies art into dst at (x, y), clipped, opaque.
func blitArt(dst, art *gfx.Buffer, x, y int) {
	w, h, ok := blitClip(dst, art, x, y)

	if !ok {
		return
	}

	dst.Sub(x, y, w, h).Copy(art.Sub(0, 0, w, h))
}

// blitArtKeyed copies art into dst at (x, y), clipped, skipping its keyed
// transparent colour (index 0; the caller keys the art's corner beforehand).
func blitArtKeyed(dst, art *gfx.Buffer, x, y int) {
	w, h, ok := blitClip(dst, art, x, y)

	if !ok {
		return
	}

	dst.Sub(x, y, w, h).TransCopy(art.Sub(0, 0, w, h))
}

func blitClip(dst, art *gfx.Buffer, x, y int) (w, h int, ok bool) {
	if art == nil {
		return 0, 0, false
	}

	w, h = art.W, art.H

	if x+w > dst.W {
		w = dst.W - x
	}

	if y+h > dst.H {
		h = dst.H - y
	}

	return w, h, w > 0 && h > 0
}

// recenterFromMinimap centres the main view on the cell under a minimap click or
// held drag. 2X freezes the overview origin on the press edge so the drag does not
// drift the view. Does nothing off the minimap.
func (s *Screen) recenterFromMinimap(mx, my int) {
	var cellX, cellY int
	var ok bool

	if s.minimap.minimap2x {
		// Capture the overview origin once, on the press edge, then hold it.
		if !s.minimap.minimapDrag {
			mmW, mmH := viewSize(Minimap)
			s.minimap.minimapDragOX, s.minimap.minimapDragOY = s.minimap2xOrigin(mmW, mmH)
			s.minimap.minimapDrag = true
		}
		cellX, cellY, ok = s.minimapCellAtOrigin(mx, my, s.minimap.minimapDragOX, s.minimap.minimapDragOY)
	} else {
		cellX, cellY, ok = s.minimapCellAt(mx, my)
	}

	if !ok {
		return // not on the minimap
	}

	vw, vh := viewSize(MainMap)
	s.camera.scrollX, s.camera.scrollY = centerScroll(cellX, cellY, s.r.Zoom(), vw, vh)
}

// minimapBlipCells is the blip size in cells: 2 for every unit (as the DOS minimap shows), 1 for a unit that
// is not a building while the minimap is in its 2X view.
func minimapBlipCells(u *game.UnitInfo, zoom2x bool) int {
	if zoom2x && !u.Flags.Has(game.FlagBuilding) {
		return 1
	}

	return 2
}
