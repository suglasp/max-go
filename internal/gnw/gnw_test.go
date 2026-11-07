// gnw_test.go : tests window ordering, composition, and input.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gnw

import (
	"testing"

	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/testutil"
)

func newSys(t *testing.T, w, h int) (*System, *gfx.Buffer) {
	t.Helper()
	screen := gfx.New(w, h)
	fs, err := fonts.LoadSet(testutil.DataDir(t))
	if err != nil {
		t.Fatal(err)
	}
	return New(screen, fs), screen
}

func TestBackgroundWindow(t *testing.T) {
	s, screen := newSys(t, 64, 48)
	s.Fill(1, 0, 0, 64, 48, 9)
	s.Composite(-100, -100)
	if screen.At(0, 0) != 9 || screen.At(63, 47) != 9 {
		t.Error("window 1 (background) not composited")
	}
}

func TestResizeScreen(t *testing.T) {
	fs, err := fonts.LoadSet(testutil.DataDir(t))
	if err != nil {
		t.Fatal(err)
	}
	p := platform.New(platform.Config{})
	s := New(p.Screen(), fs)

	s.ResizeScreen(p, platform.WideWidth, platform.ScreenHeight)
	background := s.windows[0]
	if background.buf.W != platform.WideWidth || background.buf.H != platform.ScreenHeight {
		t.Fatalf("background buffer = %dx%d, want %dx%d", background.buf.W, background.buf.H, platform.WideWidth, platform.ScreenHeight)
	}
	if background.rect.LRX != platform.WideWidth-1 {
		t.Fatalf("background rect LRX = %d, want %d", background.rect.LRX, platform.WideWidth-1)
	}
	if s.screen.W != platform.WideWidth {
		t.Fatalf("screen buffer W = %d, want %d (platform resize not applied)", s.screen.W, platform.WideWidth)
	}

	// A window parked on the wide right edge must be pulled back on screen
	// when the resolution shrinks back to 4:3.
	wide := s.AddWindow(800, 10, 40, 20, 3, 0)
	s.ResizeScreen(p, platform.ScreenWidth, platform.ScreenHeight)
	r, ok := s.WinRect(wide)
	if !ok || r.LRX > platform.ScreenWidth-1 {
		t.Fatalf("window not clamped after shrink: %+v", r)
	}
	if s.windows[0].buf.W != platform.ScreenWidth {
		t.Fatalf("background buffer W after restore = %d, want %d", s.windows[0].buf.W, platform.ScreenWidth)
	}

	// Composite must cover the whole restored screen without panicking.
	s.Fill(1, 0, 0, platform.ScreenWidth, platform.ScreenHeight, 9)
	s.Composite(-100, -100)
	if p.Screen().At(platform.ScreenWidth-1, platform.ScreenHeight-1) != 9 {
		t.Error("background not composited after resize round trip")
	}
}

func TestZOrderAndMoveOnTop(t *testing.T) {
	s, screen := newSys(t, 64, 48)

	top := s.AddWindow(0, 0, 10, 10, 5, WindowMoveOnTop) // stays above
	s.AddWindow(0, 0, 10, 10, 6, 0)                      // inserted below the MoveOnTop stratum (was mid, see the Show block)

	s.Composite(-100, -100)
	if screen.At(5, 5) != 5 {
		t.Errorf("MoveOnTop window not on top: got %d", screen.At(5, 5))
	}

	/* // note: unused code in Go port
	// the check of Show, commented out with it.
	// Show(mid) raises it within its stratum but never above MoveOnTop.
	s.Show(mid)
	s.Composite(-100, -100)
	if screen.At(5, 5) != 5 {
		t.Error("Show raised a normal window above the MoveOnTop stratum")
	}
	*/

	// Deleting the top window exposes the one below.
	s.DeleteWindow(top)
	s.Composite(-100, -100)
	if screen.At(5, 5) != 6 {
		t.Errorf("after delete, got %d want 6", screen.At(5, 5))
	}
}

func TestHideShowTransparent(t *testing.T) {
	s, screen := newSys(t, 64, 48)
	s.Fill(1, 0, 0, 64, 48, 9)

	s.AddWindow(4, 4, 8, 8, 3, 0)
	/* // note: unused code in Go port
	// the checks of Hide and Show, commented out with them.
	s.Hide(w)
	s.Composite(-100, -100)
	if screen.At(6, 6) != 9 {
		t.Error("hidden window was composited")
	}
	s.Show(w)
	*/
	s.Composite(-100, -100)
	if screen.At(6, 6) != 3 {
		t.Error("shown window missing")
	}

	// Transparent window: color 0 pixels let the background through.
	tw := s.AddWindow(4, 4, 8, 8, 0, WindowTransparent)
	s.Fill(tw, 0, 0, 4, 8, 2) // left half opaque color 2, right half 0
	s.Composite(-100, -100)
	if screen.At(5, 6) != 2 {
		t.Error("transparent window opaque pixel missing")
	}
	if screen.At(10, 6) != 3 {
		t.Errorf("transparent pixel should show window below, got %d", screen.At(10, 6))
	}
}

func TestMoveClamping(t *testing.T) {
	s, _ := newSys(t, 64, 48)
	w := s.AddWindow(100, 100, 20, 20, 1, 0) // off screen position clamps
	r, _ := s.WinRect(w)
	if r.LRX != 63 || r.LRY != 47 {
		t.Errorf("clamp on add: %+v", r)
	}
	/* // note: unused code in Go port
	// the check of Move, commented out with it.
	s.Move(w, -10, -10)
	r, _ = s.WinRect(w)
	if r.ULX != 0 || r.ULY != 0 {
		t.Errorf("clamp on move: %+v", r)
	}
	*/
}

func TestPrint(t *testing.T) {
	s, screen := newSys(t, 200, 48)
	if err := s.Print(1, "TEST", 0, 5, 5, 0x04); err != nil {
		t.Fatal(err)
	}
	s.Composite(-100, -100)
	ink := 0
	for _, c := range screen.Pix {
		if c == 4 {
			ink++
		}
	}
	if ink == 0 {
		t.Error("Print drew nothing")
	}
	if err := s.Print(1, "x", 0, 5, 45, 0x04); err == nil {
		t.Error("Print below window bottom should fail")
	}
}

func TestCursor(t *testing.T) {
	s, screen := newSys(t, 64, 48)
	s.Fill(1, 0, 0, 64, 48, 9)

	s.Composite(20, 20) // cursor hidden by default
	if screen.At(20, 20) != 9 {
		t.Error("cursor drawn while hidden")
	}

	s.MouseShow()
	s.Composite(20, 20)
	if screen.At(20, 20) != 0 { // arrow tip is the black outline pixel
		t.Errorf("cursor tip = %d, want 0", screen.At(20, 20))
	}
	if screen.At(21, 21) == 9 && screen.At(22, 22) == 9 {
		t.Error("cursor fill missing")
	}

	// Partially off screen must not panic.
	s.Composite(63, 47)
	s.Composite(-5, -5)
}

func TestInputPump(t *testing.T) {
	s, _ := newSys(t, 64, 48)

	s.events <- 'a'
	if got := s.GetInput(); got != 'a' {
		t.Errorf("GetInput = %d, want 'a'", got)
	}

	// Idle frame: a tick yields -1.
	s.ticks <- struct{}{}
	if got := s.GetInput(); got != -1 {
		t.Errorf("GetInput on tick = %d, want -1", got)
	}

	// Queued input wins over a pending tick eventually; drain both.
	s.events <- 'b'
	s.ticks <- struct{}{}
	got1 := s.GetInput()
	got2 := s.GetInput()
	if !(got1 == 'b' && got2 == -1) && !(got1 == -1 && got2 == 'b') {
		t.Errorf("pump returned %d,%d", got1, got2)
	}
}
