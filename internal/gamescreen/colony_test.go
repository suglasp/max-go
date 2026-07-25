// colony_test.go : tests Eco Sphere colony started voice line triggering rules.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/game"
)

// TestColonyVoiceMapping verifies the team->"colony started" voice mapping to the
// transcribed clips: red F098, green F103, blue F108, gray F113 (teams 0..3).
func TestColonyVoiceMapping(t *testing.T) {
	want := map[int]string{0: "V_F098", 1: "V_F103", 2: "V_F108", 3: "V_F113"}
	if len(colonyVoice) != game.TeamCount {
		t.Fatalf("colonyVoice has %d entries, want %d", len(colonyVoice), game.TeamCount)
	}
	for team, tag := range want {
		if colonyVoice[team] != tag {
			t.Errorf("colonyVoice[%d] = %q, want %q", team, colonyVoice[team], tag)
		}
	}
}
