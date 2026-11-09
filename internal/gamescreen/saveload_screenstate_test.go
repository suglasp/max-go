// saveload_screenstate_test.go : testing unit for the screen level save fields (display toggles, turn clock, mission briefing)
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"os"
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/saveload"
)

// readSnapshotFile opens and decodes a save file for inspection.
func readSnapshotFile(t *testing.T, path string) *saveload.Snapshot {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open save: %v", err)
	}
	defer f.Close()
	snap, err := saveload.Load(f)
	if err != nil {
		t.Fatalf("decode save: %v", err)
	}
	return snap
}

// TestScreenSaveLoadKeepsDisplaysClockAndMission checks that the display toggles, the turn clock and the mission briefing survive a screen save and load.
func TestScreenSaveLoadKeepsDisplaysClockAndMission(t *testing.T) {
	dir := t.TempDir()

	saver, g := newTestScreen(t, 16)
	saver.SetSaveInfo(dir, "GREEN_1.WRL")
	saver.SetTurnTimer(180)
	mkUnit(g, assets.Tank, 0, 1, 5, 5, game.FlagMobileLandUnit|game.FlagSelectable)
	g.Mission = game.MissionTraining
	g.MissionIndex = 1
	saver.SetMissionGoal("Training 1", "Objective: learn to build a base.")

	// A non default set of overlays: every other toggle differs from its neighbour.
	saver.surveyOverlay, saver.showStatus, saver.showColors = true, false, true
	saver.showHits, saver.showAmmo, saver.showRange = false, true, false
	saver.showScan, saver.grid, saver.showNames = true, false, true

	// A running clock with 120 of 180 seconds left.
	saver.turnFlow.turnSeconds = 120
	saver.turnFlow.turnStart = platform.Ticks()

	if err := saver.saveToSlot(1, "screen state", false); err != nil {
		t.Fatalf("save: %v", err)
	}

	// A fresh screen with the opposite toggles loads it, like coming in from the main menu.
	loader, _ := newTestScreen(t, 16)
	loader.SetSaveInfo(dir, "GREEN_1.WRL")
	loader.SetTurnTimer(180)
	loader.surveyOverlay, loader.showStatus, loader.showColors = false, true, false
	loader.showHits, loader.showAmmo, loader.showRange = true, false, true
	loader.showScan, loader.grid, loader.showNames = false, true, false

	if err := loader.Load(1); err != nil {
		t.Fatalf("load: %v", err)
	}

	got := loader.captureDisplayToggles()
	want := saver.captureDisplayToggles()
	if *got != *want {
		t.Errorf("display toggles after the load = %+v, want %+v", *got, *want)
	}
	if left := loader.turnRemaining(); left < 110 || left > 120 {
		t.Errorf("turn clock after the load = %ds, want the saved remainder (about 120)", left)
	}
	if loader.missionTitle != "Training 1" || loader.missionBriefing != "Objective: learn to build a base." {
		t.Errorf("mission briefing after the load = %q / %q, want the saved title and text",
			loader.missionTitle, loader.missionBriefing)
	}
}

// TestScreenSaveWritesScreenFields checks that the snapshot a screen writes carries the screen level fields that Capture leaves empty.
func TestScreenSaveWritesScreenFields(t *testing.T) {
	dir := t.TempDir()

	saver, g := newTestScreen(t, 16)
	saver.SetSaveInfo(dir, "GREEN_1.WRL")
	saver.SetTurnTimer(180)
	mkUnit(g, assets.Tank, 0, 1, 5, 5, game.FlagMobileLandUnit|game.FlagSelectable)
	g.Mission = game.MissionTraining
	saver.SetMissionGoal("Training 1", "Objective: learn to build a base.")
	saver.grid = true
	saver.turnFlow.turnSeconds = 120
	saver.turnFlow.turnStart = platform.Ticks()

	if err := saver.saveToSlot(2, "named", false); err != nil {
		t.Fatalf("save: %v", err)
	}
	path, ok := saveload.FindSlot(dir, 2)
	if !ok {
		t.Fatal("the saved slot file was not found")
	}
	snap := readSnapshotFile(t, path)

	if snap.Name != "named" {
		t.Errorf("Snapshot.Name = %q, want %q", snap.Name, "named")
	}
	if snap.Displays == nil || !snap.Displays.Grid {
		t.Errorf("Snapshot.Displays = %+v, want the grid toggle set", snap.Displays)
	}
	if snap.TurnSeconds < 110 || snap.TurnSeconds > 120 {
		t.Errorf("Snapshot.TurnSeconds = %d, want about 120", snap.TurnSeconds)
	}
	if snap.MissionTitle != "Training 1" || snap.MissionBrief == "" {
		t.Errorf("Snapshot mission fields = %q / %q, want the title and the briefing", snap.MissionTitle, snap.MissionBrief)
	}
}
