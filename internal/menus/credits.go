// credits.go : implements the scrolling credits screen and palette cycling.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++: creditmenu.cpp - CreditsMenu
package menus

import (
	"github.com/suglasp/max-go/internal/credits"
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/platform"
)

// Credits text colours: titles render in COLOR (0xFF), names in 0x04, each
// OR'd with the font outline; the colour byte for each line the original stores in
// menu_credits_lines[].
//
// C++: CREDITS_TITLE/CREDITS_TEXT colour bytes (menu.cpp)
const (
	creditsTitleColor = 0xFF
	creditsTextColor  = 0x04
)

// RunCredits scrolls the development credits up over the CREDITS image until
// the list finishes or the player presses a key or clicks. It drives the pure
// internal/credits component; the screen loop, input and cursor are here.
//
// C++: menu_credits_menu_loop (menu.cpp)
func RunCredits(ctx *Context) error {
	bg, err := ctx.loadBig("CREDITS")

	if err != nil {
		return err
	}

	ctx.Plat.SetPalette(&bg.Palette)

	resolve := func(key string) string { return ctx.text(key, key) }
	font := ctx.Sys.Fonts().Get(fonts.Font1)
	scr := credits.New(bg.Buffer(), font, credits.DefaultLines(resolve), creditsTitleColor, creditsTextColor)

	ctx.Sys.MouseHide()
	defer ctx.Sys.MouseShow()

	buf := ctx.Sys.Buf(mainWindow)
	last := platform.Ticks()
	armed := false // ignore the click that opened the screen until the button is released

	for {
		// Any key ends the credits.
		if key := ctx.Sys.GetInput(); key != -1 {
			break
		}

		buttons := ctx.Plat.Mouse().Buttons
		if buttons == 0 {
			armed = true
		} else if armed && buttons&(platform.MousePressLeft|platform.MousePressRight) != 0 {
			break
		}

		now := platform.Ticks()
		running := scr.Advance(now - last)
		last = now

		ctx.Sys.Lock()
		scr.Draw(buf)
		ctx.Sys.Unlock()

		if !running {
			break
		}
	}

	// Drain the click that closed the screen (the mouse button is still held
	// when we break) so its release does not immediately trigger the main
	// menu Credits button again as it is rebuilt under the cursor.
	for ctx.Plat.Mouse().Buttons != 0 {
		ctx.Sys.GetInput()
	}
	
	return nil
}
