// v15_test.go : save format v15 upgrade tests for the saveload package.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package saveload

import (
	"bytes"
	"testing"
)

// TestSaveLoadV15Bookmarks checks the screen-owned camera bookmarks attached after Capture.
func TestSaveLoadV15Bookmarks(t *testing.T) {
	snap := Capture(newTestGame(t), "GREEN_1.WRL")
	snap.Bookmarks = []CameraBookmarkSnapshot{
		{Slot: 0, CellX: 6, CellY: 9},
		{Slot: 3, CellX: 40, CellY: 12},
	}

	var buf bytes.Buffer
	if err := Save(&buf, snap); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := Load(&buf)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got.Bookmarks) != 2 {
		t.Fatalf("loaded %d bookmarks, want 2", len(got.Bookmarks))
	}
	if got.Bookmarks[0] != (CameraBookmarkSnapshot{Slot: 0, CellX: 6, CellY: 9}) ||
		got.Bookmarks[1] != (CameraBookmarkSnapshot{Slot: 3, CellX: 40, CellY: 12}) {
		t.Errorf("loaded bookmarks = %+v, want slots 0 and 3 back unchanged", got.Bookmarks)
	}
}
