// listbox_debug.go : exercises list box clicks, double clicks, and scrolling.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package main

/* // note: unused code in Go port
// the -emulate listbox bench of the commented out gnw.ListBox.

import (
	"fmt"

	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/gnw"
	"github.com/suglasp/max-go/internal/menus"
	"github.com/suglasp/max-go/internal/platform"
)

const (
	listboxClick          = 6300
	listboxDoubleClick    = 6301
	listboxScrollbarClick = 6302
)

// runListboxDebug is an interactive visual bench for the reusable GNW listbox
// widget. It keeps the widget detached from real menus while making selection,
// keyboard navigation, scrollbar sync, and double click confirm visible.
func runListboxDebug(ctx *menus.Context) error {
	bar, err := debugSimpleBar(ctx, "VERTGOLD")
	if err != nil {
		return err
	}

	const w, h = 360, 260
	win := ctx.Sys.AddWindow((platform.ScreenWidth-w)/2, (platform.ScreenHeight-h)/2, w, h, 0, gnw.WindowMoveOnTop|gnw.WindowModal)
	if win < 0 {
		return fmt.Errorf("listbox debug: window alloc failed")
	}
	defer ctx.Sys.DeleteWindow(win)

	oldFont := ctx.Sys.Fonts().GetFont()
	ctx.Sys.Fonts().SetFont(fonts.Font1)
	defer ctx.Sys.Fonts().SetFont(oldFont)

	ctx.Sys.MouseShow()
	drawListboxDebugChrome(ctx, win, w, h)

	entries := listboxDebugEntries

	listBG := gfx.New(261, 149)
	listBG.Fill(16)

	var lb *gnw.ListBox
	var lastConfirmed = -1
	status := func() {
		ctx.Sys.Fill(win, 24, 218, 310, 14, 160)
		text := "Selected: none"
		if lb != nil && lb.Selected() >= 0 {
			text = fmt.Sprintf("Selected: %s", entries[lb.Selected()].Text)
		}
		if lastConfirmed >= 0 {
			text += fmt.Sprintf("   confirmed: %s", entries[lastConfirmed].Text)
		}
		_ = ctx.Sys.Print(win, text, 310, 24, 218, 0)
	}

	scrollbar := ctx.Sys.NewScrollbar(gnw.ScrollbarOptions{
		Window:       win,
		SliderBounds: gfx.NewRect(301, 59, 317, 205),
		Bar:          bar,
		ClickKey:     listboxScrollbarClick,
		Orientation:  gnw.ScrollbarVertical,
		OnChange: func(_, newValue int) {
			if lb != nil {
				lb.SetScrollOffset(newValue)
				status()
			}
		},
	})
	lb = ctx.Sys.NewListBox(gnw.ListBoxOptions{
		Window:         win,
		Bounds:         gfx.NewRect(25, 59, 283, 205),
		Entries:        entries,
		RowHeight:      12,
		Background:     listBG,
		Scrollbar:      scrollbar,
		ClickKey:       listboxClick,
		DoubleClickKey: listboxDoubleClick,
		TextColor:      0xFF,
		SelectedColor:  0x38,
		OnSelect:       func(_, _ int) { status() },
		OnConfirm: func(index int) {
			lastConfirmed = index
			dbg.Printf("listbox confirmed: %s", entries[index].Text)
			status()
		},
	})
	status()

	ctx.Sys.FlushInput()

	return runListboxDebugLoop(ctx, win, lb, scrollbar, status)
}

// listboxDebugEntries is the sample unit roster the listbox bench lists.
var listboxDebugEntries = []gnw.ListBoxEntry{
	{Text: "Constructor"}, {Text: "Engineer"}, {Text: "Surveyor"}, {Text: "Scout"},
	{Text: "Scanner"}, {Text: "Tank"}, {Text: "Assault Gun"}, {Text: "Rocket Launcher"},
	{Text: "Missile Crawler"}, {Text: "Mine Layer"}, {Text: "Repair Unit"}, {Text: "Supply Truck"},
	{Text: "Fuel Truck"}, {Text: "Mobile A-A Gun"}, {Text: "Gold Truck"}, {Text: "Cargo Truck"},
	{Text: "Fighter"}, {Text: "Bomber"}, {Text: "Air Transport"}, {Text: "Missile Boat"},
}

// drawListboxDebugChrome paints the bench window: the frame, the two captions and
// the sunken wells the list and its scrollbar sit in.
func drawListboxDebugChrome(ctx *menus.Context, win gnw.WinID, w, h int) {
	ctx.Sys.Fill(win, 0, 0, w, h, 160)
	ctx.Sys.ShadedBox(win, 0, 0, w-1, h-1, 255, 0)
	ctx.Sys.ShadedBox(win, 8, 8, w-9, h-9, 148, 255)
	_ = ctx.Sys.Print(win, "GNW ListBox Widget", 240, 20, 18, 0)
	_ = ctx.Sys.Print(win, "Click, arrows, PgUp/PgDn, Enter, Esc", 320, 20, 34, 0)
	ctx.Sys.ShadedBox(win, 24, 58, 284, 206, 0, 255)
	ctx.Sys.ShadedBox(win, 300, 58, 318, 206, 0, 255)
}

// runListboxDebugLoop runs the bench input loop until Escape, routing clicks to the
// scrollbar or the list and pairing two quick clicks on one row into a double click.
func runListboxDebugLoop(ctx *menus.Context, win gnw.WinID, lb *gnw.ListBox, scrollbar *gnw.Scrollbar, status func()) error {
	type clickMark struct {
		index int
		stamp uint64
		valid bool
	}
	var lastClick clickMark
	for {
		key := ctx.Sys.GetInput()
		if key < 0 {
			continue
		}
		mouse := ctx.Plat.Mouse()
		switch {
		case key == platform.KeyEscape:
			return nil
		case scrollbar.ProcessInput(key, mouse):
			status()
		case key == listboxClick:
			if r, ok := ctx.Sys.WinRect(win); ok {
				mx, my := mouse.X-r.ULX, mouse.Y-r.ULY
				if index, ok := lb.RowAt(mx, my); ok {
					now := platform.Ticks()
					if lastClick.valid && lastClick.index == index && now >= lastClick.stamp && now-lastClick.stamp <= 500 {
						lb.ProcessKey(listboxDoubleClick, mx, my)
						lastClick.valid = false
					} else {
						lb.ProcessKey(listboxClick, mx, my)
						lastClick.index = index
						lastClick.stamp = now
						lastClick.valid = true
					}
				}
			}
		case lb.ProcessInput(key, mouse):
			status()
		}
	}
}
*/
