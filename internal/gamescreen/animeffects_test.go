// animeffects_test.go : tests the animated combat/explosion effect spawn and tick logic.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import "testing"

// TestIdleAnimationsRunFollowsEffectsPref verifies the "Animate Effects" pref gates
// the idle animations of an ANIMATED unit (radar/scanner): on by default, frozen when the
// pref is off. This locks the wiring so the effectsOff field can't go dead again.
func TestIdleAnimationsRunFollowsEffectsPref(t *testing.T) {
	s := &Screen{}
	// Default zero value: effectsOff false -> animations run.
	if !s.idleAnimationsRun() {
		t.Error("idle animations should run by default (Animate Effects on)")
	}
	// ApplyPrefs with the pref off sets effectsOff true -> animations freeze.
	s.ApplyPrefs(LivePrefs{AnimateEffects: false})
	if s.idleAnimationsRun() {
		t.Error("idle animations should freeze when Animate Effects is off")
	}
	// Enabling it again resumes them.
	s.ApplyPrefs(LivePrefs{AnimateEffects: true})
	if !s.idleAnimationsRun() {
		t.Error("idle animations should resume when Animate Effects is turned back on")
	}
}
