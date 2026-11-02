// keybinds.go : maps configurable actions to GNW input codes.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"strings"

	"github.com/suglasp/max-go/internal/cursors"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/platform"
)

// bindAction names a remappable game action from the settings keybinds map.
// The original has no key rebinding; this layer adds it while preserving the
// original keys by default.
type bindAction int

const (
	bindEndTurn bindAction = iota
	bindMenu
	bindGrid
	bindSurvey
	bindStatus
	bindFind
	bindHelp
	bindOptions
	bindZoomIn
	bindZoomOut
	bindActionCount
)

// bindNames lists the settings map key per action, in dispatch priority
// order: when two actions share a key, the earlier one wins.
var bindNames = [bindActionCount]string{
	bindEndTurn: "end_turn",
	bindMenu:    "menu",
	bindGrid:    "grid",
	bindSurvey:  "survey",
	bindStatus:  "status",
	bindFind:    "find",
	bindHelp:    "help",
	bindOptions: "options",
	bindZoomIn:  "zoom_in",
	bindZoomOut: "zoom_out",
}

// defaultKeybinds holds the default key for each action when the settings map
// is absent, misses an action, or contains an unresolvable value. The survey
// and status overlays and the options screen have no original key, so their
// port picks use Ctrl and leave plain letters for unit popup hotkeys.
var defaultKeybinds = [bindActionCount]int{
	bindEndTurn: platform.KeyReturn,
	bindMenu:    platform.KeyEscape,
	bindGrid:    'g',
	bindSurvey:  platform.CtrlMod | 'r',
	bindStatus:  platform.CtrlMod | 's',
	bindFind:    'f',
	bindHelp:    '?',
	bindOptions: platform.CtrlMod | 'o',
	bindZoomIn:  '=',
	bindZoomOut: '-',
}

// bindPortPick marks actions that the port adds without an original key. They
// resolve after unit popup hotkeys, so a popup letter wins even after rebinding.
var bindPortPick = [bindActionCount]bool{
	bindSurvey:  true,
	bindStatus:  true,
	bindOptions: true,
}

// SetKeybinds resolves the settings keybinds map onto the screen's action keys.
// A missing action or an unresolvable value keeps the default and logs a warning;
// an empty value unbinds the action. A shared key remains on both actions, and
// the earlier action wins at dispatch with a warning.
func (s *Screen) SetKeybinds(binds map[string]string) {
	s.keybinds = defaultKeybinds

	for action := bindAction(0); action < bindActionCount; action++ {
		value, present := binds[bindNames[action]]

		if !present {
			continue
		}

		if strings.TrimSpace(value) == "" {
			s.keybinds[action] = 0 // explicitly unbound

			continue
		}

		code, ok := platform.KeyFromName(value)

		if !ok {
			dbg.Printf("keybinds: %s = %q is not a known key, keeping the default", bindNames[action], value)

			continue
		}

		s.keybinds[action] = code
	}

	for action := bindAction(1); action < bindActionCount; action++ {
		for earlier := bindAction(0); earlier < action; earlier++ {
			if s.keybinds[action] != 0 && s.keybinds[action] == s.keybinds[earlier] {
				dbg.Printf("keybinds: %s and %s share a key; %s wins", bindNames[earlier], bindNames[action], bindNames[earlier])
			}
		}
	}
}

// actionKeys returns the active key table. A zero array means the screen lacks
// New/SetKeybinds setup, as in tests and benches, so the defaults apply.
func (s *Screen) actionKeys() *[bindActionCount]int {
	if s.keybinds == ([bindActionCount]int{}) {
		return &defaultKeybinds
	}

	return &s.keybinds
}

// keybindAction matches an input code against the configured action keys. A
// A shifted letter arrives uppercased, so letters fold to lowercase first. Alt
// and Ctrl combo codes carry modifier bits and never match a plain binding.
// portPick selects either the original actions or the port additions that
// resolve after popup hotkeys.
func (s *Screen) keybindAction(key int, portPick bool) (bindAction, bool) {
	if key >= 'A' && key <= 'Z' {
		key += 'a' - 'A'
	}

	keys := s.actionKeys()

	for action := bindAction(0); action < bindActionCount; action++ {
		if bindPortPick[action] != portPick {
			continue
		}

		if keys[action] != 0 && keys[action] == key {
			return action, true
		}
	}

	return 0, false
}

// runKeybindAction performs one remapped action. The overlay toggles run the
// same code as their panel buttons, so key and click behave identically.
func (s *Screen) runKeybindAction(action bindAction, buf *gfx.Buffer) {
	switch action {
	case bindEndTurn:
		s.endTurn()

	case bindMenu:
		s.menuKeyPressed(buf)

	case bindGrid:
		s.grid = !s.grid // C++: G GameManager_MenuClickGridButton
		s.redrawPanelRects(GridButton)
		s.playToggleSfx()

	case bindSurvey:
		s.surveyOverlay = !s.surveyOverlay // C++: GameManager_MenuClickSurveyButton
		s.redrawPanelRects(SurveyButton)
		s.playToggleSfx()

	case bindStatus:
		s.showStatus = !s.showStatus // C++: GameManager_MenuClickStatusButton
		s.redrawPanelRects(StatusButton)
		s.playToggleSfx()

	case bindFind:
		s.centerOnSelected() // C++: F GameManager_MenuClickFindButton (centre on selected)

	case bindHelp:
		s.openMenuScreen(func() { s.openHelp("GAME_SCREEN_SETUP") }, false) // C++: ? GameManager_MenuClickHelpButton

	case bindOptions:
		s.openMenuScreen(s.openOptions, false)

	case bindZoomIn:
		s.zoomAt(s.r.Zoom() * 2)

	case bindZoomOut:
		s.zoomAt(s.r.Zoom() / 2)
	}
}

// menuKeyPressed handles the menu key (Escape by default): it backs out of an
// armed X-Fer / transport pick / open popup / build menu / placement first,
// and only when nothing is open runs the vanilla "OK to exit game?" confirm.
// C++ ported : game_manager.cpp - GameManager_ProcessKey, the ESCAPE case.
func (s *Screen) menuKeyPressed(buf *gfx.Buffer) {
	if s.debugPlaceArmed() {
		s.cancelDebugPlace()
		return
	}

	if s.orderInput.attackArmed {
		s.cancelAttack()

		return
	}

	if s.orderInput.infiltrateArmed {
		s.cancelInfiltrate()
		return
	}

	if s.orderInput.reloadArmed {
		s.cancelReload()
		return
	}

	if s.orderInput.repairArmed {
		s.cancelRepair()
		return
	}

	if s.transportArmed() {
		s.cancelTransport()
		s.showMessage("Cancelled.")
		return
	}

	if s.orderInput.xferArmed {
		s.cancelTransfer()
		s.showMessage("Transfer cancelled.")
		return
	}

	if s.orderInput.popupOpen || s.buildUI.buildMenuOpen || s.buildUI.buildIdx > 0 || s.buildUI.buildPath {
		s.cancelBuildArm()
		return
	}

	// Otherwise confirm leaving the game (vanilla "OK to exit game?").
	s.confirmExitToMain(buf)
}

// confirmExitToMain runs the vanilla "OK to exit game?" prompt and, on OK, starts
// the leaving animation. Reached by the menu key (Escape) with nothing open, and
// straight away by Alt+X, which skips every backout above.
// C++ ported : game_manager.cpp - GameManager_ProcessKey, GNW_KB_KEY_LALT_X (OKCancelMenu_Menu -> GAME_STATE_3_MAIN_MENU)
func (s *Screen) confirmExitToMain(buf *gfx.Buffer) {
	s.installCursor(cursors.Hand) // the confirm dialog runs its own loop; show the hand, not a context cursor

	// The dialog spins its own loop with real time passing: pause the turn
	// clock across it, else a long deliberation drains the budget and the turn
	// ends automatically (sim mode: the round commits + units move) the instant Cancel
	// is pressed.
	var leaving bool
	s.withTurnClockPaused(func() {
		leaving = s.hooks.confirmFn == nil || s.hooks.confirmFn(s.lang.Get("d2f1", "OK to exit game?"))
	})

	// The dialog's dismiss click (Cancel or OK) must not leak into the map
	// as a "go here" move order on the selected unit.
	s.orderInput.swallowMapClick = true
	if leaving {
		s.startLeaving() // play the reverse landing animation, then leave
		return
	}

	s.draw(buf) // refresh after drawing the separate confirm dialog
}

// pauseDialog pops the vanilla pause box: a modal with a single OK that holds the
// game until it is dismissed. openMenuScreen stops the turn clock while it is up,
// so the pause is real and not just cosmetic. Nothing happens without a message
// hook (headless tests, benches).
// C++ ported : menu.cpp - PauseMenu_Menu (DialogMenu_Menu with the "Game Paused." text)
func (s *Screen) pauseDialog() {
	if s.hooks.messageFn == nil {
		return
	}

	text := s.pauseMessage()
	s.openMenuScreen(func() { s.hooks.messageFn(text) }, false)
}

// pauseMessage is the pause box text, from the retail strings with the vanilla
// wording as the fallback.
func (s *Screen) pauseMessage() string {
	return s.lang.Get("f9bb", "Game Paused.\nClick OK to continue.")
}
