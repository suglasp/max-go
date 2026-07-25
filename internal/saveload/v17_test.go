// v17_test.go : save format v17 upgrade tests for the saveload package.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package saveload

import (
	"bytes"
	"testing"
)

// TestSaveLoadV17WinGoal checks that each computer team's goal+1 value survives a round trip.
func TestSaveLoadV17WinGoal(t *testing.T) {
	g := newTestGame(t)
	g.State.AIWinGoal[1] = 2 // team 1 committed to colonise (goalColonize+1)
	g.State.AIWinGoal[2] = 1 // team 2 committed to kill (goalKill+1)

	var buf bytes.Buffer
	if err := Save(&buf, Capture(g, "GREEN_1.WRL")); err != nil {
		t.Fatalf("save: %v", err)
	}

	snap, err := Load(&buf)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if snap.AIWinGoal[1] != 2 || snap.AIWinGoal[2] != 1 {
		t.Errorf("snapshot AIWinGoal = %v, want [_ 2 1 _]", snap.AIWinGoal)
	}

	g2 := snap.Restore(landMap(16, 16))
	if g2.State.AIWinGoal[1] != 2 || g2.State.AIWinGoal[2] != 1 {
		t.Errorf("restored AIWinGoal = %v, want the committed goals preserved", g2.State.AIWinGoal)
	}
}

// TestRestorePreV17KeepsUnsetWinGoal: a save written before v17 carries no goals, so
// restore leaves every slot at 0 (unset), and the AI rolls its goal fresh on the
// first turn (the prior behaviour).
func TestRestorePreV17KeepsUnsetWinGoal(t *testing.T) {
	snap := Capture(newTestGame(t), "GREEN_1.WRL")
	snap.Version = 16         // pretend an older build wrote this
	snap.AIWinGoal = [4]int{} // gob zero fill for a field the old stream never had

	g2 := snap.Restore(landMap(16, 16))
	for team, v := range g2.State.AIWinGoal {
		if v != 0 {
			t.Errorf("pre-v17 restore AIWinGoal[%d] = %d, want 0 (unset)", team, v)
		}
	}
}
