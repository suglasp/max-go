// chrome_test.go : tests toolbar geometry, edge-scroll gating, and rotated labels.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package mapeditor

import (
	"testing"

	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/platform"
)

func editorTestBuffer() *gfx.Buffer { return gfx.New(640, 480) }

// TestOverUI checks the tool-bar hit region that edge scroll must avoid: the
// whole left strip when open, only the corner unfold button when folded.
func TestOverUI(t *testing.T) {
	e := &Editor{rowH: 16, unfoldW: 64}
	cases := []struct {
		name   string
		folded bool
		x, y   int
		want   bool
	}{
		{"left bar open", false, 10, 200, true},
		{"cluster open", false, 150, 10, true},
		{"map open right", false, 400, 200, false},
		{"map open top", false, 300, 5, false},
		{"unfold button folded", true, 10, 10, true},
		{"map left mid folded", true, 10, 200, false},
		{"map right folded", true, 400, 200, false},
	}
	for _, c := range cases {
		e.barFolded = c.folded
		if got := e.overUI(c.x, c.y); got != c.want {
			t.Errorf("%s: overUI(%d,%d) folded=%v = %v, want %v", c.name, c.x, c.y, c.folded, got, c.want)
		}
	}
}

// TestMinimapRect checks the minimap sits bottom-left, above the status strip.
func TestMinimapRect(t *testing.T) {
	e := &Editor{}
	buf := editorTestBuffer()
	mm := e.minimapRect(buf)

	if mm.ULX != uiMargin {
		t.Errorf("minimap ULX = %d, want %d", mm.ULX, uiMargin)
	}
	if mm.Width()+1 != MINIMAP_SIZE {
		t.Errorf("minimap width = %d, want %d", mm.Width()+1, MINIMAP_SIZE)
	}
	if mm.LRY != buf.H-statusH-uiMargin-1 {
		t.Errorf("minimap LRY = %d, want %d", mm.LRY, buf.H-statusH-uiMargin-1)
	}
}

// TestEdgeScroll checks the viewport pans against a screen edge, but not while
// the cursor is over a tool-bar overlay.
func TestEdgeScroll(t *testing.T) {
	buf := editorTestBuffer()
	at := func(x, y int) platform.MouseState { return platform.MouseState{X: x, Y: y} }

	// Right edge over the open map scrolls right.
	e := &Editor{rowH: 16, unfoldW: 64}
	e.edgeScroll(buf, at(buf.W-1, 240))
	if e.scrollX <= 0 {
		t.Errorf("right edge: scrollX = %d, want > 0", e.scrollX)
	}

	// Left edge over the open bar does not scroll.
	e = &Editor{rowH: 16, unfoldW: 64}
	e.edgeScroll(buf, at(1, 240))
	if e.scrollX != 0 {
		t.Errorf("left edge over bar: scrollX = %d, want 0", e.scrollX)
	}

	// Left edge with the bar folded (clear of the corner button) scrolls left.
	e = &Editor{rowH: 16, unfoldW: 64, barFolded: true}
	e.edgeScroll(buf, at(1, 240))
	if e.scrollX >= 0 {
		t.Errorf("left edge folded: scrollX = %d, want < 0", e.scrollX)
	}

	// Middle of the map does not scroll.
	e = &Editor{rowH: 16, unfoldW: 64}
	e.edgeScroll(buf, at(320, 240))
	if e.scrollX != 0 || e.scrollY != 0 {
		t.Errorf("center: scroll = (%d,%d), want (0,0)", e.scrollX, e.scrollY)
	}
}

// TestRotateInto checks the quarter-turn used for the vertical tab labels: a
// src row reads bottom to top in dst, background pixels are skipped, and writes
// stay in bounds.
func TestRotateInto(t *testing.T) {
	dst := gfx.New(20, 20)
	src := gfx.New(3, 2) // 3 wide, 2 tall
	src.Row(0)[0] = 5    // tx=0, ty=0 -> the first pixel of the top row
	src.Row(1)[2] = 7    // tx=2, ty=1 -> the last pixel of the bottom row

	rotateInto(dst, src, 4, 10)

	// tx=0,ty=0 lands at (4+0, 10+(3-1-0)) = (4,12).
	if got := dst.Row(12)[4]; got != 5 {
		t.Errorf("rotated (0,0): dst[12][4] = %d, want 5", got)
	}
	// tx=2,ty=1 lands at (4+1, 10+(3-1-2)) = (5,10).
	if got := dst.Row(10)[5]; got != 7 {
		t.Errorf("rotated (2,1): dst[10][5] = %d, want 7", got)
	}

	// The rest of the destination stays background.
	set := 0
	for y := 0; y < dst.H; y++ {
		row := dst.Row(y)
		for x := 0; x < dst.W; x++ {
			if row[x] != 0 {
				set++
			}
		}
	}
	if set != 2 {
		t.Errorf("set pixels = %d, want 2 (background pixels must be skipped)", set)
	}
}
