// widescreen_test.go : tests widescreen.go.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package gamescreen

import (
	"testing"

	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/platform"
)

func TestWidenFrameBands(t *testing.T) {
	// A synthetic 640x480 frame with per column values that make the three
	// bands distinguishable: column index modulo 251 (fits a byte, and no
	// repeats near a band edge).
	src := gfx.New(platform.ScreenWidth, platform.ScreenHeight)
	for y := 0; y < src.H; y++ {
		row := src.Row(y)
		for x := range row {
			row[x] = uint8(x % 251)
		}
	}

	out := widenFrame(src, platform.WideWidth)
	if out.W != platform.WideWidth || out.H != platform.ScreenHeight {
		t.Fatalf("widened frame = %dx%d, want %dx%d", out.W, out.H, platform.WideWidth, platform.ScreenHeight)
	}

	// Left band (the left panel): copied verbatim.
	for _, x := range []int{0, 100, frameLeftBand - 1} {
		if out.At(x, 240) != src.At(x, 240) {
			t.Errorf("left band col %d = %d, want %d", x, out.At(x, 240), src.At(x, 240))
		}
	}
	// Right band: the source's last columns moved to the new right edge.
	for i := 0; i < frameRightBand; i++ {
		gotX := platform.WideWidth - frameRightBand + i
		wantX := platform.ScreenWidth - frameRightBand + i
		if out.At(gotX, 240) != src.At(wantX, 240) {
			t.Errorf("right band col %d = %d, want src col %d = %d", gotX, out.At(gotX, 240), wantX, src.At(wantX, 240))
		}
	}
	// Middle band edges: the first stretched pixel is the first source middle
	// column, the last stretched pixel maps inside the source middle band.
	if out.At(frameLeftBand, 240) != src.At(frameLeftBand, 240) {
		t.Errorf("middle band start = %d, want %d", out.At(frameLeftBand, 240), src.At(frameLeftBand, 240))
	}
	lastMiddle := platform.WideWidth - frameRightBand - 1
	srcMiddle := platform.ScreenWidth - frameLeftBand - frameRightBand
	dstMiddle := platform.WideWidth - frameLeftBand - frameRightBand
	wantX := frameLeftBand + (dstMiddle-1)*srcMiddle/dstMiddle
	if out.At(lastMiddle, 240) != src.At(wantX, 240) {
		t.Errorf("middle band end = %d, want src col %d = %d", out.At(lastMiddle, 240), wantX, src.At(wantX, 240))
	}

	// nil and already wide inputs pass through untouched.
	if widenFrame(nil, platform.WideWidth) != nil {
		t.Error("widenFrame(nil) != nil")
	}
	if widenFrame(out, platform.WideWidth) != out {
		t.Error("an already wide frame must return as is")
	}
}

func TestApplyWidescreenOffsets(t *testing.T) {
	// Snapshot the 4:3 table, apply wide, verify the moves, then restore.
	defer ApplyWidescreen(false)
	narrowMainWindow, narrowMainMap := MainWindow, MainMap
	narrowEndTurn, narrowCounter, narrowTimer := EndTurnButton, TurnCounter, TurnTimer
	narrowCoords, narrowUnitName := CoordsDisplay, UnitNameDisplay
	narrowMinimap, narrowStat := Minimap, StatWindow

	ApplyWidescreen(true)
	extra := platform.WideWidth - platform.ScreenWidth

	if MainWindow.LRX != narrowMainWindow.LRX+extra || MainMap.LRX != narrowMainMap.LRX+extra {
		t.Errorf("screen/map not widened: MainWindow.LRX=%d MainMap.LRX=%d", MainWindow.LRX, MainMap.LRX)
	}
	if MainMap.ULX != narrowMainMap.ULX {
		t.Errorf("map viewport left edge moved: %d", MainMap.ULX)
	}
	// Top strip: right anchored, the whole rect shifts by the extra width.
	if EndTurnButton.ULX != narrowEndTurn.ULX+extra || TurnCounter.ULX != narrowCounter.ULX+extra || TurnTimer.LRX != narrowTimer.LRX+extra {
		t.Errorf("top strip not right anchored: EndTurn=%+v Counter=%+v Timer=%+v", EndTurnButton, TurnCounter, TurnTimer)
	}
	// The offset from the right screen edge must be IDENTICAL to 4:3.
	if MainWindow.LRX-TurnTimer.LRX != narrowMainWindow.LRX-narrowTimer.LRX {
		t.Errorf("turn timer right edge offset changed: %d vs %d", MainWindow.LRX-TurnTimer.LRX, narrowMainWindow.LRX-narrowTimer.LRX)
	}
	// Bottom strip: centered under the widened map (+extra/2).
	if CoordsDisplay.ULX != narrowCoords.ULX+extra/2 || UnitNameDisplay.LRX != narrowUnitName.LRX+extra/2 {
		t.Errorf("bottom strip not centered: Coords=%+v UnitName=%+v", CoordsDisplay, UnitNameDisplay)
	}
	// The left panel never moves.
	if Minimap != narrowMinimap || StatWindow != narrowStat {
		t.Errorf("left panel moved: Minimap=%+v StatWindow=%+v", Minimap, StatWindow)
	}

	// The switch is exactly reversible.
	ApplyWidescreen(false)
	if MainWindow != narrowMainWindow || MainMap != narrowMainMap ||
		EndTurnButton != narrowEndTurn || TurnCounter != narrowCounter || TurnTimer != narrowTimer ||
		CoordsDisplay != narrowCoords || UnitNameDisplay != narrowUnitName {
		t.Error("ApplyWidescreen(false) did not restore the exact 640 table")
	}
}
