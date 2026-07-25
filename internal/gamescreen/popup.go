// popup.go : implements in-game confirmations, unit orders, and text overlays.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/platform"
)

// orderInputState tracks clicks, long presses, the unit order popup, armed orders, and clicks consumed when a modal closes.
type orderInputState struct {
	leftDown        bool           // a long left press over a friendly unit is being held
	pressStart      uint64         // tick the press started
	pressX          int            // press position X
	pressY          int            // press position Y
	longFired       bool           // the hold already opened a popup menu (swallow the release)
	popupOpen       bool           // unit popup menu is open
	popupUnit       *game.UnitInfo // unit the popup menu is showing orders for
	popupX          int            // popup menu screen X
	popupY          int            // popup menu screen Y
	popupItems      []popupItem    // popup menu order list
	attackArmed     bool           // popup Attack order armed; next left click fires
	forceMod        bool           // SHIFT/CTRL force attack modifier, snapshotted once per frame (idle tick)
	shiftMod        bool           // SHIFT alone: the additive multi select modifier (shift click add/drop)
	infiltrateArmed bool           // commando Disable/Steal popup order armed
	infiltrateSteal bool           // steal (capture) selected over disable
	reloadArmed     bool           // Supply Truck Reload popup order armed
	repairArmed     bool           // mobile Repair unit's Repair popup order armed
	loadPassenger   *game.UnitInfo // Enter boarding pick: next click on an adjacent transport boards it
	loadArmed       bool           // boarding pick armed
	unloadFrom      *game.UnitInfo // Activate placement pick: the carrier the unit leaves
	unloadUnit      *game.UnitInfo // Activate placement pick: the stored unit to place
	xferSource      *game.UnitInfo // giving/receiving unit while the X-Fer pick is armed
	xferArmed       bool           // X-Fer target pick armed
	swallowRelease  bool           // eat the mouse release that closed a modal, so it doesn't leak into the map as an order
	swallowMapClick bool           // drop a modal's dismiss click so it doesn't leak into the map as a move order
}

// longPressMS is how long the left button must be held (without dragging) over
// a friendly unit before the order popup opens.
const longPressMS = 350

// Popup geometry (UNTBTN art is 42x21, stacked with a 3px gap and margin).
// C++: GameManager_InitPopupButtons (game_manager.cpp).
const (
	popupBtnW      = 42
	popupBtnH      = 21
	popupPad       = 3
	popupPitch     = popupBtnH + popupPad // one row's vertical pitch
	popupMenuWidth = popupBtnW + 6        // panel width (image->width + 6)
)

type popupButtonArt struct {
	up, down *gfx.Buffer
}

// popupItem is one order in the unit popup: a caption, the action it runs, and
// whether it is currently active (drawn green, like a toggled Sentry).
type popupItem struct {
	label  string
	action func()
	active bool
	tex    string // vanilla order tag (e.g. "LOAD", "STOP") - documents the order; reserved for the keyboard shortcut key (C++ RegisterButton position)
}

// trackLongPress drives the press timer each idle tick: once the left button has been
// held long enough in place over a friendly unit, it opens that unit's order popup.
// Reports whether the popup machinery consumed this frame's input.
func (s *Screen) trackLongPress(mouse platform.MouseState) bool {
	held := mouse.Buttons&platform.MousePressLeft != 0
	released := mouse.Buttons&platform.MouseReleaseLeft != 0

	// While a popup is open, it owns the mouse: a release on a row runs it, a
	// release elsewhere dismisses it (the opening release is swallowed first).
	if s.orderInput.popupOpen {
		if released {
			if s.orderInput.longFired {
				s.orderInput.longFired = false
				s.orderInput.leftDown = false
				return true
			}

			if i := s.popupRowAt(mouse.X, mouse.Y); i >= 0 {
				s.orderInput.popupItems[i].action()
			}

			s.orderInput.popupOpen = false
		}

		return true
	}

	if held && !s.orderInput.leftDown { // button just went down
		s.orderInput.leftDown = true
		s.orderInput.longFired = false
		s.orderInput.pressStart = platform.Ticks()
		s.orderInput.pressX, s.orderInput.pressY = mouse.X, mouse.Y
	}

	if !held {
		s.orderInput.leftDown = false
		return false
	}

	// Held: open the popup once the hold passes the threshold without dragging.
	if !s.orderInput.longFired && platform.Ticks()-s.orderInput.pressStart >= longPressMS &&
		iabsInt(mouse.X-s.orderInput.pressX) < 4 && iabsInt(mouse.Y-s.orderInput.pressY) < 4 {
		if s.openUnitPopup(mouse.X, mouse.Y) {
			s.orderInput.longFired = true
			return true
		}

		// No friendly unit under the cursor: arm longFired anyway so this hold
		// does not later fire a normal click on release.
		s.orderInput.longFired = true
	}

	return s.orderInput.longFired
}

// popupTargetAt resolves which active team unit a long press on a cell pops the order
// menu for, mirroring clickCell's SELECT preference so a click and a long press agree.
// Preference: a build ready builder parked here wins (target the constructor, not the
// hidden structure under it), then the already selected unit, then a mobile unit, then
// a structure. Ground cover and still hidden / unexpanded units are never targets; an
// empty cell returns nil (caller tries the site aware fallback).
// C++: the long press popup targets the unit a click would select (game_manager.cpp).
func (s *Screen) popupTargetAt(cellX, cellY int) *game.UnitInfo {
	occ := s.game.Units.Cells.At(cellX, cellY)

	// A build ready builder parked here wins, exactly like clickCell, so a long press
	// over a just finished building targets the parked constructor (to drive it off),
	// not the hidden structure under it.
	for _, e := range occ {
		if e.Team == s.game.State.ActiveTeam && s.game.IsBuildReady(e) {
			return e
		}
	}

	var mobile, structure *game.UnitInfo

	for _, e := range occ {
		// Only an active team selectable unit, never terrain or a hidden / still unfolding
		// structure (a parked factory output or an unexpanded building).
		if e.Team != s.game.State.ActiveTeam || !e.Flags.Has(game.FlagSelectable) {
			continue
		}

		if e.Flags.Has(game.FlagGroundCover) || e.HiddenUntilExpand || e.Unfold != 0 {
			continue
		}

		if e == s.selected {
			return e // the press is on the already selected unit: pop its own orders
		}

		if e.Flags.Has(game.FlagStationary) {
			if structure == nil {
				structure = e
			}
		} else if mobile == nil {
			mobile = e
		}
	}

	if mobile != nil {
		return mobile // a mobile unit wins over a structure it shares the cell with
	}

	return structure
}

// openUnitPopup builds and shows the order popup for the active team unit under
// a screen point, returning false when there is no such unit.
func (s *Screen) openUnitPopup(mx, my int) bool {
	cellX, cellY, ok := s.mapCell(mx, my)

	if !ok {
		return false
	}

	s.dbgSelectTrace("popup", cellX, cellY)
	u := s.popupTargetAt(cellX, cellY)

	// Site aware fallback: a clearing bulldozer (or building constructor) is drawn
	// centred on its 2x2 site, off its logical cell, so a long press may land on no unit.
	// Pop whichever active team unit owns a site covering the press (so the player can
	// Stop a clear / cancel a build).
	if u == nil {
		if site := s.siteUnitAt(cellX, cellY); site != nil && site.Team == s.game.State.ActiveTeam {
			u = site
		}
	}

	if u == nil {
		return false
	}

	dbg.Printf("popup -> %s (was selected=%s)", dbgUnitName(u), dbgUnitName(s.selected))
	s.setSelected(u) // a long press on one unit dissolves any active group
	s.orderInput.popupUnit = u
	s.orderInput.popupItems = s.popupItemsFor(u)

	if len(s.orderInput.popupItems) == 0 {
		return false
	}

	// Position the stack to the right of the unit, vertically centred on it;
	// flip to the left when it would run off the right edge, then clamp to the
	// map view. C++: the ulx/uly placement in GameManager_InitPopupButtons.
	z := s.r.Zoom()
	n := 1

	if u.Flags.Has(game.FlagBuilding) {
		n = 2 // 2x2 footprint
	}

	ux := MainMap.ULX + u.GridX*z - s.camera.scrollX
	uy := MainMap.ULY + u.GridY*z - s.camera.scrollY
	h := len(s.orderInput.popupItems)*popupPitch + popupPad
	s.orderInput.popupX = ux + n*z + 2
	s.orderInput.popupY = uy + n*z/2 - h/2

	if s.orderInput.popupX+popupMenuWidth > MainMap.LRX {
		s.orderInput.popupX = ux - popupMenuWidth - 2 // flip to the unit's left
	}

	s.orderInput.popupX = clampInt(s.orderInput.popupX, MainMap.ULX, MainMap.LRX-popupMenuWidth)
	s.orderInput.popupY = clampInt(s.orderInput.popupY, MainMap.ULY, MainMap.LRY-h)
	s.orderInput.popupOpen = true
	return true
}

/* // note: unused code in Go port
// popupNotReady reports that an order needs a sim system not yet ported.
func (s *Screen) popupNotReady() {
	s.showMessage("Not available yet in this build.")
}
*/

// popupRowAt returns the popup item index under a screen point, or -1 (also
// returning -1 for the 3px gaps between buttons).
func (s *Screen) popupRowAt(mx, my int) int {
	if !s.orderInput.popupOpen || mx < s.orderInput.popupX+3 || mx >= s.orderInput.popupX+3+popupBtnW {
		return -1
	}

	off := my - (s.orderInput.popupY + popupPad)

	if off < 0 {
		return -1
	}

	row := off / popupPitch

	if row >= len(s.orderInput.popupItems) || off%popupPitch >= popupBtnH {
		return -1
	}

	return row
}

// SetPopupVanillaLook picks the order popup button style: the retail look (one
// uniform UNTBTN face per row plus a text caption, what the shipped game draws) or
// the bevel art (the per order faces baked into MAX.RES). Reads video.popup_vanilla_look.
func (s *Screen) SetPopupVanillaLook(on bool) {
	s.hudArt.popupBevelArt = !on
}

// colorPopupGold is the resting popup caption colour (tan/gold under the game
// palette), matching the original's FONT2 button captions.
const colorPopupGold = 0xA2

// inFootprint reports whether cell (cx, cy) lies within the footprint x size
// anchored at (ax, ay).
func inFootprint(cx, cy, ax, ay, size int) bool {
	return cx >= ax && cx < ax+size && cy >= ay && cy < ay+size
}

// iabsInt is the int absolute value used for the press drag threshold.
func iabsInt(v int) int {
	if v < 0 {
		return -v
	}

	return v
}
