// camera.go : implements the map view: zoom, scrolling and the camera bookmarks.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/saveload"
)

// cameraState holds the map viewport, camera lock, bookmarks, and landing or leaving animation state.
type cameraState struct {
	scrollX         int               // horizontal map offset
	scrollY         int               // vertical map offset
	scrollStepPx    int               // per frame edge scroll px, from quick_scroll (0 = default)
	lockUnit        *game.UnitInfo    // unit the view is locked onto (LOCK panel button); C++: GameManager_LockUnit
	landingStep     int               // camera zoom in from orbit: steps left (0 = not landing)
	landingZoom     int               // camera zoom in from orbit: target zoom
	landingStamp    uint64            // camera zoom in from orbit: last step timestamp
	landingSmooth   bool              // ramp the zoom in small linear steps (post load focus) instead of orbit doubling
	leavingStart    uint64            // tick the planet leaving animation began (0 = not leaving)
	cameraBookmarks [4]cameraBookmark // map camera pins: Alt+F5..F8 sets, F5..F8 recalls
}

// zoomAt changes the zoom while keeping the viewport centered on the same
// map cell, so zooming does not jump the view.
func (s *Screen) zoomAt(z int) {
	s.orderInput.popupOpen = false // the popup is anchored in screen space; drop it on zoom
	vw, vh := viewSize(MainMap)
	cellX := (s.camera.scrollX + vw/2) / s.r.Zoom()
	cellY := (s.camera.scrollY + vh/2) / s.r.Zoom()
	s.r.SetZoom(z)
	s.camera.scrollX, s.camera.scrollY = centerScroll(cellX, cellY, s.r.Zoom(), vw, vh)
}

// zoomAtPoint changes the zoom while keeping the map cell under (px, py) fixed
// (scroll wheel zoom); falls back to centered zoom outside the viewport.
func (s *Screen) zoomAtPoint(z, px, py int) {
	s.orderInput.popupOpen = false // drop the screen anchored popup on zoom
	cellX, cellY, ok := s.mapCell(px, py)

	if !ok {
		s.zoomAt(z)
		return
	}

	s.r.SetZoom(z)
	nz := s.r.Zoom()
	s.camera.scrollX = cellX*nz - (px - MainMap.ULX)
	s.camera.scrollY = cellY*nz - (py - MainMap.ULY)
}

// handleViewControls handles camera input: edge and minimap scrolling, wheel zooming, and minimap zoom controls. It remains active during AI turns.
// C++ ported : game_manager.cpp - scroll/zoom stays live during another team's turn.
func (s *Screen) handleViewControls(mouse platform.MouseState, wheelY int) bool {
	step := scrollStep

	if s.camera.scrollStepPx > 0 { // custom edge scroll speed (settings) overrides the default
		step = s.camera.scrollStepPx
	}

	// Click To Scroll only moves at the edge while the button is held. C++: click_scroll.
	if !s.clickScroll || mouse.Buttons&platform.MousePressLeft != 0 {
		dx, dy := edgeScroll(mouse.X, mouse.Y, step)
		s.camera.scrollX += dx
		s.camera.scrollY += dy
	}

	// Scroll wheel zooms toward the cursor; each notch doubles/halves.
	if wheelY > 0 {
		for i := 0; i < wheelY; i++ {
			s.zoomAtPoint(s.r.Zoom()*2, mouse.X, mouse.Y)
		}
	} else if wheelY < 0 {
		for i := 0; i < -wheelY; i++ {
			s.zoomAtPoint(s.r.Zoom()/2, mouse.X, mouse.Y)
		}
	}

	// A left-click or drag on the minimap recentres the view. At 2X, the press freezes the overview origin so a drag cannot drift.
	if mouse.Buttons&platform.MousePressLeft != 0 {
		s.recenterFromMinimap(mouse.X, mouse.Y)
	} else {
		s.minimap.minimapDrag = false // button up: end any drag, drop the frozen origin
	}

	// Minimap 2X / TNT toggles and the zoom +/- buttons and slider.
	return s.handleMinimapZoomControls(mouse)
}

// cameraBookmark stores a cell pinned with Alt+F5 through Alt+F8. F5 through F8 return the view to that cell.
type cameraBookmark struct {
	cellX, cellY int
	set          bool
}

// setCameraBookmark pins the cell at the view centre in slot 0 through 3.
// C++: GameManager_ProcessKey Alt+F5..F8 writing team screen_locations[i].
func (s *Screen) setCameraBookmark(slot int) {
	if slot < 0 || slot >= len(s.camera.cameraBookmarks) {
		return
	}

	cellX, cellY := s.viewCenterCell()
	s.camera.cameraBookmarks[slot] = cameraBookmark{cellX: cellX, cellY: cellY, set: true}
}

// recallCameraBookmark centres the view on the cell pinned in slot 0 through 3. Empty slots do nothing. C++: GameManager_ProcessKey F5..F8 recalling a location.
func (s *Screen) recallCameraBookmark(slot int) {
	if slot < 0 || slot >= len(s.camera.cameraBookmarks) {
		return
	}

	b := s.camera.cameraBookmarks[slot]
	if !b.set {
		return
	}

	vw, vh := viewSize(MainMap)
	s.camera.scrollX, s.camera.scrollY = centerScroll(b.cellX, b.cellY, s.r.Zoom(), vw, vh)
	s.camera.scrollX, s.camera.scrollY = s.r.ClampScroll(s.camera.scrollX, s.camera.scrollY, vw, vh)
}

// captureCameraBookmarks copies assigned camera pins into save snapshots. C++: saveload.cpp writes team screen_locations[4].
func (s *Screen) captureCameraBookmarks() []saveload.CameraBookmarkSnapshot {
	var out []saveload.CameraBookmarkSnapshot

	for i, b := range s.camera.cameraBookmarks {
		if b.set {
			out = append(out, saveload.CameraBookmarkSnapshot{Slot: i, CellX: b.cellX, CellY: b.cellY})
		}
	}

	return out
}

// applyCameraBookmarks replaces the camera pins with those in a save. Missing slots stay empty.
func (s *Screen) applyCameraBookmarks(marks []saveload.CameraBookmarkSnapshot) {
	s.camera.cameraBookmarks = [4]cameraBookmark{}

	for _, m := range marks {
		if m.Slot < 0 || m.Slot >= len(s.camera.cameraBookmarks) {
			continue
		}

		s.camera.cameraBookmarks[m.Slot] = cameraBookmark{cellX: m.CellX, cellY: m.CellY, set: true}
	}
}

// unitInsideMapView reports whether any part of u (2x2 for a building) shows in the main viewport.
//
// C++ ported : game_manager.cpp - GameManager_IsInsideMapView
func (s *Screen) unitInsideMapView(u *game.UnitInfo) bool {
	if s.r == nil {
		return false // no map view (headless): nothing is on screen
	}
	zoom := s.r.Zoom()
	if zoom <= 0 {
		return false
	}

	size := zoom
	if u.Flags.Has(game.FlagBuilding) {
		size = 2 * zoom
	}

	vw, vh := viewSize(MainMap)
	x := u.GridX*zoom - s.camera.scrollX
	y := u.GridY*zoom - s.camera.scrollY
	return x+size > 0 && x < vw && y+size > 0 && y < vh
}

// viewCenterCell returns the map cell at the centre of the main viewport. It is the inverse of centerScroll and is used for camera bookmarks.
func (s *Screen) viewCenterCell() (cellX, cellY int) {
	zoom := s.r.Zoom()
	if zoom <= 0 {
		return 0, 0
	}

	vw, vh := viewSize(MainMap)
	cellX = (s.camera.scrollX + vw/2 - zoom/2) / zoom
	cellY = (s.camera.scrollY + vh/2 - zoom/2) / zoom
	return cellX, cellY
}
