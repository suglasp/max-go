// panelanim_test.go : covers the instrument bar flipbook that plays once the landing
// is done: frame selection, when it ends, and the fallback when the art is absent.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/gfx"
)

// panelAnimArt fakes the two flipbooks at their reference frame counts and sizes: five
// top frames (256 wide, growing 17 to 24 tall) and four bottom frames (346x24).
func panelAnimArt() panelAnimState {
	var pa panelAnimState

	for i := 0; i < 5; i++ {
		pa.top = append(pa.top, gfx.New(256, 17+i*2))
	}

	for i := 0; i < 4; i++ {
		pa.btm = append(pa.btm, gfx.New(346, 24))
	}

	return pa
}

// TestPanelAnimFrameHoldsTheLast checks the step to frame mapping. The top strip runs
// one step longer than the bottom, so on the final step the bottom must hold its last
// frame rather than vanish or panic. C++: the BPNLSQ_1 + i <= BPNLSQ_4 guard.
func TestPanelAnimFrameHoldsTheLast(t *testing.T) {
	pa := panelAnimArt()

	for step := 0; step < panelAnimSteps; step++ {
		top := panelAnimFrame(pa.top, step)
		btm := panelAnimFrame(pa.btm, step)

		if top == nil || btm == nil {
			t.Fatalf("step %d: a bar has no frame to draw", step)
		}

		if want := pa.top[min(step, 4)]; top != want {
			t.Errorf("step %d: wrong top frame", step)
		}

		// Step 4 is past the bottom's last frame (index 3), so it holds there.
		if want := pa.btm[min(step, 3)]; btm != want {
			t.Errorf("step %d: the bottom bar must hold its final frame, not run off the end", step)
		}
	}
}

// TestPanelAnimFrameSurvivesNoArt verifies that missing art draws no flipbook and leaves the bars visible.
func TestPanelAnimFrameSurvivesNoArt(t *testing.T) {
	if got := panelAnimFrame(nil, 0); got != nil {
		t.Error("absent art must yield no frame instead of panicking")
	}

	s := &Screen{} // no art loaded

	if s.panelAnimPlaying() {
		t.Error("with no flipbook art the animation must not run, so the real bars draw at once")
	}
}

// TestPanelAnimEndsAndStaysEnded checks the animation runs once, then hands the bars
// over to the live chrome for good. A flipbook that restarted every frame would hide
// the End Turn button forever.
func TestPanelAnimEndsAndStaysEnded(t *testing.T) {
	s := &Screen{panelAnim: panelAnimArt()}

	if !s.panelAnimPlaying() {
		t.Fatal("the flipbook must start on the first frame after landing")
	}

	if s.panelAnim.start == 0 {
		t.Fatal("the first frame must stamp the start tick")
	}

	// Wind the clock back past the whole run: 5 frames at 42ms.
	s.panelAnim.start -= panelAnimSteps * panelAnimFrameMS

	if s.panelAnimPlaying() {
		t.Error("the flipbook must end once its frames are spent")
	}

	if !s.panelAnim.done {
		t.Error("ending the flipbook must latch done")
	}

	if s.panelAnimPlaying() {
		t.Error("a finished flipbook must stay finished, or it would hide the bars forever")
	}
}

// TestPanelAnimBlitsAtTheInstrumentWindows verifies that frames land in the two instrument windows in 4:3 and widescreen. C++: WINDOW_TOP_INSTRUMENTS_WINDOW (384,0) and WINDOW_BOTTOM_INSTRUMENTS_WINDOW (240,456).
func TestPanelAnimBlitsAtTheInstrumentWindows(t *testing.T) {
	const topInk, btmInk uint8 = 0x41, 0x42

	newScreen := func() *Screen {
		s := &Screen{panelAnim: panelAnimArt()}
		s.panelAnim.top[0].Fill(topInk)
		s.panelAnim.btm[0].Fill(btmInk)
		s.panelAnimPlaying() // stamp the start tick so drawPanelAnim is on step 0

		return s
	}

	cases := []struct {
		name          string
		wide          bool
		width         int
		topX, bottomX int
	}{
		// 4:3 puts the bars exactly where the window table does.
		{"classic", false, 640, 384, 240},
		// Widescreen adds 214px on the right: the top bar keeps its offset from the
		// right edge, the bottom one centres under the widened map (extra / 2).
		{"widescreen", true, 854, 384 + 214, 240 + 107},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ApplyWidescreen(tc.wide)
			t.Cleanup(func() { ApplyWidescreen(false) }) // the layout table is package level

			s := newScreen()
			buf := gfx.New(tc.width, 480)
			s.drawPanelAnim(buf)

			if got := buf.At(tc.topX, panelAnimTopY); got != topInk {
				t.Errorf("top bar frame is not at x=%d: got colour %#x, want %#x", tc.topX, got, topInk)
			}

			if got := buf.At(tc.bottomX, panelAnimBottomY); got != btmInk {
				t.Errorf("bottom bar frame is not at x=%d: got colour %#x, want %#x", tc.bottomX, got, btmInk)
			}

			// Nothing may leak to the left of either bar: that is the map and the left panel.
			if got := buf.At(tc.topX-1, panelAnimTopY); got == topInk {
				t.Error("the top bar frame starts one pixel too far left")
			}

			if got := buf.At(tc.bottomX-1, panelAnimBottomY); got == btmInk {
				t.Error("the bottom bar frame starts one pixel too far left")
			}
		})
	}
}

// TestReplayPanelAnimRearms checks a load plays the bars in again, which is what the
// original does when it rebuilds them. C++: GameManager_LoadGame.
func TestReplayPanelAnimRearms(t *testing.T) {
	s := &Screen{panelAnim: panelAnimArt()}
	s.panelAnim.done = true
	s.panelAnim.start = 1234

	s.replayPanelAnim()

	if s.panelAnim.done || s.panelAnim.start != 0 {
		t.Fatal("a load must re arm the flipbook")
	}

	if !s.panelAnimPlaying() {
		t.Error("the bars must unfold again after a load")
	}
}
