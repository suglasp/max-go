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
	s, _ := newClickScreen(t)

	cases := []struct {
		key  int
		want bindAction
	}{
		{platform.KeyReturn, bindEndTurn},
		{platform.KeyEscape, bindMenu},
		{'g', bindGrid}, {'G', bindGrid},
		{'r', bindSurvey},
		{'s', bindStatus}, {'S', bindStatus},
		{'f', bindFind},
		{'?', bindHelp},
		{'o', bindOptions},
		{'=', bindZoomIn},
		{'-', bindZoomOut},
	}

	for _, c := range cases {
		got, ok := s.keybindAction(c.key)

		if !ok || got != c.want {
			t.Errorf("keybindAction(%d) = %v/%v, want %v", c.key, got, ok, c.want)
		}
	}

	// Unbound keys and Alt combos never match a plain binding.
	if _, ok := s.keybindAction('z'); ok {
		t.Errorf("keybindAction('z') matched, want a miss")
	}

	if _, ok := s.keybindAction(platform.AltMod | 's'); ok {
		t.Errorf("keybindAction(Alt+s) matched, want a miss (Alt+S is the save hotkey)")
	}
}

// TestSetKeybindsRemap: a remapped end turn key takes over and the old default
// stops matching; a bad value keeps the default; an empty value unbinds.
func TestSetKeybindsRemap(t *testing.T) {
	s, _ := newClickScreen(t)
	s.SetKeybinds(map[string]string{
		"end_turn": "T",
		"grid":     "no such key",
		"help":     "",
	})

	if got, ok := s.keybindAction('t'); !ok || got != bindEndTurn {
		t.Errorf("remapped end_turn: keybindAction('t') = %v/%v, want bindEndTurn", got, ok)
	}

	if _, ok := s.keybindAction(platform.KeyReturn); ok {
		t.Errorf("KeyReturn still matches after end_turn moved to T")
	}

	if got, ok := s.keybindAction('g'); !ok || got != bindGrid {
		t.Errorf("bad grid value must keep the default: got %v/%v", got, ok)
	}

	if _, ok := s.keybindAction('?'); ok {
		t.Errorf("empty help value must unbind the action")
	}
}

// TestAltShortcutsStayOutOfTheKeybinds: the vanilla Alt hotkeys (Files, Pause,
// eXit, Save, Load) carry AltMod, so no configurable binding can claim them even
// when the player remaps the plain letter to another action.
func TestAltShortcutsStayOutOfTheKeybinds(t *testing.T) {
	s, _ := newClickScreen(t)
	s.SetKeybinds(map[string]string{"find": "F", "options": "P", "status": "X"})

	for _, key := range []int{platform.AltMod | 'f', platform.AltMod | 'p', platform.AltMod | 'x'} {
		if action, ok := s.keybindAction(key); ok {
			t.Errorf("Alt+%c was claimed by the %s binding, want a miss", key&0xff, bindNames[action])
		}
	}
}

// TestPauseMessageComesFromTheStrings: the pause box reads the retail text and
// falls back to the vanilla wording when the strings are missing.
func TestPauseMessageComesFromTheStrings(t *testing.T) {
	s, _ := newClickScreen(t)

	if got := s.pauseMessage(); got != "Game Paused.\nClick OK to continue." {
		t.Errorf("pause fallback = %q, want the vanilla wording", got)
	}

	s.lang = assets.Strings{"f9bb": "Spel gepauzeerd."}

	if got := s.pauseMessage(); got != "Spel gepauzeerd." {
		t.Errorf("pause text = %q, want the string from the language file", got)
	}
}

// TestPauseDialogIsInertWithoutAHook: no message hook (headless, benches) means
// Alt+P does nothing instead of reaching into a nil modal.
func TestPauseDialogIsInertWithoutAHook(t *testing.T) {
	s, _ := newClickScreen(t)
	s.pauseDialog()
}

// TestKeybindSurveyTogglesOverlay: the survey action flips the same field the
// panel SurveyButton click flips.
func TestKeybindSurveyTogglesOverlay(t *testing.T) {
	s, _ := newClickScreen(t)

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
