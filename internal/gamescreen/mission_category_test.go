// mission_category_test.go : tests the rules that key off the running mission's kind.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/screendata"
	"github.com/suglasp/max-go/internal/voice"
)

// TestBuildPlacementPromptSpeaksInTraining verifies that a TRAINING mission plays the V_F050 "position the tape" cue when a builder arms placement.
func TestBuildPlacementPromptSpeaksInTraining(t *testing.T) {
	// A constructor arming SELECT_SITE speaks the prompt.
	con, _ := builderScreen(t, assets.Constrct, 8, 8, -1, -1,
		screendata.BuildSelection{Type: assets.MiningSt, Rate: 1, Path: false})
	con.SetMissionCategory(game.MissionTraining)

	var conPlayed []string
	con.announcer.voiceFn = func(tag string) { conPlayed = append(conPlayed, tag) }
	con.announcer.clock = func() uint64 { return 1000 }
	con.openBuild()

	if !spokePlacementPrompt(conPlayed) {
		t.Errorf("a training constructor SELECT_SITE must speak the prompt, got %v", conPlayed)
	}

	// So does an engineer arming a path (line) build.
	path, _ := builderScreen(t, assets.Engineer, 8, 8, -1, -1,
		screendata.BuildSelection{Type: assets.Road, Rate: 1, Path: true})
	path.SetMissionCategory(game.MissionTraining)

	var pathPlayed []string
	path.announcer.voiceFn = func(tag string) { pathPlayed = append(pathPlayed, tag) }
	path.announcer.clock = func() uint64 { return 1000 }
	path.openBuild()

	if !spokePlacementPrompt(pathPlayed) {
		t.Errorf("a training engineer path build must speak the prompt, got %v", pathPlayed)
	}

	// A CAMPAIGN mission is still a mission, but it is not teaching: stays silent.
	camp, _ := builderScreen(t, assets.Constrct, 8, 8, -1, -1,
		screendata.BuildSelection{Type: assets.MiningSt, Rate: 1, Path: false})
	camp.SetMissionCategory(game.MissionCampaign)

	var campPlayed []string
	camp.announcer.voiceFn = func(tag string) { campPlayed = append(campPlayed, tag) }
	camp.announcer.clock = func() uint64 { return 1000 }
	camp.openBuild()

	if spokePlacementPrompt(campPlayed) {
		t.Errorf("a campaign mission must not speak the training prompt, got %v", campPlayed)
	}
}

// spokePlacementPrompt reports whether either sample of the placement prompt pool
// (F049/F050, picked at random per arm) is in the played lines.
func spokePlacementPrompt(played []string) bool {
	return spoke(played, voice.BuildPlacementPrompt) || spoke(played, voice.BuildPlacementPromptB)
}

// TestMissionCategoryRidesTheSim checks the category is stored on the game (which is
// what the save captures), so a reloaded training mission is still a training mission.
func TestMissionCategoryRidesTheSim(t *testing.T) {
	s, g := newClickScreen(t)

	if got := s.MissionCategory(); got != game.MissionNone {
		t.Errorf("a fresh game reports %v, want none", got)
	}

	s.SetMissionCategory(game.MissionTraining)

	if got := g.Mission; got != game.MissionTraining {
		t.Errorf("the sim holds %v, want training", got)
	}

	if !g.Mission.IsMission() {
		t.Error("a training mission does not report as a mission")
	}
}
