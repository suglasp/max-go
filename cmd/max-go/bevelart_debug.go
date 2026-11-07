// bevelart_debug.go : renders beveled button samples through the GNW menu widgets.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package main

import (
	"fmt"

	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/gamescreen"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/gnw"
	"github.com/suglasp/max-go/internal/menus"
	"github.com/suglasp/max-go/internal/platform"
)

type bevelSample struct {
	label string
	off   string
	on    string
}

const (
	bevelStartStop = 6400 + iota
	bevelSentry
	bevelAttack
	bevelRemove
)

// runBevelArtDebug shows the per-order popup button resources beside the generic UNTBTN fallback. It is a visual bench for the long-press popup art.
func runBevelArtDebug(ctx *menus.Context) error {
	const w, h = 430, 360
	win := ctx.Sys.AddWindow((platform.ScreenWidth-w)/2, (platform.ScreenHeight-h)/2, w, h, 0, gnw.WindowMoveOnTop|gnw.WindowModal)
	if win < 0 {
		return fmt.Errorf("bevelart debug: window alloc failed")
	}
	defer ctx.Sys.DeleteWindow(win)

	oldFont := ctx.Sys.Fonts().GetFont()
	ctx.Sys.Fonts().SetFont(fonts.Font1)
	defer ctx.Sys.Fonts().SetFont(oldFont)

	ctx.Sys.MouseShow()
	ctx.Sys.Fill(win, 0, 0, w, h, 160)
	ctx.Sys.ShadedBox(win, 0, 0, w-1, h-1, 255, 0)
	ctx.Sys.ShadedBox(win, 8, 8, w-9, h-9, 148, 255)
	_ = ctx.Sys.Print(win, "Popup bevel art", 200, 20, 16, 0)
	_ = ctx.Sys.Print(win, "OFF", 50, 150, 38, 0)
	_ = ctx.Sys.Print(win, "ON", 50, 235, 38, 0)
	_ = ctx.Sys.Print(win, "Esc exits", 120, 330, 332, 0)

	drawBevelSampleGrid(ctx, win)

	const popupX, popupY = 36, 236
	clickPad := gfx.New(42, 21)
	ctx.Sys.AddButton(win, popupX, popupY, 42, 21, gnw.NoValue, gnw.NoValue, gnw.NoValue, bevelStartStop, clickPad, clickPad, nil, 0)
	ctx.Sys.AddButton(win, popupX, popupY+24, 42, 21, gnw.NoValue, gnw.NoValue, gnw.NoValue, bevelSentry, clickPad, clickPad, nil, 0)
	ctx.Sys.AddButton(win, popupX, popupY+48, 42, 21, gnw.NoValue, gnw.NoValue, gnw.NoValue, bevelAttack, clickPad, clickPad, nil, 0)
	ctx.Sys.AddButton(win, popupX, popupY+72, 42, 21, gnw.NoValue, gnw.NoValue, gnw.NoValue, bevelRemove, clickPad, clickPad, nil, 0)

	return runBevelArtDebugLoop(ctx, win, popupX, popupY)
}

// bevelArtSamples pairs each per order popup button resource with its pressed face,
// in the order the bench lists them.
var bevelArtSamples = []bevelSample{
	{label: "Generic", off: "UNTBTN_U", on: "UNTBTN_D"},
	{label: "Build", off: "BUILD_OF", on: "BUILD_ON"},
	{label: "Attack", off: "ATTK_OF", on: "ATTK_ON"},
	{label: "Sentry", off: "SNTRY_OF", on: "SNTRY_ON"},
	{label: "X-Fer", off: "XFER_OF", on: "XFER_ON"},
	{label: "Load", off: "LOAD_OF", on: "LOAD_ON"},
	{label: "Activate", off: "ACTVT_OF", on: "ACTVT_ON"},
	{label: "Clear", off: "CLEAR_OF", on: "CLEAR_ON"},
	{label: "Stop", off: "STOP_OF", on: "STOP_ON"},
	{label: "Start", off: "START_OF", on: "START_ON"},
	{label: "Reload", off: "RLOAD_OF", on: "RLOAD_ON"},
	{label: "Remove", off: "REMVE_OF", on: "REMVE_ON"},
	{label: "Enter", off: "ENTER_OF", on: "ENTER_ON"},
	{label: "Steal", off: "STEAL_OF", on: "STEAL_ON"},
	{label: "Disable", off: "DISBL_OF", on: "DISBL_ON"},
	{label: "Done", off: "DONE_OF", on: "DONE_ON"},
	{label: "Research", off: "RSRCH_OF", on: "RSRCH_ON"},
}

// drawBevelSampleGrid lists every sample, drawing its off and on faces side by side
// and printing the tag name where a resource is missing.
func drawBevelSampleGrid(ctx *menus.Context, win gnw.WinID) {
	for i, s := range bevelArtSamples {
		x := 22
		y := 60 + i*15
		if i >= 9 {
			x = 228
			y = 60 + (i-9)*15
		}
		_ = ctx.Sys.Print(win, s.label, 84, x, y+2, 0)
		if off := gamescreen.LoadPopupFace(ctx.Lib, s.off); off != nil {
			blitDebugArt(ctx, win, x+88, y, off)
		} else {
			_ = ctx.Sys.Print(win, "missing "+s.off, 110, x+88, y+2, 1)
		}
		if on := gamescreen.LoadPopupFace(ctx.Lib, s.on); on != nil {
			blitDebugArt(ctx, win, x+148, y, on)
		} else {
			_ = ctx.Sys.Print(win, "missing "+s.on, 110, x+148, y+2, 1)
		}
	}
}

// runBevelArtDebugLoop draws the working popup sample and runs the bench input loop
// until Escape, each click toggling one order state and repainting the popup.
func runBevelArtDebugLoop(ctx *menus.Context, win gnw.WinID, popupX, popupY int) error {
	running := true
	sentryOn := true
	status := "Click the rows to toggle/test."
	drawBevelWorkingPopup(ctx, win, popupX, popupY, running, sentryOn, status)

	ctx.Sys.FlushInput()

	for {
		key := ctx.Sys.GetInput()
		switch key {
		case platform.KeyEscape:
			return nil
		case bevelStartStop:
			running = !running
			if running {
				status = "Production running: STOP is armed."
			} else {
				status = "Production stopped: START is armed."
			}
			drawBevelWorkingPopup(ctx, win, popupX, popupY, running, sentryOn, status)
		case bevelSentry:
			sentryOn = !sentryOn
			if sentryOn {
				status = "Sentry ON: button uses green ON face."
			} else {
				status = "Sentry OFF."
			}
			drawBevelWorkingPopup(ctx, win, popupX, popupY, running, sentryOn, status)
		case bevelAttack:
			status = "Attack clicked."
			drawBevelWorkingPopup(ctx, win, popupX, popupY, running, sentryOn, status)
		case bevelRemove:
			status = "Remove clicked."
			drawBevelWorkingPopup(ctx, win, popupX, popupY, running, sentryOn, status)
		}
	}
}

func drawBevelWorkingPopup(ctx *menus.Context, win gnw.WinID, x, y int, running, sentryOn bool, status string) {
	ctx.Sys.Fill(win, 14, 220, 395, 103, 160)
	ctx.Sys.ShadedBox(win, 14, 220, 408, 322, 148, 255)
	_ = ctx.Sys.Print(win, "Working popup", 120, 20, 224, 0)
	_ = ctx.Sys.Print(win, status, 290, 100, 246, 0)
	_ = ctx.Sys.Print(win, "Start/Stop toggles", 150, 100, 266, 0)
	_ = ctx.Sys.Print(win, "Sentry toggles ON art", 170, 260, 266, 0)
	_ = ctx.Sys.Print(win, fmt.Sprintf("Run: %s   Sentry: %s", onOff(running), onOff(sentryOn)), 290, 100, 286, 0)

	drawFace := func(row int, tag string, active bool) {
		faceTag := tag + "_OF"
		if active {
			faceTag = tag + "_ON"
		}
		if face := gamescreen.LoadPopupFace(ctx.Lib, faceTag); face != nil {
			blitDebugArt(ctx, win, x, y+row*24, face)
		} else {
			_ = ctx.Sys.Print(win, tag, 42, x, y+row*24+6, 1)
		}
	}

	startStop := "STOP"
	if !running {
		startStop = "START"
	}
	drawFace(0, startStop, false)
	drawFace(1, "SNTRY", sentryOn)
	drawFace(2, "ATTK", false)
	drawFace(3, "REMVE", false)
}

func onOff(on bool) string {
	if on {
		return "ON"
	}
	return "OFF"
}

/* // note: unused code in Go port
// loadBevelFace was a copy of the popup face loader; the bench now draws through gamescreen.LoadPopupFace, the same code as the live popup.
func loadBevelFace(ctx *menus.Context, tag string) (*gfx.Buffer, error) {
	data, err := ctx.Lib.Read(tag)
	if err != nil {
		return nil, err
	}
	img, err := sprites.DecodeSimple(data)
	if err != nil {
		return nil, err
	}
	buf := img.Buffer().Clone()
	if buf.W > 0 && buf.H > 0 {
		if key := buf.At(0, 0); maxBevelTransparentKey(key) {
			buf.KeyToZero(key)
		}
	}
	return buf, nil
}
*/

func blitDebugArt(ctx *menus.Context, win gnw.WinID, x, y int, art *gfx.Buffer) {
	buf := ctx.Sys.Buf(win)
	if buf == nil || art == nil {
		return
	}
	ctx.Sys.Lock()
	defer ctx.Sys.Unlock()
	w := min(art.W, buf.W-x)
	h := min(art.H, buf.H-y)
	if w > 0 && h > 0 {
		buf.Sub(x, y, w, h).TransCopy(art.Sub(0, 0, w, h))
	}
}

/* // note: unused code in Go port
// maxBevelTransparentKey was a copy of the popup transparent key test, used only by loadBevelFace above.
func maxBevelTransparentKey(idx uint8) bool {
	return idx == 0 || idx == 2 || idx == 3 || (idx >= 9 && idx <= 31) || (idx >= 64 && idx <= 159)
}
*/
