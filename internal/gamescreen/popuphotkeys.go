// popuphotkeys.go : runs a unit popup button from the keyboard by the first letter of its caption.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/suglasp/max-go/internal/game"
)

// popupHotkey runs the popup button of the selected unit whose caption starts with the pressed letter (S = Sentry,
// B = Build, ...), whether or not the popup is open. Only plain letters count: digits recall unit groups and the
// Alt / Ctrl combos keep their own meaning. Reports whether a button ran, so the key goes no further.
//
// C++ ported : game_manager.cpp - GameManager_ProcessPopupMenuInput
func (s *Screen) popupHotkey(key int) bool {
	item, ok := s.popupHotkeyItem(key)

	if !ok {
		return false
	}

	if s.sound.sfxFn != nil {
		s.sound.sfxFn("KCARG0", "", 0)
	}

	item.action()

	return true
}

// popupHotkeyItem finds the popup button a key runs for the selected unit: the first one, in screen order, whose
// caption starts with the letter. It reads the open popup when that belongs to the selected unit, otherwise it
// builds the unit's list the way a long press would.
func (s *Screen) popupHotkeyItem(key int) (popupItem, bool) {
	letter, ok := popupHotkeyLetter(key)

	if !ok || !s.popupHotkeysAllowed() {
		return popupItem{}, false
	}

	u := s.selected
	items := s.orderInput.popupItems

	if !s.orderInput.popupOpen || s.orderInput.popupUnit != u {
		items = s.popupItemsFor(u)
	}

	for _, item := range items {
		first, _ := utf8.DecodeRuneInString(strings.TrimSpace(item.label))

		if unicode.ToUpper(first) == letter && item.action != nil {
			return item, true
		}
	}

	return popupItem{}, false
}

// popupHotkeyLetter folds a plain letter key to its upper case; every other key code is not a popup hotkey.
func popupHotkeyLetter(key int) (rune, bool) {
	switch {
	case key >= 'a' && key <= 'z':
		return rune(key - 'a' + 'A'), true
	case key >= 'A' && key <= 'Z':
		return rune(key), true
	}

	return 0, false
}

// popupHotkeysAllowed reports whether the selected unit takes popup hotkeys now: it must be the player's on the
// player's own turn, with no landing, site selection, hand over banner or build placement in the way. A disabled
// unit, a land unit parked build ready on its finished building, and a unit caught between the steps of an order
// take none.
//
// C++ ported : game_manager.cpp - GameManager_ProcessPopupMenuInput (the gates)
func (s *Screen) popupHotkeysAllowed() bool {
	u := s.selected

	if u == nil || s.game == nil || s.game.State == nil {
		return false
	}

	active := s.game.State.ActiveTeam

	if u.Team != active || !s.isHumanTeam(active) || s.turnFlow.aiTurn || s.turnFlow.handover != -1 {
		return false
	}

	if s.siteSelect != nil || s.camera.landingStep > 0 || s.debugMenu.dbgMenuOpen {
		return false
	}

	if s.buildUI.buildMenuOpen || s.buildUI.buildIdx > 0 || s.buildUI.buildPath {
		return false
	}

	if u.Order == game.OrderDisable || (u.Flags.Has(game.FlagMobileLandUnit) && s.game.IsBuildReady(u)) {
		return false
	}

	switch u.Order {
	case game.OrderMove, game.OrderMoveToUnit, game.OrderMoveToAttack, game.OrderPowerOn, game.OrderPowerOff:
		return u.State == game.OrderStateExecutingOrder
	case game.OrderBuild:
		return u.State == game.OrderStateBuildInProgress
	case game.OrderClear:
		// A clear under way keeps the executing state here (sim/rubble.go), so both count as working.
		return u.State == game.OrderStateInProgress || u.State == game.OrderStateExecutingOrder
	}

	return true
}
