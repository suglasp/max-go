// input.go : implements mouse handling and map click routing.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"fmt"

	"github.com/suglasp/max-go/internal/build"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/platform"
)

// swallowDismissClick decides whether handleMouse should drop this frame's map
// click as the trailing dismiss click of a just closed modal. Returns whether to
// drop the click this frame and the updated guard state.
func swallowDismissClick(buttons uint32) (swallow, stillActive bool) {
	if buttons&platform.MousePressLeft != 0 { // still pressing: keep swallowing
		return true, true
	}

	return buttons&platform.MouseReleaseLeft != 0, false // swallow the trailing release, deactivate
}

// idleAnimationsRun reports whether ANIMATED units (radar/scanner/AWAC dish spin,
// radar body cycle) should advance this frame; freezes with "Animate Effects" off.
// Running FACTORIES are never gated on this (their working frame is a state
// indicator, not an effects animation). C++: units_manager.cpp ProcessOrderSentry.
func (s *Screen) idleAnimationsRun() bool {
	return !s.effectsOff
}

// handleMouse routes one frame's mouse state to whatever should act on it. The order
// below is the priority order: a modal's dismiss click is swallowed first, then the HUD
// chrome, then any armed order pick, and only then the map itself.
func (s *Screen) handleMouse() {
	mouse := s.plat.Mouse()

	if s.orderInput.swallowMapClick { // drop a modal's dismiss click before it reaches clickCell
		var swallow bool
		swallow, s.orderInput.swallowMapClick = swallowDismissClick(mouse.Buttons)

		if swallow {
			return
		}
	}

	if s.handleViewControls(mouse, s.plat.ConsumeWheelY()) { // camera input runs first
		return
	}

	// The build popup intercepts clicks while open: a row click selects that type
	// and enters placement mode; a click elsewhere dismisses it.
	if s.buildUI.buildMenuOpen && mouse.Buttons&platform.MouseReleaseLeft != 0 {
		if row := s.buildMenuRowAt(mouse.X, mouse.Y); row >= 0 {
			s.buildUI.buildIdx = row + 1
		}

		s.buildUI.buildMenuOpen = false

		return
	}

	// A long left press over a friendly unit opens its order popup; checked before
	// the click handlers so a hold does not also select/move.
	if !s.buildUI.buildMenuOpen && s.trackLongPress(mouse) {
		return
	}

	// A left drag over the open map rubber bands a multi unit selection box. The
	// whole press/drag/release gesture runs here; a committed drag consumes the
	// release so the click below does not also fire. C++: GameManager_ProcessInput
	// drag box -> Access_MultiSelect.
	if s.trackMarquee(mouse) {
		return
	}

	if s.handleXferPick(mouse) {
		return
	}

	if s.handleAttackPick(mouse) {
		return
	}

	if s.handleInfiltratePick(mouse) {
		return
	}

	if s.handleReloadPick(mouse) {
		return
	}

	if s.handleRepairPick(mouse) {
		return
	}

	if s.handleTransportPick(mouse) {
		return
	}

	if s.handleChromeButtons(mouse) {
		return
	}

	// Swallow the release that closed a modal, so closing a screen does not also
	// order a move / place a building / fire.
	swallow := s.orderInput.swallowRelease

	if mouse.Buttons&(platform.MousePressLeft|platform.MousePressRight) == 0 {
		s.orderInput.swallowRelease = false
	}

	s.handleMapLeftClick(mouse, swallow)

	s.handleMapRightClick(mouse, swallow)
}

// handleMenuBarButtons draws and dispatches top-strip buttons that issue no unit orders: FILES, PREFERENCES, FLIC PLAY/PAUSE, REPORTS, and HELP.
// They remain available during any team turn. openMenuScreen pauses the clock and swallows the closing click. Returns true when a release fires an action.
func (s *Screen) handleMenuBarButtons(mouse platform.MouseState) bool {
	left := mouse.Buttons&platform.MousePressLeft != 0
	s.setMenuButtonDown(FilesButton, s.hudArt.filesArt, s.hudArt.filesOnArt, &s.hudArt.filesDown, left && inRect(FilesButton, mouse.X, mouse.Y))
	s.setMenuButtonDown(PrefsButton, s.hudArt.prefsArt, s.hudArt.prefsOnArt, &s.hudArt.prefsDown, left && inRect(PrefsButton, mouse.X, mouse.Y))

	if mouse.Buttons&platform.MouseReleaseLeft != 0 {
		if inRect(FilesButton, mouse.X, mouse.Y) {
			s.playMenuBarSfx()
			s.openMenuScreen(s.openSaveMenu, true)
			return true
		}

		if inRect(PrefsButton, mouse.X, mouse.Y) {
			s.playMenuBarSfx()
			s.openMenuScreen(s.openOptions, false)
			return true
		}
	}

	flicReady := s.selected != nil // corner FLIC Play/Pause needs a selected unit's portrait
	s.setMenuButtonDown(PlayButton, s.hudArt.playArt, s.hudArt.playOnArt, &s.hudArt.playDown, flicReady && left && inRect(PlayButton, mouse.X, mouse.Y))
	s.setMenuButtonDown(PauseButton, s.hudArt.pauseArt, s.hudArt.pauseOnArt, &s.hudArt.pauseDown, flicReady && left && inRect(PauseButton, mouse.X, mouse.Y))

	if flicReady && mouse.Buttons&platform.MouseReleaseLeft != 0 {
		if inRect(PlayButton, mouse.X, mouse.Y) {
			s.playMenuBarSfx() // the Play button clicks like a menu button; the flic itself stays silent
			s.hudArt.playFlic = true
			return true
		}

		if inRect(PauseButton, mouse.X, mouse.Y) {
			s.playMenuBarSfx() // same for Pause: the button blips, flic playback has no sound
			s.hudArt.playFlic = false
			return true
		}
	}

	// REPORTS is handled by updatePanelPress instead (its REPT_OFF art is a bare bevel).
	s.setMenuButtonDown(HelpButton, s.hudArt.helpArt, s.hudArt.helpOnArt, &s.hudArt.helpBtnDown, left && inRect(HelpButton, mouse.X, mouse.Y))
	if mouse.Buttons&platform.MouseReleaseLeft != 0 {
		if inRect(ReportsButton, mouse.X, mouse.Y) {
			s.playMenuBarSfx()
			s.openMenuScreen(s.openReports, true)
			return true
		}

		if inRect(HelpButton, mouse.X, mouse.Y) {
			s.playMenuBarSfx()
			// Route through openMenuScreen (like the ? key and the menu help) so
			// the closing OK release is swallowed. A bare openHelp let that click
			// bleed into the map and drove the selected unit to the OK button spot.
			s.openMenuScreen(func() { s.openHelp("GAME_SCREEN_SETUP") }, false)
			return true
		}
	}

	return false
}

// handleSpectatorInput runs during a computer turn. View toggles, Center, Reports, left-click inspection, and right-click Status remain available, while orders, placement, cycling, and End Turn stay locked.
// The active unit remains selectable for inspection while another team moves. C++: GameManager keeps overlay, report, and info controls available while order input is suppressed.
func (s *Screen) handleSpectatorInput(mouse platform.MouseState) {
	s.updatePanelPress(mouse)

	if s.handlePanelButtons(mouse) {
		return
	}

	// FILES/PREFS/PLAY/PAUSE/REPORTS/HELP stay usable during the opponent's turn:
	// none issue a unit order, and each modal pauses the turn clock (item 493).
	if s.handleMenuBarButtons(mouse) {
		return
	}

	// Left click selects a unit for the info panel WITHOUT issuing any order.
	if mouse.Buttons&platform.MouseReleaseLeft != 0 {
		if cellX, cellY, ok := s.mapCell(mouse.X, mouse.Y); ok {
			s.selectForInspect(cellX, cellY)
		}

		return
	}

	// Right click opens Status for inspection only; never orders a move or deselects.
	if mouse.Buttons&platform.MouseReleaseRight != 0 {
		if cellX, cellY, ok := s.mapCell(mouse.X, mouse.Y); ok {
			if u := s.topUnitAt(cellX, cellY); u != nil {
				s.openStatus(u)
			}
		}
	}
}

// selectForInspect selects the top unit at a cell for the info panel without
// issuing any order - the left click inspection allowed during the opponent's
// turn. An empty cell keeps the current selection (it is not cleared), so the
// unit you were working with survives the turn. C++: the active unit stays
// selected across turn advance; only order input is gated while another team moves.
func (s *Screen) selectForInspect(cellX, cellY int) {
	if u := s.topUnitAt(cellX, cellY); u != nil {
		s.setSelected(u)
		s.orderInput.popupOpen = false
	}
}

// rightClickMap opens the unit's Status Screen, or deselects on empty terrain
// (a deliberate deviation: vanilla does nothing on empty ground). C++: UnitStats_Menu.
func (s *Screen) rightClickMap(cellX, cellY int) {
	if u := s.topUnitAt(cellX, cellY); u != nil {
		s.openStatus(u)

		return
	}

	s.clearSelected() // empty cell: deselect
	s.orderInput.popupOpen = false
}

// mapCell converts a screen pixel inside the MainMap viewport to a map cell,
// returning ok=false when the pixel is outside the viewport or the map.
func (s *Screen) mapCell(px, py int) (int, int, bool) {
	if px < MainMap.ULX || px > MainMap.LRX || py < MainMap.ULY || py > MainMap.LRY {
		return 0, 0, false
	}

	z := s.r.Zoom()
	cellX := (s.camera.scrollX + px - MainMap.ULX) / z
	cellY := (s.camera.scrollY + py - MainMap.ULY) / z

	if cellX < 0 || cellY < 0 || cellX >= s.wmap.W || cellY >= s.wmap.H {
		return 0, 0, false
	}

	return cellX, cellY, true
}

// dbgSelectTrace logs (-debug) every unit hashed on a clicked cell with the flags
// selection/popup resolution keys on. tag marks the call site. C++: diagnostic only.
func (s *Screen) dbgSelectTrace(tag string, cellX, cellY int) {
	if !dbg.Enabled() {
		return
	}

	occ := s.game.Units.Cells.At(cellX, cellY)

	dbg.Printf("sel-trace %s cell(%d,%d): %d occupant(s); selected=%s", tag, cellX, cellY, len(occ), dbgUnitName(s.selected))

	for _, e := range occ {
		dbg.Printf("  - %s id=%d team=%d mobile=%v building(pose)=%v stationary=%v buildReady=%v hidden=%v unfold=%d isSelected=%v", displayName(e.Type), e.ID, e.Team, isMobileUnit(e), e.Building, e.Flags.Has(game.FlagStationary), s.game.IsBuildReady(e), e.HiddenUntilExpand, e.Unfold, e == s.selected)
	}
}

// dbgUnitName is a nil safe unit label for traces.
func dbgUnitName(u *game.UnitInfo) string {
	if u == nil {
		return "<none>"
	}

	return fmt.Sprintf("%s#%d", displayName(u.Type), u.ID)
}

// clickCell selects a unit in the clicked cell, or moves the selected unit
// to it. C++: GameManager_ProcessTilePress (the click dispatch).
func (s *Screen) clickCell(cellX, cellY int) {
	s.dbgSelectTrace("click", cellX, cellY)

	// A build ready builder drives off when the player clicks one of its BLDMRK exit
	// markers; off the markers the click is swallowed. C++: exits UNIT_READY.
	if s.selected != nil && s.game.IsBuildReady(s.selected) {
		if s.tryBuildExit(cellX, cellY) {
			return
		}
	}

	// SHIFT click on a friendly mobile unit toggles it in the multi selection (add
	// if new, drop if already in). CTRL is left for force attack (fire on a friendly),
	// and shift click on an enemy still force attacks below. C++: GameManager_
	// UpdateSelection shift add/remove.
	if s.shiftHeld() {
		if u := s.friendlySelectableAt(cellX, cellY); u != nil {
			s.toggleSelection(u)

			return
		}
	}

	if s.clickCombat(cellX, cellY) {
		return
	}

	if s.resolveOccupantSelect(cellX, cellY) {
		return
	}

	// A unit drawn CENTRED on a multi cell work site sits off its hashed cell, so a
	// click on a site cell with no unit should still select it.
	if site := s.siteUnitAt(cellX, cellY); site != nil {
		s.selectUnit(site)

		return
	}

	if canTakeMoveOrder(s.selected) {
		if len(s.selectionSet()) > 1 {
			s.orderGroupMoveTo(cellX, cellY) // move the whole group in formation
		} else {
			s.orderMoveTo(s.selected, cellX, cellY)
		}
	}
}

// forceModifierHeld reports whether SHIFT/CTRL force attack is down this frame,
// reading the once per frame snapshot so headless tests can set it directly.
func (s *Screen) forceModifierHeld() bool {
	return s.orderInput.forceMod
}

// shiftHeld reports whether SHIFT alone is down this frame (the additive multi
// select modifier), reading the once per frame snapshot so tests can set it directly.
func (s *Screen) shiftHeld() bool {
	return s.orderInput.shiftMod
}

// handleGameKey routes one key press through the modal gates and then the hotkey
// table. The gates run in order and each one swallows the key outright: the attract
// demo, the leaving animation, a hot seat handover banner, site selection, the landing
// descent, the F2 debug menu and an armed cheat code. The configurable keybind actions
// resolve next, so a remapped key always wins over the fixed hotkeys below them.
// Every path here ends this frame's input handling, which is why the caller continues
// its loop straight afterwards.
func (s *Screen) handleGameKey(key int, buf *gfx.Buffer) {
	// Attract demo: any key returns to the main menu, so normal handlers stay inactive during playback.
	if s.demoMode {
		s.exitToMenu = true

		return
	}

	if s.camera.leavingStart != 0 {
		return // input locked while the leaving animation plays
	}

	// A hot seat handover banner is up: any key acknowledges it and
	// hands control to the team that just started its turn.
	if s.turnFlow.handover != -1 {
		s.turnFlow.handover = -1
		s.turnFlow.turnStart = platform.Ticks()
		s.draw(buf)
		return
	}

	// Site selection: RETURN confirms the landing site, the arrows pan, and
	// ESC / Cancel returns to the previous (cargo) menu - no "exit game?"
	// prompt, since nothing has started yet. C++: GameManager_SelectSite.
	if s.siteSelect != nil {
		if s.siteSelect.introStart != 0 {
			return // input locked while the landing animation plays
		}
		switch key {
		case platform.KeyReturn:
			s.confirmSite()

		case platform.KeyEscape:
			s.cancelLanding() // exitToMenu -> the run loop leaves next iteration

		case platform.KeyLeft:
			s.camera.scrollX -= s.r.Zoom() * 2

		case platform.KeyRight:
			s.camera.scrollX += s.r.Zoom() * 2

		case platform.KeyUp:
			s.camera.scrollY -= s.r.Zoom() * 2

		case platform.KeyDown:
			s.camera.scrollY += s.r.Zoom() * 2
		}

		return
	}

	if s.camera.landingStep > 0 {
		return // input is locked while the camera lands and the base unfolds
	}

	// The F2 debug flag menu grabs all keys while it is open.
	if s.debugMenu.dbgMenuOpen {
		s.handleDebugMenuKey(key)

		return
	}

	// A typed cheat code is armed: the buffer eats its keys (letters, Backspace,
	// Enter, Esc) before any hotkey below fires. C++: GameManager_ProcessKey routes
	// keys into the text handler while GameManager_TextInput is non empty.
	if s.cheatInput != "" && s.cheatKeyPressed(key) {
		return
	}

	// The configurable actions (max-go.json keybinds) resolve first, so a
	// remapped key wins over the fixed hotkeys below. Defaults reproduce
	// the vanilla keys, so an untouched settings file changes nothing.
	if action, ok := s.keybindAction(key); ok {
		s.runKeybindAction(action, buf)

		return
	}

	s.handleHotkey(key, buf)
}

// handleHotkey runs the fixed hotkey table, the keys no keybind setting can remap:
// the debug menu, the Alt shortcuts for the file screens and the exit prompt, arrow
// panning, the PageUp/PageDown zoom aliases, F1 and the F5..F8 camera bookmarks, the
// Alt+N / N unit groups, the debug overlays and the cheat code prompt.
func (s *Screen) handleHotkey(key int, buf *gfx.Buffer) {
	switch key {
	case platform.KeyF2:
		if s.debug {
			s.openDebugMenu()
		}

	case platform.AltMod | 's':
		s.openMenuScreen(s.openSaveMenu, true) // C++: Alt+S GameManager_SaveLoadGame(true)

	case platform.AltMod | 'l':
		s.openMenuScreen(s.openLoadMenu, true) // C++: Alt+L GameManager_SaveLoadGame(false)

	case platform.AltMod | 'f':
		s.openMenuScreen(s.openSaveMenu, true) // C++: Alt+F GameManager_MenuClickFileButton (the FILES button, save mode)

	case platform.AltMod | 'p':
		s.pauseDialog() // C++: Alt+P PauseMenu_Menu

	case platform.AltMod | 'x':
		s.confirmExitToMain(buf) // C++: Alt+X OKCancelMenu "OK to exit game?" -> main menu

	case platform.AltMod | 'd':
		if s.debug {
			s.debugSelfDestructSelected()
		}

	case platform.KeyLeft:
		s.camera.scrollX -= s.r.Zoom() * 2

	case platform.KeyRight:
		s.camera.scrollX += s.r.Zoom() * 2

	case platform.KeyUp:
		s.camera.scrollY -= s.r.Zoom() * 2

	case platform.KeyDown:
		s.camera.scrollY += s.r.Zoom() * 2

	// PageUp/PageDown and the shifted '+' stay fixed zoom aliases; the
	// primary '='/'-' zoom keys live in the keybinds map above.
	case platform.KeyPageUp, '+':
		s.zoomAt(s.r.Zoom() * 2)

	case platform.KeyPageDown:
		s.zoomAt(s.r.Zoom() / 2)

	case platform.KeyF1:
		s.centerOnLastSpotted() // C++: F1 GameManager_SpottedEnemyPosition (centre on last spotted enemy)

	case platform.KeyF5, platform.KeyF6, platform.KeyF7, platform.KeyF8:
		s.recallCameraBookmark(key - platform.KeyF5) // F5..F8: recall a camera bookmark

	case platform.AltMod | platform.KeyF5, platform.AltMod | platform.KeyF6,
		platform.AltMod | platform.KeyF7, platform.AltMod | platform.KeyF8:
		s.setCameraBookmark(key - (platform.AltMod | platform.KeyF5)) // Alt+F5..F8: set a camera bookmark

	case platform.AltMod | '1', platform.AltMod | '2', platform.AltMod | '3',
		platform.AltMod | '4', platform.AltMod | '5', platform.AltMod | '6',
		platform.AltMod | '7', platform.AltMod | '8', platform.AltMod | '9':
		s.setUnitGroup(key - (platform.AltMod | '1')) // Alt+N: assign the selection to group N

	case '1', '2', '3', '4', '5', '6', '7', '8', '9':
		s.recallUnitGroup(int(key - '1')) // N: recall (select whole group + centre)

	case 'a', 'A':
		if s.debug {
			s.accessOverlay = !s.accessOverlay
		}

	case 'v', 'V':
		if s.debug {
			s.fogShow = !s.fogShow // debug: toggle fog of war (was F before F became Find)
		}

	case 'm', 'M':
		s.maxSurvey = !s.maxSurvey // debug: reveal every cell's resources

	case '[':
		s.armCheatInput() // start typing a [MAX...] cheat code (cheats.go)
	}
}

// handleChromeButtons runs the HUD widgets that take a click before the map ever sees
// it: the left panel bar, the top menu bar strip and the End Turn button. The panel bar
// is checked before the hand over gate so view controls stay live while another team
// plays; the cycle actions lock themselves inside their own handler. Returns whether a
// release fired an action.
func (s *Screen) handleChromeButtons(mouse platform.MouseState) bool {
	s.updatePanelPress(mouse)

	if s.handlePanelButtons(mouse) {
		return true
	}

	// FILES/PREFS/PLAY/PAUSE/REPORTS/HELP: the menu bar buttons that issue no unit
	// order. Shared with the opponent turn (item 493) via handleMenuBarButtons.
	if s.turnFlow.handover == -1 && !s.gameOver {
		if s.handleMenuBarButtons(mouse) {
			return true
		}
	}

	// End Turn button: mouse equivalent of RETURN; disabled during a handover/game over.
	overEnd := inRect(EndTurnButton, mouse.X, mouse.Y) && s.turnFlow.handover == -1 && !s.gameOver
	s.hudArt.endTurnPressed = overEnd && mouse.Buttons&platform.MousePressLeft != 0

	if overEnd && mouse.Buttons&platform.MouseReleaseLeft != 0 {
		s.playMenuBarSfx() // Plays MBUTT0 before the turn voice.
		s.endTurn()
		return true
	}

	return false
}

// handleMapLeftClick routes a left click that reached the map: an armed path build takes
// its destination, an armed debug placement drops its unit, an armed build places at the
// clicked quadrant of the builder's footprint, and everything else falls through to
// clickCell for select or move. swallow is the release that closed a modal.
func (s *Screen) handleMapLeftClick(mouse platform.MouseState, swallow bool) {
	if mouse.Buttons&platform.MouseReleaseLeft != 0 && s.turnFlow.handover == -1 && !s.gameOver && !swallow {
		if s.buildUI.buildPath { // Path build armed: this click is the destination
			if cellX, cellY, ok := s.mapCell(mouse.X, mouse.Y); ok {
				s.confirmBuildPath(cellX, cellY)
			}
		} else if s.debugPlaceArmed() {
			if cellX, cellY, ok := s.mapCell(mouse.X, mouse.Y); ok {
				s.placeDebugAt(cellX, cellY)
			}
		} else if s.buildUI.buildIdx > 0 {
			if d, ok := s.buildDesc(); ok {
				n := build.Footprint(d.Flags)
				if b := s.selectedBuilder(); b != nil {
					// Footprint anchored on the builder; the click picks the quadrant.
					if _, _, in := s.mapCell(mouse.X, mouse.Y); in {
						tx, ty := s.buildSiteCell(b, n, mouse.X, mouse.Y)
						s.placeBuildingAt(tx, ty)
					}
				} else if cellX, cellY, in := s.buildCell(mouse.X, mouse.Y, n); in {
					s.placeBuildingAt(cellX, cellY) // demo/debug free placement
				}
			}

		} else if cellX, cellY, ok := s.mapCell(mouse.X, mouse.Y); ok {
			s.clickCell(cellX, cellY)
		}
	}
}

// handleMapRightClick routes a right click that reached the map: it cancels an armed
// build or path pick, otherwise it opens the clicked unit's Status Screen. swallow is
// the release that closed a modal. C++: UnitStats_Menu.
func (s *Screen) handleMapRightClick(mouse platform.MouseState, swallow bool) {
	if mouse.Buttons&platform.MouseReleaseRight != 0 && s.turnFlow.handover == -1 && !s.gameOver && !swallow {
		if s.buildUI.buildPath || s.buildUI.buildIdx > 0 {
			s.cancelBuildArm() // right click cancels an armed build / path pick
		} else if !s.minimapOrder(mouse.X, mouse.Y) {
			if cellX, cellY, ok := s.mapCell(mouse.X, mouse.Y); ok {
				s.rightClickMap(cellX, cellY)
			}
		}
	}
}
