// selfdestruct.go : implements the confirmation dialog shown before a unit destroys itself.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++: selfdestructmenu.cpp - SelfDestructMenu
package menus

import (
	"fmt"

	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/gnw"
	"github.com/suglasp/max-go/internal/platform"
)

// selfDestructArmFrames is the number of charge up images (SLFDOPN1..N) cycled
// once the player arms the device.
const selfDestructArmFrames = 6

// selfDestructFrameMS paces the charge up cycle. The original steps each frame
// at TIMER_FPS_TO_MS(48); a touch slower here so the arming reads on screen.
const selfDestructFrameMS = 70

// Self destruct dialog button result codes.
const (
	sdArm      = 1
	sdCancel   = 2
	sdDetonate = 3
)

// RunSelfDestruct shows the SELFDSTR modal. Arming plays SLFDOPN1 through SLFDOPN6 before revealing the detonate button; Cancel or Escape returns false.
//
// C++: UnitsManager_SelfDestructMenu + UnitsManager_SelfDestructActiveMenu
// (units_manager.cpp).
func RunSelfDestruct(ctx *Context, center gfx.Rect) (detonate bool, err error) {
	// SELFDSTR is a Big image (its own palette header); the buttons/frames below
	// are simple images. The dialog overlays the live game palette, so we draw
	// its pixels without swapping the palette (as the original does).
	pic, err := ctx.loadBig("SELFDSTR")
	if err != nil {
		return false, err
	}
	bg := pic.Buffer()
	w, h := bg.W, bg.H

	// SELFDSTR is an opaque rectangular panel that draws under the active game
	// palette (its UI chrome/hazard indices are the stable upper range colours
	// shared with COLOR.PAL, so no palette swap is needed, and swapping would
	// recolour the live map showing around the small dialog).
	ox := max(center.ULX+(center.Width()+1-w)/2, 0)
	oy := max(center.ULY+(center.Height()+1-h)/2, 0)
	win := ctx.Sys.AddWindow(ox, oy, w, h, 0, gnw.WindowMoveOnTop|gnw.WindowModal)
	if win < 0 {
		return false, fmt.Errorf("menus: self-destruct window alloc failed")
	}
	defer ctx.Sys.DeleteWindow(win)

	// Pin the menu font before any caption bearing button is built (a plate
	// caption bakes with the GNW global current font at creation time).
	oldFont := ctx.Sys.Fonts().GetFont()
	defer ctx.Sys.Fonts().SetFont(oldFont)
	ctx.Sys.Fonts().SetFont(fonts.Font5)

	// Paint the dialog background.
	winBuf := ctx.Sys.Buf(win)
	ctx.Sys.Lock()
	winBuf.Sub(0, 0, w, h).Copy(bg.Sub(0, 0, w, h))
	ctx.Sys.Unlock()

	var buttons []gnw.ButtonID
	defer func() {
		for _, b := range buttons {
			ctx.Sys.DeleteButton(b)
		}
	}()

	if arm, e := ctx.plateButton(win, 89, 14, "SLFDAR_U", "SLFDAR_D", ctx.text("d5cb", "Arm"), sdArm); e == nil {
		ctx.registerButtonSfx(arm, "MBUTT0")
		buttons = append(buttons, arm)
	}
	if cancel, e := ctx.plateButton(win, 89, 46, "SLFDCN_U", "SLFDCN_D", ctx.text("ef77", "Cancel"), sdCancel); e == nil {
		ctx.registerButtonSfx(cancel, "NCANC0")
		buttons = append(buttons, cancel)
	}
	ctx.Sys.MouseShow()

	// playArming cycles SLFDOPN1..6 in the (13,11) charge up area, pumping input
	// each frame so it presents. Input is ignored during the cycle (matching the
	// original's blocking GameManager_ProcessState delay).
	playArming := func() {
		for i := 1; i <= selfDestructArmFrames; i++ {
			if img, e := ctx.loadSimple(fmt.Sprintf("SLFDOPN%d", i)); e == nil {
				ctx.drawSimpleAt(win, img, 13, 11, false)
			}
			start := platform.Ticks()
			for platform.Ticks()-start < selfDestructFrameMS {
				ctx.Sys.GetInput()
			}
		}
	}

	armed := false
	for {
		key := ctx.Sys.GetInput()
		if !armed {
			switch key {
			case sdArm, platform.KeyReturn:
				armed = true
				playArming()
				// Reveal the detonate button over the charge up area.
				if d, e := ctx.imageButton(win, 13, 11, "SLFDOK_U", "SLFDOK_D", "", 0, 0, false, sdDetonate); e == nil {
					ctx.registerButtonSfx(d, "NDONE0")
					buttons = append(buttons, d)
				}
			case sdCancel, platform.KeyEscape:
				return false, nil
			}
		} else {
			switch key {
			case sdDetonate, platform.KeyReturn:
				return true, nil
			case sdCancel, platform.KeyEscape:
				return false, nil
			}
		}
	}
}
