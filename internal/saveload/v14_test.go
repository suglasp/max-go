// v14_test.go : save format v14 upgrade tests for the saveload package.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package saveload

import (
	"bytes"
	"testing"

	"github.com/suglasp/max-go/internal/game"
)

// TestSaveLoadV14Difficulty checks that the v14 opponent tier survives a round trip.
func TestSaveLoadV14Difficulty(t *testing.T) {
	g := newTestGame(t)
	g.State.Difficulty = 5 // God: an explicitly chosen tier

	var buf bytes.Buffer
	if err := Save(&buf, Capture(g, "GREEN_1.WRL")); err != nil {
		t.Fatalf("save: %v", err)
	}
	snap, err := Load(&buf)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if snap.OpponentTier != 5 {
		t.Errorf("snapshot OpponentTier = %d, want 5", snap.OpponentTier)
	}
	g2 := snap.Restore(landMap(16, 16))
	if g2.State.Difficulty != 5 {
		t.Errorf("restored difficulty = %d, want 5 (God) preserved across save/load", g2.State.Difficulty)
	}
}

// TestRestorePreV14KeepsUnsetDifficulty: a save written before v14 carries no tier, so
// restore must keep the "unset" default (the AI's launch behaviour) rather than the
// CLUELESS(0) that gob's zero fill leaves in the field.
func TestRestorePreV14KeepsUnsetDifficulty(t *testing.T) {
	snap := Capture(newTestGame(t), "GREEN_1.WRL")
	snap.Version = 13     // pretend an older build wrote this
	snap.OpponentTier = 0 // gob zero fill for a field the old stream never had

	g2 := snap.Restore(landMap(16, 16))
	if g2.State.Difficulty != game.DifficultyUnset {
		t.Errorf("pre-v14 restore difficulty = %d, want DifficultyUnset(%d)", g2.State.Difficulty, game.DifficultyUnset)
	}
}
