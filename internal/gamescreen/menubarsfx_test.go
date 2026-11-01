// menubarsfx_test.go : tests the in game menu bar button blip.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import "testing"

// TestPlayMenuBarSfx: the menu bar helper plays the MBUTT0 blip through the sfx
// hook, and a nil hook is a safe no op.
func TestPlayMenuBarSfx(t *testing.T) {
	var played []string
	s := &Screen{sound: soundState{sfxFn: func(primary, fallback string, pan float64) { played = append(played, primary) }}}

	s.playMenuBarSfx()

	if len(played) != 1 || played[0] != "MBUTT0" {
		t.Fatalf("playMenuBarSfx played %v, want [MBUTT0]", played)
	}

	(&Screen{}).playMenuBarSfx() // nil sfxFn must not panic
}

// TestPlayToggleSfx: the left panel overlay toggles play the IHITS0 blip (all the
// per button tags resolve to the same ihits0.wav), and a nil hook is safe.
func TestPlayToggleSfx(t *testing.T) {
	var played []string
	s := &Screen{sound: soundState{sfxFn: func(primary, fallback string, pan float64) { played = append(played, primary) }}}

	s.playToggleSfx()

	if len(played) != 1 || played[0] != "IHITS0" {
		t.Fatalf("playToggleSfx played %v, want [IHITS0]", played)
	}

	(&Screen{}).playToggleSfx() // nil sfxFn must not panic
}
