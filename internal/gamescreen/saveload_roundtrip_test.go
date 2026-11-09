// saveload_roundtrip_test.go : end to end Screen.Save -> Screen.Load coverage.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/platform"
)

// TestScreenSaveLoadRoundTrip covers the full saveToSlot and Load path, restoring a save into a fresh Screen.
func TestScreenSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()

	saver, g := newTestScreen(t, 16)
	saver.SetSaveInfo(dir, "GREEN_1.WRL")
	saver.SetTurnTimer(180)
	mkUnit(g, assets.Tank, 0, 0, 5, 5, game.FlagMobileLandUnit|game.FlagSelectable)
	saver.logMessage("a message from the game that was saved")

	// Two thirds of the turn is gone when the player saves.
	saver.turnFlow.turnSeconds = 120
	saver.turnFlow.turnStart = platform.Ticks()

	if err := saver.saveToSlot(1, "round trip", false); err != nil {
		t.Fatalf("save: %v", err)
	}

	// A fresh screen loads it, exactly like coming in from the main menu.
	loader, _ := newTestScreen(t, 16)
	loader.SetSaveInfo(dir, "GREEN_1.WRL")
	loader.SetTurnTimer(180)
	loader.logMessage("a message from the game being replaced")

	if err := loader.Load(1); err != nil {
		t.Fatalf("load: %v", err)
	}

	if n := len(loader.game.Units.Units()); n == 0 {
		t.Fatal("the loaded game has no units: the save chain did not round trip")
	}
	if len(loader.banner.msgLog) != 0 {
		t.Errorf("the replaced game's message log survived the load (%d entries)", len(loader.banner.msgLog))
	}
	if left := loader.turnRemaining(); left < 110 || left > 120 {
		t.Errorf("turn clock after the load = %ds, want the saved remainder (about 120), not a full fresh turn", left)
	}
}
