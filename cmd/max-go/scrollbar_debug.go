// scrollbar_debug.go : displays horizontal, vertical, and limited scroll bar samples.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package main

import (
	"fmt"

	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/gnw"
	"github.com/suglasp/max-go/internal/menus"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/sprites"
)

const (
	scrollbarClickHorizontal = 6200
	scrollbarClickVertical   = 6201
	scrollbarClickLimited    = 6202
)

// runScrollbarDebug displays the reusable GNW scroll bar without connecting it
// to a menu or list.
func runScrollbarDebug(ctx *menus.Context) error {
	rawBar, fuelBar, goldBar, vertGold, err := loadScrollbarDebugBars(ctx)
	if err != nil {
		return err
	}

	const w, h = 430, 250
	win := ctx.Sys.AddWindow((platform.ScreenWidth-w)/2, (platform.ScreenHeight-h)/2, w, h, 0, gnw.WindowMoveOnTop|gnw.WindowModal)
	if win < 0 {
		return fmt.Errorf("scrollbar debug: window alloc failed")
	}
	defer ctx.Sys.DeleteWindow(win)

	ctx.Sys.MouseShow()
	ctx.Sys.Fonts().SetFont(fonts.Font1)

	var horizontal, vertical, limited *gnw.Scrollbar
	var limitedBar *gnw.LimitedScrollbar
	drawValues := func() {
		ctx.Sys.Fill(win, 28, 62, 160, 12, 160)
		ctx.Sys.Fill(win, 300, 62, 100, 12, 160)
		ctx.Sys.Fill(win, 28, 149, 160, 12, 160)
		_ = ctx.Sys.Print(win, fmt.Sprintf("value %d", horizontal.Value()), 120, 28, 62, 0)
		_ = ctx.Sys.Print(win, fmt.Sprintf("value %d", vertical.Value()), 100, 300, 62, 0)
		_ = ctx.Sys.Print(win, fmt.Sprintf("value %d (-4..12)", limited.Value()), 160, 28, 149, 0)
	}

	drawScrollbarDebugFrame(ctx, win, w, h)
	horizontal = ctx.Sys.NewScrollbar(gnw.ScrollbarOptions{
		Window:       win,
		SliderBounds: gfx.NewRect(25, 77, 245, 89),
		Bar:          rawBar,
		IncreaseKey:  platform.KeyRight,
		DecreaseKey:  platform.KeyLeft,
		ClickKey:     scrollbarClickHorizontal,
		OnChange:     func(_, _ int) { drawValues() },
	})
	horizontal.SetFreeCapacity(100)
	horizontal.SetValue(35)
	vertical = ctx.Sys.NewScrollbar(gnw.ScrollbarOptions{
		Window:       win,
		SliderBounds: gfx.NewRect(307, 77, 325, 177),
		Bar:          vertGold,
		IncreaseKey:  platform.KeyUp,
		DecreaseKey:  platform.KeyDown,
		ClickKey:     scrollbarClickVertical,
		Orientation:  gnw.ScrollbarVertical,
		OnChange:     func(_, _ int) { drawValues() },
	})
	vertical.SetFreeCapacity(40)
	vertical.SetValue(16)
	limitedBar = ctx.Sys.NewLimitedScrollbar(gnw.ScrollbarOptions{
		Window:       win,
		SliderBounds: gfx.NewRect(25, 167, 245, 179),
		Bar:          fuelBar,
		IncreaseKey:  '+',
		DecreaseKey:  '-',
		ClickKey:     scrollbarClickLimited,
		OnChange:     func(_, _ int) { drawValues() },
	})
	limitedBar.SetZeroOffset(-10)
	limitedBar.SetFreeCapacity(30)
	limitedBar.SetXferGiveMax(12)
	limitedBar.SetXferTakeMax(-4)
	limitedBar.SetValue(0)
	limited = limitedBar.Scrollbar
	limited.SetBar(goldBar)
	limited.SetBar(fuelBar)
	drawValues()

	ctx.Sys.FlushInput()

	return runScrollbarDebugLoop(ctx, horizontal, vertical, limitedBar)
}

// loadScrollbarDebugBars loads the four bar sprites the bench fills its sliders with.
func loadScrollbarDebugBars(ctx *menus.Context) (rawBar, fuelBar, goldBar, vertGold *gfx.Buffer, err error) {
	if rawBar, err = debugSimpleBar(ctx, "BARRAW"); err != nil {
		return nil, nil, nil, nil, err
	}

	if fuelBar, err = debugSimpleBar(ctx, "BARFUEL"); err != nil {
		return nil, nil, nil, nil, err
	}

	if goldBar, err = debugSimpleBar(ctx, "BARGOLD"); err != nil {
		return nil, nil, nil, nil, err
	}

	if vertGold, err = debugSimpleBar(ctx, "VERTGOLD"); err != nil {
		return nil, nil, nil, nil, err
	}

	return rawBar, fuelBar, goldBar, vertGold, nil
}

// drawScrollbarDebugFrame paints the bench window: the frame, the captions and the
// three sunken wells the sliders run in.
func drawScrollbarDebugFrame(ctx *menus.Context, win gnw.WinID, w, h int) {
	ctx.Sys.Fill(win, 0, 0, w, h, 160)
	ctx.Sys.ShadedBox(win, 0, 0, w-1, h-1, 255, 0)
	ctx.Sys.ShadedBox(win, 8, 8, w-9, h-9, 148, 255)
	_ = ctx.Sys.Print(win, "GNW Scrollbar Widget", 240, 18, 16, 0)
	_ = ctx.Sys.Print(win, "Arrows move bars   +/- limited   Esc", 360, 18, 226, 0)
	_ = ctx.Sys.Print(win, "Raw horizontal", 150, 28, 50, 0)
	_ = ctx.Sys.Print(win, "Gold vertical", 130, 300, 50, 0)
	_ = ctx.Sys.Print(win, "Limited fuel", 130, 28, 137, 0)
	ctx.Sys.ShadedBox(win, 24, 76, 244, 90, 0, 255)
	ctx.Sys.ShadedBox(win, 306, 76, 326, 178, 0, 255)
	ctx.Sys.ShadedBox(win, 24, 166, 244, 180, 0, 255)
}

// runScrollbarDebugLoop runs the bench input loop until Escape, offering each key
// and click to the three sliders in turn.
func runScrollbarDebugLoop(ctx *menus.Context, horizontal, vertical *gnw.Scrollbar, limitedBar *gnw.LimitedScrollbar) error {
	for {
		key := ctx.Sys.GetInput()
		if key < 0 {
			continue
		}
		mouse := ctx.Plat.Mouse()
		switch {
		case key == platform.KeyEscape:
			return nil
		case horizontal.ProcessInput(key, mouse):
		case vertical.ProcessInput(key, mouse):
		case limitedBar.ProcessInput(key, mouse):
		}
	}
}

func debugSimpleBar(ctx *menus.Context, tag string) (*gfx.Buffer, error) {
	data, err := ctx.Lib.Read(tag)
	if err != nil {
		return nil, fmt.Errorf("scrollbar debug: read %s: %w", tag, err)
	}
	img, err := sprites.DecodeSimple(data)
	if err != nil {
		return nil, fmt.Errorf("scrollbar debug: decode %s: %w", tag, err)
	}
	buf := img.Buffer().Clone()
	if buf.W > 0 && buf.H > 0 {
		buf.KeyToZero(buf.At(0, 0))
	}
	return buf, nil
}
