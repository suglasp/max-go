// view_controls_test.go : tests viewport scroll, zoom, and edge pan controls on the game screen.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/platform"
)

// TestHandleViewControlsScrolls verifies the view only camera helper that keeps the
// map scrollable/zoomable during the AI turn wait (the function the aiTurn branch
// now calls): a mouse at the map's right edge edge scrolls the view right, and a
// centred mouse over no minimap control leaves the view put and reports nothing
// consumed (so handleMouse can fall through to gameplay input).
func TestHandleViewControlsScrolls(t *testing.T) {
	s := &Screen{}
	s.turnFlow.handover = -1 // not a hot seat hand over; let the minimap controls run

	// A mouse at the right edge auto scrolls the view right (clickScroll defaults
	// off). wheelY 0 so the zoom path (which needs the renderer) is not taken.
	s.camera.scrollX, s.camera.scrollY = 100, 100
	if consumed := s.handleViewControls(platform.MouseState{X: 637, Y: 240}, 0); consumed {
		t.Error("edge scroll should not report a minimap control consumed the click")
	}
	if s.camera.scrollX <= 100 {
		t.Errorf("right-edge mouse should edge-scroll the view right, scrollX = %d, want > 100", s.camera.scrollX)
	}

	// A centred mouse over no edge and no minimap control leaves the view put.
	s.camera.scrollX, s.camera.scrollY = 100, 100
	if consumed := s.handleViewControls(platform.MouseState{X: 320, Y: 240}, 0); consumed {
		t.Error("a centred mouse should not consume a minimap control")
	}
	if s.camera.scrollX != 100 || s.camera.scrollY != 100 {
		t.Errorf("a centred mouse should not scroll, scroll = %d,%d, want 100,100", s.camera.scrollX, s.camera.scrollY)
	}
}
