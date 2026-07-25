// layout_test.go : tests layout.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/gfx"
)

func TestCornerMarker(t *testing.T) {
	// A 16px box (0,0)-(15,15) with 4px arms: each corner draws a 5px L, the
	// middle of every edge stays empty; the selection brackets, not a box.
	buf := gfx.New(16, 16)
	drawCornerMarker(buf, 0, 0, 15, 15, 4, 0xFF)
	// Corners and arm ends are set.
	for _, p := range [][2]int{{0, 0}, {4, 0}, {0, 4}, {15, 0}, {11, 0}, {0, 15}, {15, 15}} {
		if buf.At(p[0], p[1]) != 0xFF {
			t.Errorf("corner pixel %v not drawn", p)
		}
	}
	// The middle of each edge (between the brackets) is empty.
	for _, p := range [][2]int{{7, 0}, {8, 0}, {0, 8}, {7, 15}, {15, 8}} {
		if buf.At(p[0], p[1]) != 0 {
			t.Errorf("edge-gap pixel %v should be empty", p)
		}
	}
	// The centre is empty (brackets only, no fill).
	if buf.At(8, 8) != 0 {
		t.Error("marker centre should be empty")
	}
}

func TestDisplayName(t *testing.T) {
	if got := displayName(assets.MiningSt); got != "Mining Station" {
		t.Errorf("displayName(MiningSt) = %q, want %q", got, "Mining Station")
	}
	// An unmapped type falls back to its resource tag, never panics.
	if got := displayName(assets.Waldo); got != assets.Waldo.Tag() {
		t.Errorf("displayName(Waldo) = %q, want tag %q", got, assets.Waldo.Tag())
	}
}

func TestMarkerColorBlinks(t *testing.T) {
	// The marker colour is one of the two blink phases (white or black).
	if c := markerColor(); c != 0 && c != 0xFF {
		t.Errorf("markerColor = %d, want 0 or 255", c)
	}
}

func TestFormatClock(t *testing.T) {
	for _, c := range []struct {
		sec  int
		want string
	}{{0, "00:00"}, {5, "00:05"}, {20, "00:20"}, {65, "01:05"}, {180, "03:00"}} {
		if got := formatClock(c.sec); got != c.want {
			t.Errorf("formatClock(%d) = %q, want %q", c.sec, got, c.want)
		}
	}
}

func TestTurnRemaining(t *testing.T) {
	s := &Screen{turnFlow: turnFlowState{turnSeconds: 180, turnStart: 1000}}
	// Just started: the full budget is left.
	if got := s.remainingAt(1000); got != 180 {
		t.Errorf("remainingAt(start) = %d, want 180", got)
	}
	// 19s in: 161 left (and not yet red territory).
	if got := s.remainingAt(1000 + 19*1000); got != 161 {
		t.Errorf("remainingAt(+19s) = %d, want 161", got)
	}
	// Past the budget: clamps to zero (the automatic end trigger).
	if got := s.remainingAt(1000 + 200*1000); got != 0 {
		t.Errorf("remainingAt(+200s) = %d, want 0", got)
	}
	// A disabled timer always reads zero.
	s.turnFlow.turnSeconds = 0
	if got := s.remainingAt(1000 + 5*1000); got != 0 {
		t.Errorf("disabled remainingAt = %d, want 0", got)
	}
}

// TestResumeTurnClockPreservesBudget verifies the modal pause fix: time spent in a
// blocking dialog (exit confirm, Save/Load/Options) must not be charged against the
// turn timer, else a long deliberation expires the turn the instant it is dismissed.
func TestResumeTurnClockPreservesBudget(t *testing.T) {
	s := &Screen{turnFlow: turnFlowState{turnSeconds: 180, turnStart: 1000}}
	// At 4s in, 176s remain.
	if got := s.remainingAt(5000); got != 176 {
		t.Fatalf("pre-pause remaining = %d, want 176", got)
	}
	// A dialog sat open for 60s (t=5000..65000). Compensate the turn clock.
	s.resumeTurnClock(5000, 65000)
	// Right after dismissal the budget is unchanged: the paused 60s did not count.
	if got := s.remainingAt(65000); got != 176 {
		t.Errorf("post-pause remaining = %d, want 176 (the paused 60s must not count)", got)
	}
	// A backwards / zero span does nothing (never extends the budget).
	before := s.turnFlow.turnStart
	s.resumeTurnClock(65000, 5000)
	if s.turnFlow.turnStart != before {
		t.Errorf("a backwards span moved turnStart: %d -> %d", before, s.turnFlow.turnStart)
	}
}

// TestWithTurnClockPausedMarksModal verifies the stall watchdog gate: while the
// blocking modal's fn runs, wdModalDepth is raised (so the watchdog skips the
// parked frame), and it returns to 0 afterwards. Nested modals stack.
func TestWithTurnClockPausedMarksModal(t *testing.T) {
	s := &Screen{}
	if d := s.turnFlow.wdModalDepth.Load(); d != 0 {
		t.Fatalf("initial modal depth = %d, want 0", d)
	}
	s.withTurnClockPaused(func() {
		if d := s.turnFlow.wdModalDepth.Load(); d != 1 {
			t.Errorf("modal depth inside fn = %d, want 1", d)
		}
		// A modal opening another dialog nests: depth reaches 2.
		s.withTurnClockPaused(func() {
			if d := s.turnFlow.wdModalDepth.Load(); d != 2 {
				t.Errorf("nested modal depth = %d, want 2", d)
			}
		})
		if d := s.turnFlow.wdModalDepth.Load(); d != 1 {
			t.Errorf("depth after nested modal = %d, want 1", d)
		}
	})
	if d := s.turnFlow.wdModalDepth.Load(); d != 0 {
		t.Errorf("modal depth after fn = %d, want 0 (watchdog re-armed)", d)
	}
}

func TestColorStat(t *testing.T) {
	for _, c := range []struct {
		cur, max int
		want     uint8
	}{
		{100, 100, 0x02}, // full -> green
		{51, 100, 0x02},  // just above half -> green
		{50, 100, 0x04},  // exactly half -> yellow
		{26, 100, 0x04},  // just above a quarter -> yellow
		{25, 100, 0x01},  // exactly a quarter -> red
		{0, 100, 0x01},   // empty -> red
	} {
		if got := colorStat(c.cur, c.max); got != c.want {
			t.Errorf("colorStat(%d,%d) = %#x, want %#x", c.cur, c.max, got, c.want)
		}
	}
}

func TestStatBarFill(t *testing.T) {
	if got := statBarFill(100, 100, 77); got != 77 {
		t.Errorf("full bar = %d, want 77", got)
	}
	if got := statBarFill(50, 100, 77); got != 38 {
		t.Errorf("half bar = %d, want 38", got)
	}
	if got := statBarFill(0, 100, 77); got != 0 {
		t.Errorf("empty bar = %d, want 0", got)
	}
	// Degenerate inputs clamp, never divide by zero or overrun.
	if got := statBarFill(5, 0, 77); got != 0 {
		t.Errorf("zero max = %d, want 0", got)
	}
	if got := statBarFill(200, 100, 77); got != 77 {
		t.Errorf("over-full clamps to %d, want 77", got)
	}
}

func TestViewSize(t *testing.T) {
	if w, h := viewSize(MainMap); w != 448 || h != 448 {
		t.Errorf("MainMap viewSize = %dx%d, want 448x448", w, h)
	}
	if w, h := viewSize(Minimap); w != 112 || h != 112 {
		t.Errorf("Minimap viewSize = %dx%d, want 112x112", w, h)
	}
}

func TestCenterScroll(t *testing.T) {
	// Cell (10,10) at zoom 16 in a 448x448 viewport: its center pixel is
	// 10*16+8 = 168, minus half the viewport (224) -> -56 (clamped later).
	if x, y := centerScroll(10, 10, 16, 448, 448); x != -56 || y != -56 {
		t.Errorf("centerScroll = %d,%d, want -56,-56", x, y)
	}
}

func TestMinimapCell(t *testing.T) {
	// Top left corner of the minimap maps to cell (0,0).
	if cx, cy, ok := minimapCell(Minimap.ULX, Minimap.ULY, 112, 112); !ok || cx != 0 || cy != 0 {
		t.Errorf("minimap top-left = %d,%d ok=%v, want 0,0 true", cx, cy, ok)
	}
	// One minimap pixel == one cell when the map is 112x112.
	if cx, cy, ok := minimapCell(Minimap.ULX+56, Minimap.ULY+30, 112, 112); !ok || cx != 56 || cy != 30 {
		t.Errorf("minimap interior = %d,%d ok=%v, want 56,30 true", cx, cy, ok)
	}
	// A coarse map: clicks divide down to the cell count.
	if cx, cy, ok := minimapCell(Minimap.ULX+56, Minimap.ULY+56, 8, 8); !ok || cx != 4 || cy != 4 {
		t.Errorf("coarse map = %d,%d ok=%v, want 4,4 true", cx, cy, ok)
	}
	// Outside the box.
	if _, _, ok := minimapCell(Minimap.LRX+1, Minimap.ULY, 112, 112); ok {
		t.Error("point right of minimap reported inside")
	}
	if _, _, ok := minimapCell(MainMap.ULX, MainMap.ULY, 112, 112); ok {
		t.Error("point in map viewport reported inside minimap")
	}
}

func TestEndTurnButtonHit(t *testing.T) {
	// The top bar End Turn button rect (390,2..461,22) is hit on its corners
	// and centre, and missed just outside and over the turn counter slot.
	for _, tc := range []struct {
		name string
		x, y int
		want bool
	}{
		{"top-left corner", EndTurnButton.ULX, EndTurnButton.ULY, true},
		{"bottom-right corner", EndTurnButton.LRX, EndTurnButton.LRY, true},
		{"centre", 425, 12, true},
		{"left of button", EndTurnButton.ULX - 1, 12, false},
		{"over turn counter", TurnCounter.ULX, 12, false},
		{"below button", 425, EndTurnButton.LRY + 1, false},
	} {
		if got := inRect(EndTurnButton, tc.x, tc.y); got != tc.want {
			t.Errorf("%s: inRect(EndTurnButton, %d, %d) = %v, want %v", tc.name, tc.x, tc.y, got, tc.want)
		}
	}
}

func TestEdgeScroll(t *testing.T) {
	for _, tc := range []struct {
		name           string
		mx, my, wx, wy int
	}{
		{"left", 2, 240, -scrollStep, 0},
		{"right", 637, 240, scrollStep, 0},
		{"top", 320, 2, 0, -scrollStep},
		{"bottom", 320, 477, 0, scrollStep},
		{"top-left corner", 1, 1, -scrollStep, -scrollStep},
		{"center", 320, 240, 0, 0},
	} {
		if dx, dy := edgeScroll(tc.mx, tc.my, scrollStep); dx != tc.wx || dy != tc.wy {
			t.Errorf("%s: edgeScroll = %d,%d, want %d,%d", tc.name, dx, dy, tc.wx, tc.wy)
		}
	}
}
