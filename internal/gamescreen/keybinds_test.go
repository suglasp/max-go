// keybinds_test.go : tests the configurable key map (keybinds.go).
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/platform"
)

// TestKeybindDefaults: a fresh screen resolves the vanilla keys without any
// SetKeybinds call, letters match both shift cases, and unmapped keys miss.
func TestKeybindDefaults(t *testing.T) {
	s, _ := newTestScreen(t, 16)

	cases := []struct {
		key  int
		want bindAction
	}{
		{platform.KeyReturn, bindEndTurn},
		{platform.KeyEscape, bindMenu},
		{'g', bindGrid}, {'G', bindGrid},
		{platform.CtrlMod | 'r', bindSurvey},
		{platform.CtrlMod | 's', bindStatus},
		{'f', bindFind},
		{'?', bindHelp},
		{platform.CtrlMod | 'o', bindOptions},
		{'=', bindZoomIn},
		{'-', bindZoomOut},
	}

	for _, c := range cases {
		got, ok := anyKeybind(s, c.key)

		if !ok || got != c.want {
			t.Errorf("keybindAction(%d) = %v/%v, want %v", c.key, got, ok, c.want)
		}
	}

	// Unbound keys, the old plain port letters and Alt combos never match.
	for _, key := range []int{'r', 's', 'o'} {
		if action, ok := anyKeybind(s, key); ok {
			t.Errorf("plain %c matched %s, want a miss (the port picks moved to Ctrl)", key, bindNames[action])
		}
	}

	if _, ok := anyKeybind(s, 'z'); ok {
		t.Errorf("keybindAction('z') matched, want a miss")
	}

	if _, ok := anyKeybind(s, platform.AltMod|'s'); ok {
		t.Errorf("keybindAction(Alt+s) matched, want a miss (Alt+S is the save hotkey)")
	}
}

// TestSetKeybindsRemap: a remapped end turn key takes over and the old default
// stops matching; a bad value keeps the default; an empty value unbinds.
func TestSetKeybindsRemap(t *testing.T) {
	s, _ := newTestScreen(t, 16)
	s.SetKeybinds(map[string]string{
		"end_turn": "T",
		"grid":     "no such key",
		"help":     "",
	})

	if got, ok := anyKeybind(s, 't'); !ok || got != bindEndTurn {
		t.Errorf("remapped end_turn: keybindAction('t') = %v/%v, want bindEndTurn", got, ok)
	}

	if _, ok := anyKeybind(s, platform.KeyReturn); ok {
		t.Errorf("KeyReturn still matches after end_turn moved to T")
	}

	if got, ok := anyKeybind(s, 'g'); !ok || got != bindGrid {
		t.Errorf("bad grid value must keep the default: got %v/%v", got, ok)
	}

	if _, ok := anyKeybind(s, '?'); ok {
		t.Errorf("empty help value must unbind the action")
	}
}

// TestAltShortcutsStayOutOfTheKeybinds: the vanilla Alt hotkeys (Files, Pause,
// eXit, Save, Load) carry AltMod, so no configurable binding can claim them even
// when the player remaps the plain letter to another action.
func TestAltShortcutsStayOutOfTheKeybinds(t *testing.T) {
	s, _ := newTestScreen(t, 16)
	s.SetKeybinds(map[string]string{"find": "F", "options": "P", "status": "X"})

	for _, key := range []int{platform.AltMod | 'f', platform.AltMod | 'p', platform.AltMod | 'x'} {
		if action, ok := anyKeybind(s, key); ok {
			t.Errorf("Alt+%c was claimed by the %s binding, want a miss", key&0xff, bindNames[action])
		}
	}
}

// TestPauseMessageComesFromTheStrings: the pause box reads the retail text and
// falls back to the vanilla wording when the strings are missing.
func TestPauseMessageComesFromTheStrings(t *testing.T) {
	s, _ := newTestScreen(t, 16)

	if got := s.pauseMessage(); got != "Game Paused.\nClick OK to continue." {
		t.Errorf("pause fallback = %q, want the vanilla wording", got)
	}

	s.lang = assets.Strings{"f9bb": "Spel gepauzeerd."}

	if got := s.pauseMessage(); got != "Spel gepauzeerd." {
		t.Errorf("pause text = %q, want the string from the language file", got)
	}
}

// TestPauseDialogIsInertWithoutAHook: no message hook (headless, benches) means
// Alt+P does nothing instead of reaching into a nil modal: no modal opens and the turn clock is untouched.
func TestPauseDialogIsInertWithoutAHook(t *testing.T) {
	s, _ := newTestScreen(t, 16)
	clock := s.turnFlow.turnStart
	s.pauseDialog()

	if s.turnFlow.turnStart != clock || s.turnFlow.wdModalDepth.Load() != 0 {
		t.Errorf("no hook: turn clock %d -> %d, modal depth %d, want untouched", clock, s.turnFlow.turnStart, s.turnFlow.wdModalDepth.Load())
	}
}

// TestKeybindSurveyTogglesOverlay: the survey action flips the same field the
// panel SurveyButton click flips.
func TestKeybindSurveyTogglesOverlay(t *testing.T) {
	s, _ := newTestScreen(t, 16)

	if s.surveyOverlay {
		t.Fatal("survey overlay unexpectedly on at start")
	}

	s.runKeybindAction(bindSurvey, nil)

	if !s.surveyOverlay {
		t.Errorf("survey action did not raise the overlay")
	}

	s.runKeybindAction(bindStatus, nil)

	if !s.showStatus {
		t.Errorf("status action did not raise the status icons")
	}
}

// anyKeybind matches a key against both action groups, the vanilla ones first, the way handleGameKey tries
// them around the popup hotkeys.
func anyKeybind(s *Screen, key int) (bindAction, bool) {
	if action, ok := s.keybindAction(key, false); ok {
		return action, true
	}

	return s.keybindAction(key, true)
}

// TestPortPickGroup: the vanilla group never returns a port action and the port group never a vanilla one.
func TestPortPickGroup(t *testing.T) {
	s, _ := newTestScreen(t, 16)

	if _, ok := s.keybindAction(platform.CtrlMod|'s', false); ok {
		t.Error("Ctrl+S (status) matched in the vanilla group")
	}

	if got, ok := s.keybindAction(platform.CtrlMod|'s', true); !ok || got != bindStatus {
		t.Errorf("Ctrl+S in the port group = %v/%v, want bindStatus", got, ok)
	}

	if _, ok := s.keybindAction('g', true); ok {
		t.Error("G (grid) matched in the port group")
	}
}
