// panel.go : implements the side panel renderer.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : interface.c - GNW_interface update and button dispatch; unitinfo.cpp - panel state
package gamescreen

import (
	"sort"

	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/platform"
)

// panel.go labels and wires the left panel button bar (the WINDOW_*_BUTTON
// strip): the Center/Lock/Prev/Done/Next control row and the 3x3 display overlay
// toggle grid (Survey/Status/Colors/Hits/Ammo/Range/Scan/Grid/Name). View
// controls (Center, the toggles) stay live during a handover, like the map
// keeps scrolling and zooming; the unit cycle actions lock until it is the
// player's turn again. All buttons are wired: Center, Lock (follow), the unit
// cycle, and the display toggles (Survey/Status/Colors/Hits/Ammo/Range/Scan/
// Grid/Name); the overlay rendering lives in unitdisplays.go and resourceoverlay.go.
//
// C++ ported : window_manager.cpp - the WINDOW_*_BUTTON layout + GameManager handlers

// hudArtState holds HUD button and panel art together with pressed states and caption colours.
type hudArtState struct {
	panelPressed                        gfx.Rect                    // bar button currently held down (zero = none), press visual
	panelOffArt                         map[gfx.Rect]*gfx.Buffer    // control row / toggle grid button rest (_OFF) face, keyed by rect
	panelOnArt                          map[gfx.Rect]*gfx.Buffer    // control row / toggle grid button pressed/active (_ON) face, keyed by rect
	goalOffArt, goalOnArt               *gfx.Buffer                 // GOAL_OFF/GOAL_ON faces the CHAT slot shows when a game goal is set
	endTurnUp                           *gfx.Buffer                 // ENDTRN button face (nil if the resource is absent)
	endTurnDown                         [game.TeamCount]*gfx.Buffer // per team PRESSED face, held while the turn belongs to someone else
	turnCounterBg, turnTimerBg          *gfx.Buffer                 // TURNS/TIMER readout slot backgrounds
	coordsBg, unitNameBg                *gfx.Buffer                 // coords/unit name readout slot backgrounds
	statGauges                          map[string]gaugePair        // AMMO/SPEED/SHOTS/CARGO full+empty gauge icons
	hitsGauge                           [3]gaugePair                // HITS: [0] blue (>1/2), [1] yellow (>1/4), [2] red
	endTurnPressed                      bool                        // End Turn button currently held
	capEnabled, capPressed, capDisabled fonts.Color                 // End Turn caption colours: white local turn, red held, green disabled
	filesArt, prefsArt                  *gfx.Buffer                 // FILES/PREFS button rest face
	filesOnArt, prefsOnArt              *gfx.Buffer                 // FILES/PREFS button pressed face
	filesDown, prefsDown                bool                        // pressed visual held state
	playArt, pauseArt                   *gfx.Buffer                 // corner portrait Play/Pause button rest face
	playOnArt, pauseOnArt               *gfx.Buffer                 // corner portrait Play/Pause button pressed face
	playDown, pauseDown                 bool                        // pressed visual held state
	playFlic                            bool                        // gates whether drawCornerFlic advances frames
	reptArt, helpArt                    *gfx.Buffer                 // REPORTS/HELP button rest face
	helpOnArt                           *gfx.Buffer                 // HELP button pressed face
	helpBtnDown                         bool                        // pressed visual held state
	untbtnUp, untbtnDown                *gfx.Buffer                 // unit/building order popup button rest/pressed art
	popupArt                            map[string]popupButtonArt   // per order popup button art, keyed by tag
	popupBevelArt                       bool                        // draw the bevel faces instead of the retail uniform face (zero value keeps the retail look)
	panelTopArt, panelBtmArt            *gfx.Buffer                 // landing sequence left column panels (logo / Cancel/Help)
	pnlCancelArt, pnlHelpArt            *gfx.Buffer                 // Cancel/Help buttons on the bottom landing panel
	/* // note: unused code in Go port
	// reptOnArt loaded REPT_ON a second time; the pressed REPORTS face comes from panelFaces.
	reptOnArt *gfx.Buffer // REPORTS button pressed face
	*/
}

// panelLabelGreen is the pressed/active label colour (the green ramp the vanilla
// SCAN/AMMO/NEXT labels and the _ON plate border use). C++: SetCaption down color.
const panelLabelGreen = 0x22

// panelButton is one bar button: its rect, the text label drawn over it (empty
// for the icon only Center/Lock), the label x nudge (PREV/NEXT sit on the wide
// part of their arrow), and the toggle state probe (nil = a momentary button).
type panelButton struct {
	rect   gfx.Rect
	label  string
	xoff   int
	active func() bool
}

// panelButtons lists the control row + toggle grid buttons, top to bottom. The
// labels are drawn in gold over the FRAMEPIC bevel; the highlighted (_ON) face
// adds the green border when pressed or active. C++: GameManager_MenuItems +
// GameManager_MenuItemLabelOffsets (PREV x=-10 / NEXT x=+10). SetCaption
// centres the caption in a box of width (button_width - x), so its offset x moves
// the text centre by -x/2: PREV +5 px right, NEXT -5 px left, onto the wide half
// of each pennant arrow (PREV points left, NEXT points right). The port's xoff is
// a direct pixel shift, so it takes +5 / -5 (the old -2 / +2 shifted the labels
// the wrong way, over each arrow's narrow point).
func (s *Screen) panelButtons() []panelButton {
	return []panelButton{
		{CenterButton, "", 0, nil}, // FIND icon (FRAMEPIC)
		{LockButton, "", 0, func() bool { return s.camera.lockUnit != nil }}, // LOCK icon (FRAMEPIC)
		{PrevButton, "PREV", 5, nil},
		{DoneButton, "DONE", 0, nil},
		{NextButton, "NEXT", -5, nil},
		{ReportsButton, "REPORTS", 0, nil},
		{ChatButton, s.chatLabel(), 0, nil}, // GOAL when a game goal is set, else CHAT

		{SurveyButton, "SURVEY", 0, func() bool { return s.surveyOverlay }},
		{StatusButton, "STATUS", 0, func() bool { return s.showStatus }},
		{ColorsButton, "COLORS", 0, func() bool { return s.showColors }},
		{HitsButton, "HITS", 0, func() bool { return s.showHits }},
		{AmmoButton, "AMMO", 0, func() bool { return s.showAmmo }},
		{RangeButton, "RANGE", 0, func() bool { return s.showRange }},
		{ScanButton, "SCAN", 0, func() bool { return s.showScan }},
		{GridButton, "GRID", 0, func() bool { return s.grid }},
		{NameButton, "NAMES", 0, func() bool { return s.showNames }},
	}
}

// playMenuBarSfx plays the small menu button blip every in game control button
// shares: Files, Preferences, Play, Pause, Center, Lock, Prev, Done, Next,
// Reports, Goal/Chat, Help, End Turn. In the original each carries sfx = MENUOP and the
// GNW button plays it on press (Button::PlaySound); MENUOP and MBUTT0 both
// resolve to mengens4.wav, so a single tag serves.
// C++ ported : game_manager.cpp - MENU_GUI_ITEM_DEF(..., MENUOP), button.cpp - Button::PlaySound.
func (s *Screen) playMenuBarSfx() {
	if s.sound.sfxFn != nil {
		s.sound.sfxFn("MBUTT0", "", 0)
	}
}

// playToggleSfx plays the left panel overlay toggle blip (Survey/Status/Colors/
// Hits/Ammo/Range/Scan/Grid/Names). the original tags each button separately (ISURV0,
// ISTAT0, ICOLO0, ...) but they all resolve to the same ihits0.wav, so one tag
// serves. C++ ported : game_manager.cpp - PlaySfx(ISURV0/ISTAT0/... = ihits0.wav).
func (s *Screen) playToggleSfx() {
	if s.sound.sfxFn != nil {
		s.sound.sfxFn("IHITS0", "", 0)
	}
}

// panelHighlighted reports whether a button shows its _ON (green border) face:
// while held down, or (for a toggle) while active.
func (s *Screen) panelHighlighted(b panelButton) bool {
	if b.rect == s.hudArt.panelPressed {
		return true
	}

	return b.active != nil && b.active()
}

// drawPanelButton repaints one bar button: the FRAMEPIC bevel, the _ON face over
// it when highlighted, then the gold label. The caller holds the screen lock.
func (s *Screen) drawPanelButton(buf *gfx.Buffer, b panelButton) {
	r := b.rect

	if s.bg != nil {
		w := min(r.Width()+1, buf.W-r.ULX)
		h := min(r.Height()+1, buf.H-r.ULY)

		if w > 0 && h > 0 {
			buf.Sub(r.ULX, r.ULY, w, h).Copy(s.bg.Sub(r.ULX, r.ULY, w, h))
		}
	}

	// Swap the rest (_OFF) and pressed/active (_ON) faces, exactly like the
	// Files/Prefs buttons swap FILES_OF <-> FILES_ON.
	art := s.hudArt.panelOffArt[r]

	if s.panelHighlighted(b) {
		art = s.hudArt.panelOnArt[r]
	}

	// The CHAT slot shows the GOAL_OFF/GOAL_ON face when a game goal is set
	// (single player), matching its GOAL label. Falls back to CHAT art if missing.
	if r == ChatButton && s.gameGoal != "" && s.hudArt.goalOffArt != nil {
		art = s.hudArt.goalOffArt

		if s.panelHighlighted(b) {
			art = s.hudArt.goalOnArt
		}
	}

	blitButtonArt(buf, art, r)

	if b.label != "" {
		// Gold at rest, green when pressed/active (matching the _ON plate's green
		// border) - the vanilla SCAN/AMMO/NEXT pressed look. C++: SetCaption down
		// FontColor green ramp (~0x22).
		color := colorPopupGold

		if s.panelHighlighted(b) {
			color = panelLabelGreen
		}

		f := s.sys.Fonts().Get(fonts.Font2)
		// Label y offset -1 (one pixel up) reads better on the bevels than centred.
		s.blitStatText(buf, r.ULX+b.xoff, r.ULY-1, r.Width()+1, r.Height()+1, f, b.label, color, true)
	}
}

// drawPanelBar paints the whole control row + toggle grid. The caller holds the
// screen lock (it runs inside repaintBackground).
func (s *Screen) drawPanelBar(buf *gfx.Buffer) {
	for _, b := range s.panelButtons() {
		s.drawPanelButton(buf, b)
	}
}

// updatePanelPress swaps a bar button to its _ON face while the mouse is held
// over it, redrawing on change (like Files/Prefs and the minimap buttons). Run
// every frame.
func (s *Screen) updatePanelPress(mouse platform.MouseState) {
	var over gfx.Rect

	if mouse.Buttons&platform.MousePressLeft != 0 {
		for _, b := range s.panelButtons() {
			if inRect(b.rect, mouse.X, mouse.Y) {
				over = b.rect
				break
			}
		}
	}

	if over == s.hudArt.panelPressed {
		return
	}

	prev := s.hudArt.panelPressed
	s.hudArt.panelPressed = over
	s.redrawPanelRects(prev, over)
}

// redrawPanelRects repaints the named bar buttons (zero rects ignored), e.g. the
// one that lost and the one that gained the pressed face, or a toggled button.
func (s *Screen) redrawPanelRects(rects ...gfx.Rect) {
	if s.sys == nil {
		return // a test built Screen with no live window; nothing to repaint
	}

	buf := s.sys.Buf(bgWindow)
	s.sys.Lock()
	defer s.sys.Unlock()

	for _, b := range s.panelButtons() {
		for _, r := range rects {
			if r != (gfx.Rect{}) && b.rect == r {
				s.drawPanelButton(buf, b)
			}
		}
	}
}

// drawPanelCaptions paints the whole bar (kept name: called from
// repaintBackground, caller holds the lock).
func (s *Screen) drawPanelCaptions(buf *gfx.Buffer) {
	s.drawPanelBar(buf)
}

// handlePanelButtons acts on a left click of a bar button, returning whether it
// consumed the click. View controls work always; the unit cycle actions only
// while it is the player's turn (no handover / game over).
func (s *Screen) handlePanelButtons(mouse platform.MouseState) bool {
	if mouse.Buttons&platform.MouseReleaseLeft == 0 {
		return false
	}

	hit := func(r gfx.Rect) bool { return inRect(r, mouse.X, mouse.Y) }

	if s.handlePanelViewButtons(mouse) {
		return true
	}

	if hit(ChatButton) {
		s.playMenuBarSfx() // GOAL/CHAT clicks like the Reports button
		// In single player the CHAT slot becomes GOAL when a goal is set: a click
		// pops the goal/info dialog (a blocking modal, so the turn clock is paused
		// via openMenuScreen). With no goal it stays the inert networking CHAT stub.
		// C++: GameManager_MenuClickChatGoalButton (the offline branch).
		if s.gameGoal != "" && s.hooks.messageFn != nil {
			s.openMenuScreen(func() { s.hooks.messageFn(s.gameGoal) }, false)
		}

		return true
	}

	// Unit cycle actions: locked during a handover / game over (but the click
	// is still swallowed so it does not fall through to the map).
	action := hit(LockButton) || hit(PrevButton) || hit(DoneButton) || hit(NextButton)

	if !action {
		return false
	}

	// Locked during a handover banner, a computer turn, or game over (the click is
	// still swallowed so it never falls through to the map). The display toggles
	// above stay live in all three, matching vanilla.
	if s.turnFlow.handover != -1 || s.turnFlow.aiTurn || s.gameOver {
		return true
	}

	switch {
	case hit(LockButton):
		s.playMenuBarSfx() // Lock/Prev/Done/Next all click like the menu bar (the MENUOP press sound)
		s.toggleLock()
	case hit(PrevButton):
		s.playMenuBarSfx()
		s.cycleSelection(-1)
	case hit(NextButton):
		s.playMenuBarSfx()
		s.cycleSelection(1)
	case hit(DoneButton):
		s.playMenuBarSfx()
		s.markSelectedDone()
	}

	return true
}

// handlePanelViewButtons acts on the bar's FIND/Center button and display toggles,
// returning whether it consumed the click. These stay live during a handover, a computer
// turn and game over, which is why they answer before the unit cycle actions below them.
func (s *Screen) handlePanelViewButtons(mouse platform.MouseState) bool {
	hit := func(r gfx.Rect) bool { return inRect(r, mouse.X, mouse.Y) }

	switch {
	case hit(CenterButton):
		s.playMenuBarSfx() // FIND/Center clicks like the other control buttons (MENUOP)
		s.centerOnSelected()
		return true
	case hit(GridButton):
		s.grid = !s.grid
		s.redrawPanelRects(GridButton)
		s.playToggleSfx()
		return true
	case hit(NameButton):
		s.showNames = !s.showNames
		s.redrawPanelRects(NameButton)
		s.playToggleSfx()
		return true
	case hit(SurveyButton):
		// Toggle the map wide resource overlay: with it on, the survey markers
		// draw over every cell the player team has surveyed (not only while a
		// surveyor is selected). C++: GameManager_MenuClickSurveyButton.
		s.surveyOverlay = !s.surveyOverlay
		s.redrawPanelRects(SurveyButton)
		s.playToggleSfx()
		return true
	case hit(StatusButton):
		// STATUS: speed/shots/disabled icons over each unit. C++: ...ClickStatusButton.
		s.showStatus = !s.showStatus
		s.redrawPanelRects(StatusButton)
		s.playToggleSfx()
		return true
	case hit(ColorsButton):
		// COLORS: a team colour bracket around mobile units. C++: ...ClickColorsButton.
		s.showColors = !s.showColors
		s.redrawPanelRects(ColorsButton)
		s.playToggleSfx()
		return true
	case hit(HitsButton):
		// HITS: the hit point bar under each unit. C++: ...ClickHitsButton.
		s.showHits = !s.showHits
		s.redrawPanelRects(HitsButton)
		s.playToggleSfx()
		return true
	case hit(AmmoButton):
		// AMMO: the ammo bar under each player unit. C++: ...ClickAmmoButton.
		s.showAmmo = !s.showAmmo
		s.redrawPanelRects(AmmoButton)
		s.playToggleSfx()
		return true
	case hit(RangeButton):
		// RANGE: each player unit's weapon range ring. C++: ...ClickRangeButton.
		s.showRange = !s.showRange
		s.redrawPanelRects(RangeButton)
		s.playToggleSfx()
		return true
	case hit(ScanButton):
		// SCAN: each player unit's scan range ring. C++: ...ClickScanButton.
		s.showScan = !s.showScan
		s.redrawPanelRects(ScanButton)
		s.playToggleSfx()
		return true
	}

	return false
}

// toggleLock turns the lock that follows the selected unit on or off: on, the view
// recenters on that unit every frame (e.g. to watch it drive); a second press,
// or pressing it with no selection, releases it. C++: the LOCK control row toggle.
func (s *Screen) toggleLock() {
	// Already locked (to anyone): release.
	if s.camera.lockUnit != nil {
		s.camera.lockUnit = nil
		s.redrawPanelRects(LockButton)
		return
	}

	// Lock onto the current selection, if any, and snap to it at once.
	if s.selected != nil {
		s.camera.lockUnit = s.selected
		s.centerOnSelected()
	}

	s.redrawPanelRects(LockButton)
}

// followLockedUnit recenters the view on the locked unit each frame while LOCK is
// engaged. It drops the lock if that unit is gone (destroyed/removed), so the
// button does not stay lit on a dead reference.
func (s *Screen) followLockedUnit() {
	if s.camera.lockUnit == nil {
		return
	}

	// Release the lock if the followed unit has left the game (destroyed/removed).
	if s.game.Units.ByID.Get(s.camera.lockUnit.ID) != s.camera.lockUnit {
		s.camera.lockUnit = nil
		s.redrawPanelRects(LockButton)
		return
	}

	// Keep the followed unit centred (it shares centerOnSelected's math).
	vw, vh := viewSize(MainMap)
	s.camera.scrollX, s.camera.scrollY = centerScroll(s.camera.lockUnit.GridX, s.camera.lockUnit.GridY, s.r.Zoom(), vw, vh)
	s.camera.scrollX, s.camera.scrollY = s.r.ClampScroll(s.camera.scrollX, s.camera.scrollY, vw, vh)
}

// centerOnSelected scrolls the map so the selected unit is centred.
func (s *Screen) centerOnSelected() {
	if s.selected == nil {
		return
	}

	vw, vh := viewSize(MainMap)
	s.camera.scrollX, s.camera.scrollY = centerScroll(s.selected.GridX, s.selected.GridY, s.r.Zoom(), vw, vh)
	s.camera.scrollX, s.camera.scrollY = s.r.ClampScroll(s.camera.scrollX, s.camera.scrollY, vw, vh)
}

// centerOnLastSpotted jumps the view to the cell of the last event message - the F1 key: the
// last enemy spotted, or the last of the player's units hit, lost or targeted by a commando
// (C++: GameManager_SpottedEnemyPosition). With nothing known it does nothing.
func (s *Screen) centerOnLastSpotted() {
	cellX, cellY, ok := s.lastSpottedTarget()

	if !ok {
		return
	}

	vw, vh := viewSize(MainMap)
	s.camera.scrollX, s.camera.scrollY = centerScroll(cellX, cellY, s.r.Zoom(), vw, vh)
	s.camera.scrollX, s.camera.scrollY = s.r.ClampScroll(s.camera.scrollX, s.camera.scrollY, vw, vh)
}

// lastSpottedTarget returns the cell F1 should centre on: the cell latchEventCell stored
// at the last event message, or, if none has been stored yet, the first enemy currently
// visible to the player. ok is false when nothing is known at all.
func (s *Screen) lastSpottedTarget() (cellX, cellY int, ok bool) {
	if s.announcer.hasEventCell {
		return s.announcer.lastEventCellX, s.announcer.lastEventCellY, true
	}

	return s.firstVisibleEnemy()
}

// firstVisibleEnemy returns the cell of the first selectable enemy currently visible to
// the player team (the F1 fallback when nothing has been latched yet, e.g. right after a
// load). ok is false when no enemy is visible.
func (s *Screen) firstVisibleEnemy() (cellX, cellY int, ok bool) {
	team := s.game.State.PlayerTeam

	for _, u := range s.game.Units.Units() {
		if u.Team == team || u.Team < 0 || !u.Flags.Has(game.FlagSelectable) {
			continue
		}

		if s.unitVisibleToPlayer(u) {
			return u.GridX, u.GridY, true
		}
	}

	return 0, 0, false
}

// cycleSelection selects the next (dir>0) or previous (dir<0) of the active
// team's selectable units and centres the view on it.
func (s *Screen) cycleSelection(dir int) {
	team := s.game.State.ActiveTeam
	var us []*game.UnitInfo

	for _, u := range s.game.Units.Units() {
		// A unit on guard (Sentry) is held and skipped by the awaiting orders cycle.
		if u.Team == team && u.Flags.Has(game.FlagSelectable) && !u.Flags.Has(game.FlagGroundCover) && u.Order != game.OrderSentry {
			us = append(us, u)
		}
	}

	if len(us) == 0 {
		return
	}

	sort.Slice(us, func(i, j int) bool { return us[i].ID < us[j].ID })
	s.setSelected(cycleNext(us, s.selected, dir)) // cycling to one unit dissolves any group
	s.unitStatusVoice(s.selected)                 // "Ready!" (or the unit's status/order line)
	s.centerOnSelected()
}

// selectNextWithMoves selects the next active team unit (after the current one,
// by id) that still has movement left this turn, centring on it. The Auto-Select
// pref calls it when the selected unit finishes its move with no budget left.
// C++ ported : game_manager.cpp - the auto_select preference advancing the cursor.
func (s *Screen) selectNextWithMoves() {
	team := s.game.State.ActiveTeam
	var us []*game.UnitInfo

	for _, u := range s.game.Units.Units() {
		// Skip a unit on guard (Sentry): the Auto-Select pref must not grab it.
		if u.Team == team && u.Flags.Has(game.FlagSelectable) && !u.Flags.Has(game.FlagGroundCover) && u.Speed > 0 && u.Order != game.OrderSentry {
			us = append(us, u)
		}
	}

	if len(us) == 0 {
		return // no unit left with moves: keep the current selection
	}

	sort.Slice(us, func(i, j int) bool { return us[i].ID < us[j].ID })
	s.setSelected(cycleNext(us, s.selected, 1))
	s.unitStatusVoice(s.selected)
	s.centerOnSelected()
}

// cycleNext returns the unit dir steps from cur in us (wrapping); with cur not
// in us it returns the first (dir>0) or last (dir<0).
func cycleNext(us []*game.UnitInfo, cur *game.UnitInfo, dir int) *game.UnitInfo {
	if len(us) == 0 {
		return cur
	}

	idx := -1

	for i, u := range us {
		if u == cur {
			idx = i
			break
		}
	}

	if idx < 0 { // nothing selected: Next -> first, Prev -> last

		if dir < 0 {
			return us[len(us)-1]
		}

		return us[0]
	}

	idx = ((idx+dir)%len(us) + len(us)) % len(us)
	return us[idx]
}

// markSelectedDone spends the selected unit's turn and advances to the next
// unit (C++: Done selects the next on its own), so the panel keeps showing a unit
// rather than emptying to the bare frame.
func (s *Screen) markSelectedDone() {
	if s.selected == nil {
		return
	}

	s.selected.Speed = 0
	s.game.CancelOrder(s.selected)
	s.cycleSelection(1)
}

// drawNameOverlay labels each on screen unit with its type name when the Name
// toggle is on. It walks units in the same stable painter's order as the sprite
// pass so overlapping labels keep a fixed z order instead of flickering.
func (s *Screen) drawNameOverlay(view *gfx.Buffer) {
	if !s.showNames {
		return
	}

	f := s.sys.Fonts().Get(fonts.Font1)
	z := s.r.Zoom()
	col := fonts.Color{Base: 0xFF, Outline: 0xFF, Shadow: 199}

	for _, u := range s.unitDrawOrder() {
		if u.Flags.Has(game.FlagGroundCover) {
			continue
		}

		name := displayName(u.Type)
		w := f.Width(name)
		tx := u.GridX*z + z/2 - s.camera.scrollX - w/2
		ty := u.GridY*z - s.camera.scrollY - 2

		if tx < 0 || ty < 0 || tx >= view.W || ty >= view.H {
			continue
		}

		f.BlitShadowed(view.Sub(tx, ty, view.W-tx, view.H-ty), name, w+1, col)
	}
}
