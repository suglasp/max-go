// mission_category_test.go : tests the AI tier a mission runs at and the file to category map.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package main

import (
	"testing"

	"github.com/suglasp/max-go/internal/ai"
	"github.com/suglasp/max-go/internal/game"
)

// TestMissionDifficultyLocksTraining checks that training missions always use the most passive computer tier, regardless of setup or -difficulty.
// Other mission categories honour the selected tier.
func TestMissionDifficultyLocksTraining(t *testing.T) {
	restore := difficultyOverride
	defer func() { difficultyOverride = restore }()

	difficultyOverride = -1 // no CLI override: the setup screen's value applies

	if got := missionDifficulty(game.MissionTraining, ai.OpponentGod); got != ai.OpponentClueless {
		t.Errorf("training with God chosen = %d, want the locked Clueless (%d)", got, ai.OpponentClueless)
	}

	if got := missionDifficulty(game.MissionCampaign, ai.OpponentExpert); got != ai.OpponentExpert {
		t.Errorf("campaign with Expert chosen = %d, want Expert (%d)", got, ai.OpponentExpert)
	}

	// A -difficulty override beats the setup screen, but never the training lock.
	difficultyOverride = ai.OpponentGod

	if got := missionDifficulty(game.MissionTraining, ai.OpponentAverage); got != ai.OpponentClueless {
		t.Errorf("training with -difficulty God = %d, want the locked Clueless", got)
	}

	if got := missionDifficulty(game.MissionScenario, ai.OpponentAverage); got != ai.OpponentGod {
		t.Errorf("scenario with -difficulty God = %d, want God", got)
	}
}

// TestMissionCategoryOfFile maps each retail mission format to its category, so a
// -mission launch of a .TRA file behaves like the menu's training flow.
func TestMissionCategoryOfFile(t *testing.T) {
	for _, tc := range []struct {
		path string
		want game.MissionCategory
	}{
		{"TRAINING1.TRA", game.MissionTraining},
		{"c:/games/max/SCE_001.sce", game.MissionScenario},
		{"MISSION1.CAM", game.MissionCampaign},
		{"DUEL.MPS", game.MissionMultiScenario},
		{"SAVE1.DMO", game.MissionDemo},
		{"SAVE1.SAV", game.MissionNone},
		{"nothing", game.MissionNone},
	} {
		if got := missionCategoryOf(tc.path); got != tc.want {
			t.Errorf("missionCategoryOf(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}
