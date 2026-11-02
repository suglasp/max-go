// popuphotkeys_test.go : tests the popup hotkeys (popuphotkeys.go).
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/platform"
)

// hotkeyScreen builds a headless screen with one awaiting player Tank selected, outside any hand over banner.
func hotkeyScreen(t *testing.T) (*Screen, *game.UnitInfo) {
	t.Helper()
	s, g := newClickScreen(t)
	s.turnFlow.handover = -1

	tank := mkUnit(g, assets.Tank, 1, 4, 4, game.FlagMobileLandUnit|game.FlagSelectable)
	tank.Order = game.OrderAwait
	tank.State = game.OrderStateExecutingOrder
	s.selected = tank

	return s, tank
}

// TestPopupHotkeySTogglesSentry: S puts an awaiting Tank on Sentry and a second S takes it off, in either case.
func TestPopupHotkeySTogglesSentry(t *testing.T) {
	s, tank := hotkeyScreen(t)

	if !s.popupHotkey('s') || tank.Order != game.OrderSentry {
		t.Fatalf("S: order = %v, want OrderSentry", tank.Order)
	}

	if !s.popupHotkey('S') || tank.Order == game.OrderSentry {
		t.Errorf("second S: order = %v, want the guard taken off", tank.Order)
	}
}

// TestPopupHotkeyBFindsBuild: B on an idle Constructor finds its Build button (running it opens the menu).
func TestPopupHotkeyBFindsBuild(t *testing.T) {
	s, g := newClickScreen(t)
	s.turnFlow.handover = -1

	c := mkUnit(g, assets.Constrct, 1, 4, 4, game.FlagMobileLandUnit|game.FlagConstructorUnit|game.FlagSelectable)
	c.Order = game.OrderAwait
	c.State = game.OrderStateExecutingOrder
	s.selected = c

	item, ok := s.popupHotkeyItem('b')

	if !ok || item.tex != "BUILD" {
		t.Errorf("B on an idle Constructor = %q/%v, want the BUILD button", item.tex, ok)
	}
}

// TestPopupHotkeyMisses: a letter with no button, a digit and the Ctrl / Alt combos never run a button.
func TestPopupHotkeyMisses(t *testing.T) {
	s, tank := hotkeyScreen(t)

	for _, key := range []int{'z', '8', '9', platform.CtrlMod | 's', platform.AltMod | 's', platform.KeyReturn} {
		if s.popupHotkey(key) {
			t.Errorf("key %d ran a popup button, want a miss", key)
		}
	}

	if tank.Order != game.OrderAwait {
		t.Errorf("order = %v after the misses, want OrderAwait untouched", tank.Order)
	}
}

// TestPopupHotkeyRefusals: a disabled unit, a moving one, an enemy unit, the AI turn and a hand over banner take
// no hotkeys.
func TestPopupHotkeyRefusals(t *testing.T) {
	cases := []struct {
		name  string
		setup func(s *Screen, u *game.UnitInfo)
	}{
		{"disabled", func(_ *Screen, u *game.UnitInfo) { u.Order = game.OrderDisable }},
		{"moving", func(_ *Screen, u *game.UnitInfo) { u.Order, u.State = game.OrderMove, game.OrderStateInProgress }},
		{"enemy unit", func(_ *Screen, u *game.UnitInfo) { u.Team = 1 }},
		{"ai turn", func(s *Screen, _ *game.UnitInfo) { s.turnFlow.aiTurn = true }},
		{"hand over", func(s *Screen, _ *game.UnitInfo) { s.turnFlow.handover = 0 }},
		{"build placement", func(s *Screen, _ *game.UnitInfo) { s.buildUI.buildIdx = 1 }},
	}

	for _, c := range cases {
		s, tank := hotkeyScreen(t)
		c.setup(s, tank)
		before := tank.Order

		if s.popupHotkey('s') || tank.Order != before {
			t.Errorf("%s: S ran (order %v -> %v), want a refusal", c.name, before, tank.Order)
		}
	}
}

// TestPopupHotkeyRefusesParkedBuilder: a Constructor parked build ready on its finished building takes none.
func TestPopupHotkeyRefusesParkedBuilder(t *testing.T) {
	g, c, _, _ := finishedConstructor(t)
	s := &Screen{game: g}
	s.turnFlow.handover = -1
	s.selected = c

	if _, ok := s.popupHotkeyItem('d'); ok {
		t.Error("D on a parked build ready Constructor found a button, want a refusal")
	}
}

// TestPopupHotkeyUsesTheOpenPopup: with the popup open, a letter runs the shown button.
func TestPopupHotkeyUsesTheOpenPopup(t *testing.T) {
	s, tank := hotkeyScreen(t)
	ran := false
	s.orderInput.popupOpen = true
	s.orderInput.popupUnit = tank
	s.orderInput.popupItems = []popupItem{{label: "Zap", action: func() { ran = true }}}

	if !s.popupHotkey('z') || !ran {
		t.Error("Z did not run the open popup's Zap button")
	}
}
