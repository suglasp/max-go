// panelanim.go : plays the top and bottom instrument-bar flipbooks after landing.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : game_manager.cpp - GameManager_MenuAnimateDisplayControls
package gamescreen

import (
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/units"
)

// Where the two instrument bars sit, from the window table in window_manager.cpp:
// WINDOW_TOP_INSTRUMENTS_WINDOW is (384,0)-(639,24) and WINDOW_BOTTOM_INSTRUMENTS_WINDOW
// is (240,456)-(585,479). The art matches those rects exactly (256 and 346 wide).
// Widescreen keeps the top bar at its offset from the right edge and centres the
// bottom one under the widened map, the same rule the live readouts follow.
const (
	panelAnimTopX    = 384
	panelAnimTopY    = 0
	panelAnimBottomX = 240
	panelAnimBottomY = 456

	// panelAnimSteps is klei's loop count, and each step is held at 24 FPS, so the
	// whole thing is over in about a fifth of a second.
	panelAnimSteps   = 5
	panelAnimFrameMS = 42
)

// panelAnimState holds the instrument-bar flipbook art and its current state.
type panelAnimState struct {
	top   []*gfx.Buffer // PNLSEQ_1..5, the top bar unfolding downward
	btm   []*gfx.Buffer // BPNLSQ_1..4, the bottom bar unfolding upward
	start uint64        // tick the flipbook began; 0 until the first live frame
	done  bool          // the bars are up, so the real chrome draws from here on
}

// loadPanelAnimArt reads the two flipbooks. Missing art makes the bars appear immediately.
func loadPanelAnimArt(lib units.Reader) panelAnimState {
	var pa panelAnimState

	for _, tag := range []string{"PNLSEQ_1", "PNLSEQ_2", "PNLSEQ_3", "PNLSEQ_4", "PNLSEQ_5"} {
		if art := loadSimpleArt(lib, tag); art != nil {
			pa.top = append(pa.top, art)
		}
	}

	for _, tag := range []string{"BPNLSQ_1", "BPNLSQ_2", "BPNLSQ_3", "BPNLSQ_4"} {
		if art := loadSimpleArt(lib, tag); art != nil {
			pa.btm = append(pa.btm, art)
		}
	}

	return pa
}

// panelAnimPlaying reports whether the bars are still arriving and starts the clock on its first call after landing.
func (s *Screen) panelAnimPlaying() bool {
	pa := &s.panelAnim

	if pa.done || len(pa.top) == 0 {
		return false
	}

	if pa.start == 0 {
		pa.start = platform.Ticks()
	}

	if platform.Ticks()-pa.start >= panelAnimSteps*panelAnimFrameMS {
		pa.done = true

		return false
	}

	return true
}

// replayPanelAnim restarts the flipbook after loading a game. C++: GameManager_LoadGame.
func (s *Screen) replayPanelAnim() {
	s.panelAnim.start = 0
	s.panelAnim.done = false
}

// drawPanelAnim paints the current flipbook frame over both instrument bars,
// standing in for the real chrome until the animation ends.
func (s *Screen) drawPanelAnim(buf *gfx.Buffer) {
	step := int((platform.Ticks() - s.panelAnim.start) / panelAnimFrameMS)
	extra := MainWindow.LRX - 639 // widescreen adds its width on the right

	blitPanelClipped(buf, panelAnimFrame(s.panelAnim.top, step), panelAnimTopX+extra, panelAnimTopY)
	blitPanelClipped(buf, panelAnimFrame(s.panelAnim.btm, step), panelAnimBottomX+extra/2, panelAnimBottomY)
}

// panelAnimFrame picks a frame for one step and holds the last frame past the end. C++: `BPNLSQ_1 + i <= BPNLSQ_4` guard.
func panelAnimFrame(frames []*gfx.Buffer, step int) *gfx.Buffer {
	if len(frames) == 0 {
		return nil
	}

	if step >= len(frames) {
		step = len(frames) - 1
	}

	if step < 0 {
		step = 0
	}

	return frames[step]
}
