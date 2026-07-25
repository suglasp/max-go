// submarine_detect_test.go : tests the "Submarine detected" stealth sea detection cue.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// TestSubmarineDetectedPlaysCue checks the player's sonar revealing a hostile
// submarine speaks the DISTINCT "Submarine detected!" line (V_F201), not the
// generic "Enemy detected!" (V_F070/F071), announces once per detection, and
// rearms when the sub slips back out of sonar coverage.
func TestSubmarineDetectedPlaysCue(t *testing.T) {
	s, g := newClickScreen(t)
	var voices []string
	s.announcer.voiceFn = func(tag string) { voices = append(voices, tag) }
	s.sound.sfxFn = func(primary, _ string, _ float64) {} // swallow the PING blip
	s.announcer.clock = func() uint64 { return 0 }

	// A player Corvette (sonar) covers the cell a hostile submarine sits on.
	corvette := mkUnit(g, assets.Corvette, 1, 5, 5, game.FlagMobileSeaUnit|game.FlagSelectable)
	corvette.Values.Set(game.AttribScan, 4)
	sub := game.NewUnitInfo(assets.Submarne, 1, 2, 0) // team 1 = enemy
	sub.Flags = game.FlagMobileSeaUnit | game.FlagSelectable
	sub.Values = &game.UnitValues{}
	sub.Order = game.OrderAwait
	g.Units.Deploy(sub, 6, 5)
	g.Vis.Rebuild(g.Units.Units())

	s.checkSubmarineDetected()
	if !containsTag(voices, "V_F201") {
		t.Errorf("a detected submarine should speak V_F201, voices=%v", voices)
	}

	// A submarine must NOT also trigger the generic enemy detected cue.
	voices = nil
	s.announcer.enemySeen = nil
	s.checkEnemyDetected()
	if containsTag(voices, "V_F070") || containsTag(voices, "V_F071") {
		t.Errorf("a submarine must not trigger the generic enemy-detected cue, voices=%v", voices)
	}

	// Still in sonar: a second check does not reannounce (latched).
	voices = nil
	s.checkSubmarineDetected()
	if len(voices) != 0 {
		t.Errorf("a still-detected sub should not re-announce, voices=%v", voices)
	}

	// The sub slips out of sonar (corvette gone): the latch rearms, and a later
	// redetection announces again.
	g.Units.Remove(corvette)
	g.Vis.Rebuild(g.Units.Units())
	s.checkSubmarineDetected() // out of coverage: rearm, no cue
	g.Units.Deploy(corvette, 5, 5)
	g.Vis.Rebuild(g.Units.Units())
	voices = nil
	s.checkSubmarineDetected()
	if !containsTag(voices, "V_F201") {
		t.Errorf("a re-detected submarine should announce again, voices=%v", voices)
	}
}

// containsTag reports whether tag appears in the played voice slice.
func containsTag(tags []string, tag string) bool {
	for _, got := range tags {
		if got == tag {
			return true
		}
	}
	return false
}
