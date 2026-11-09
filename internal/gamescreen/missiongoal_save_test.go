// missiongoal_save_test.go : tests that a saved mission reloads as that mission, not a skirmish.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"strings"
	"testing"

	"github.com/suglasp/max-go/internal/game"
)

// TestSavedMissionKeepsItsBriefing verifies that reloading a campaign or scenario restores its mission briefing instead of the skirmish "Random game:" block.
func TestSavedMissionKeepsItsBriefing(t *testing.T) {
	dir := t.TempDir()

	saver, g := newTestScreen(t, 16)
	saver.SetSaveInfo(dir, "GREEN_1.WRL")
	g.Mission = game.MissionCampaign
	saver.SetMissionGoal("Mission 3", "Objective: hold the eastern ridge for 30 turns.")

	if err := saver.saveToSlot(1, "mission", false); err != nil {
		t.Fatalf("save: %v", err)
	}

	loader, _ := newTestScreen(t, 16)
	loader.SetSaveInfo(dir, "GREEN_1.WRL")

	if err := loader.Load(1); err != nil {
		t.Fatalf("load: %v", err)
	}

	if !strings.Contains(loader.gameGoal, "hold the eastern ridge") {
		t.Errorf("the reloaded mission's GOAL text = %q, want its own briefing back", loader.gameGoal)
	}
	if strings.Contains(loader.gameGoal, "Random game:") {
		t.Error("the reloaded mission shows the skirmish info block instead of its objective")
	}
	if got := loader.chatLabel(); got != "GOAL" {
		t.Errorf("chatLabel = %q, want GOAL for a reloaded mission", got)
	}
}

// TestSavedSkirmishKeepsTheInfoBlock verifies that a save without a mission briefing rebuilds the "Random game:" block from restored state.
func TestSavedSkirmishKeepsTheInfoBlock(t *testing.T) {
	dir := t.TempDir()

	saver, _ := newTestScreen(t, 16)
	saver.SetSaveInfo(dir, "GREEN_1.WRL")
	saver.SetRandomGameGoal()

	if err := saver.saveToSlot(1, "skirmish", false); err != nil {
		t.Fatalf("save: %v", err)
	}

	loader, _ := newTestScreen(t, 16)
	loader.SetSaveInfo(dir, "GREEN_1.WRL")

	if err := loader.Load(1); err != nil {
		t.Fatalf("load: %v", err)
	}

	if !strings.Contains(loader.gameGoal, "Random game:") {
		t.Errorf("a reloaded skirmish's GOAL text = %q, want the random game info block", loader.gameGoal)
	}
}
