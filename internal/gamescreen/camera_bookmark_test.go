// camera_bookmark_test.go : tests the four per-player camera bookmarks in screen_locations.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/saveload"
)

// TestViewCenterCellInvertsCenterScroll: viewCenterCell is the exact inverse of
// centerScroll, so a cell pinned into a bookmark reads back as the same cell at
// whatever zoom is current.
func TestViewCenterCellInvertsCenterScroll(t *testing.T) {
	s, _ := newTestScreen(t, 16)
	zoom := s.r.Zoom()
	vw, vh := viewSize(MainMap)

	for _, c := range []struct{ x, y int }{{0, 0}, {3, 11}, {7, 9}, {15, 2}} {
		s.camera.scrollX, s.camera.scrollY = centerScroll(c.x, c.y, zoom, vw, vh)
		gotX, gotY := s.viewCenterCell()
		if gotX != c.x || gotY != c.y {
			t.Errorf("viewCenterCell round trip for (%d,%d) = (%d,%d)", c.x, c.y, gotX, gotY)
		}
	}
}

// TestCameraBookmarkSetAndRecall: Alt+F5 pins the current view centre and F5 jumps
// straight back to it; an unset slot and an out of range slot are quiet no ops.
func TestCameraBookmarkSetAndRecall(t *testing.T) {
	s, _ := newTestScreen(t, 16)
	zoom := s.r.Zoom()
	vw, vh := viewSize(MainMap)

	// Aim the view at a known cell, then pin it into slot 0.
	const cellX, cellY = 6, 9
	s.camera.scrollX, s.camera.scrollY = centerScroll(cellX, cellY, zoom, vw, vh)
	s.setCameraBookmark(0)

	if b := s.camera.cameraBookmarks[0]; !b.set || b.cellX != cellX || b.cellY != cellY {
		t.Fatalf("slot 0 = %+v; want the pinned cell (%d,%d) set", b, cellX, cellY)
	}

	// Recall lands on the clamped centre of the stored cell - the same result the
	// code computes, so it holds whether or not the small map clamps the scroll.
	wantX, wantY := centerScroll(cellX, cellY, zoom, vw, vh)
	wantX, wantY = s.r.ClampScroll(wantX, wantY, vw, vh)

	s.camera.scrollX, s.camera.scrollY = -999, -999 // scroll away first
	s.recallCameraBookmark(0)
	if s.camera.scrollX != wantX || s.camera.scrollY != wantY {
		t.Errorf("recall slot 0 scrolled to (%d,%d); want (%d,%d)", s.camera.scrollX, s.camera.scrollY, wantX, wantY)
	}

	// An unset slot leaves the view where it is.
	s.camera.scrollX, s.camera.scrollY = 42, 24
	s.recallCameraBookmark(1)
	if s.camera.scrollX != 42 || s.camera.scrollY != 24 {
		t.Errorf("recall of an empty slot moved the view to (%d,%d)", s.camera.scrollX, s.camera.scrollY)
	}

	// Out of range slots must not panic or store anything.
	s.setCameraBookmark(-1)
	s.setCameraBookmark(len(s.camera.cameraBookmarks))
	s.recallCameraBookmark(99)
}

// TestCameraBookmarkSaveRoundTrip: captureCameraBookmarks stores only the assigned
// slots and applyCameraBookmarks replaces the current pins with the save's set,
// clearing everything a pre v15 save (an empty list) or the saved game never pinned.
func TestCameraBookmarkSaveRoundTrip(t *testing.T) {
	s, _ := newTestScreen(t, 16)
	s.camera.cameraBookmarks[1] = cameraBookmark{cellX: 6, cellY: 9, set: true}
	s.camera.cameraBookmarks[2] = cameraBookmark{cellX: 40, cellY: 12, set: true}

	marks := s.captureCameraBookmarks()
	if len(marks) != 2 {
		t.Fatalf("captured %d bookmarks, want the 2 assigned slots", len(marks))
	}
	if marks[0] != (saveload.CameraBookmarkSnapshot{Slot: 1, CellX: 6, CellY: 9}) ||
		marks[1] != (saveload.CameraBookmarkSnapshot{Slot: 2, CellX: 40, CellY: 12}) {
		t.Errorf("captured bookmarks = %+v, want slots 1 and 2 with their cells", marks)
	}

	// Loading onto a screen with different pins replaces them wholesale.
	s2, _ := newTestScreen(t, 16)
	s2.camera.cameraBookmarks[0] = cameraBookmark{cellX: 1, cellY: 1, set: true}
	s2.applyCameraBookmarks(marks)
	if s2.camera.cameraBookmarks[0].set {
		t.Errorf("slot 0 kept its old pin after a load; want it cleared")
	}
	if b := s2.camera.cameraBookmarks[1]; !b.set || b.cellX != 6 || b.cellY != 9 {
		t.Errorf("slot 1 after load = %+v, want the saved pin (6,9)", b)
	}
	if b := s2.camera.cameraBookmarks[2]; !b.set || b.cellX != 40 || b.cellY != 12 {
		t.Errorf("slot 2 after load = %+v, want the saved pin (40,12)", b)
	}

	// A pre v15 save has no bookmark list: every pin clears. A hostile slot index
	// must not panic or land anywhere.
	s2.applyCameraBookmarks(nil)
	for i, b := range s2.camera.cameraBookmarks {
		if b.set {
			t.Errorf("slot %d still set after applying an empty save", i)
		}
	}
	s2.applyCameraBookmarks([]saveload.CameraBookmarkSnapshot{{Slot: 99, CellX: 1, CellY: 1}})
	for i, b := range s2.camera.cameraBookmarks {
		if b.set {
			t.Errorf("slot %d set by an out of range save entry", i)
		}
	}
}
