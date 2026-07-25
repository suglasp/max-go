// v18_test.go : save format v18 upgrade tests for the saveload package.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package saveload

import (
	"bytes"
	"testing"
)

// TestSaveLoadV18Strategy checks that each computer team's strategy+1 value survives a round trip.
func TestSaveLoadV18Strategy(t *testing.T) {
	g := newTestGame(t)
	g.State.AIStrategy[1] = 5 // team 1 committed to some strategy (index 4 + 1)
	g.State.AIStrategy[2] = 7 // team 2 committed to another (index 6 + 1)

	var buf bytes.Buffer
	if err := Save(&buf, Capture(g, "GREEN_1.WRL")); err != nil {
		t.Fatalf("save: %v", err)
	}

	snap, err := Load(&buf)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if snap.AIStrategy[1] != 5 || snap.AIStrategy[2] != 7 {
		t.Errorf("snapshot AIStrategy = %v, want [_ 5 7 _]", snap.AIStrategy)
	}

	g2 := snap.Restore(landMap(16, 16))
	if g2.State.AIStrategy[1] != 5 || g2.State.AIStrategy[2] != 7 {
		t.Errorf("restored AIStrategy = %v, want the committed strategies preserved", g2.State.AIStrategy)
	}
}

// TestRestorePreV18KeepsUnsetStrategy: a save written before v18 carries no strategies, so
// restore leaves every slot at 0 (unset), and each team's SelectStrategy roll stands (the
// prior behaviour).
func TestRestorePreV18KeepsUnsetStrategy(t *testing.T) {
	snap := Capture(newTestGame(t), "GREEN_1.WRL")
	snap.Version = 17          // pretend an older build wrote this
	snap.AIStrategy = [4]int{} // gob zero fill for a field the old stream never had

	g2 := snap.Restore(landMap(16, 16))
	for team, v := range g2.State.AIStrategy {
		if v != 0 {
			t.Errorf("pre-v18 restore AIStrategy[%d] = %d, want 0 (unset)", team, v)
		}
	}
}
