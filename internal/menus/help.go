// help.go : implements the in-game context help: point at something and it tells you what it is.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : helpmenu.cpp - HelpMenu_Menu
package menus

import (
	"github.com/suglasp/max-go/internal/cursors"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/gnw"
	"github.com/suglasp/max-go/internal/help"
	"github.com/suglasp/max-go/internal/platform"
)

// Help enters context-help mode. The pointer becomes a question mark and the next click looks up a retail HELP_ENG rectangle for the active 640x480 screen.

// helpClickValue is the button value of the invisible full screen region that catches the
// help mode click. It sits above 2000 so it cannot collide with a screen's own buttons.
const helpClickValue = 2900

// Help runs help mode for a screen section (the section names are the original's, e.g.
// "REPORTS_SETUP"). It returns when the player leaves help mode with ESC, Return or a
// right click.
//
// C++ ported : helpmenu.cpp:315 - the HelpMenu_Menu outer loop (help cursor, modal input
// swallow, click -> HelpMenu::Run, right click -> leave)
func Help(ctx *Context, section string) {
	db := ctx.helpDB()

	if db.Sections() == 0 {
		// Report a missing database instead of entering a mode with no answers.
		Message(ctx, "No help database was found in the game data.")

		return
	}

	// The pointer becomes the question mark for as long as help mode lasts.
	restore := ctx.setHelpCursor()
	defer restore()

	// One invisible region over the whole screen: it is what turns a click anywhere into
	// an input event we can hit test.
	click := ctx.Sys.AddButton(mainWindow, 0, 0, platform.ScreenWidth, platform.ScreenHeight,
		gnw.NoValue, gnw.NoValue, gnw.NoValue, helpClickValue, nil, nil, nil, 0)

	if click >= 0 {
		defer ctx.Sys.DeleteButton(click)
	}

	ctx.Sys.MouseShow()

	for {
		// A right click leaves help mode, wherever the pointer is.
		if m := ctx.Plat.Mouse(); m.Buttons&platform.MouseReleaseRight != 0 {
			return
		}

		key := ctx.Sys.GetInput()

		switch {
		case key == platform.KeyEscape || key == platform.KeyReturn:
			return

		case key == helpClickValue:
			m := ctx.Plat.Mouse()
			x, y := ctx.helpPoint(m.X, m.Y)

			text, ok := db.Lookup(section, x, y)
			dbg.Printf("help: %s at (%d,%d) -> %v", section, x, y, ok)

			if !ok {
				continue // nothing to say about that spot; stay in help mode
			}

			// The help window is the same HELPFRAM modal the message box uses, so the text
			// wraps, scrolls with the pointer and closes on Done.
			Message(ctx, text)
			ctx.setHelpCursor() // the modal restored the hand pointer; back to the question mark
		}
	}
}

// helpDB parses the help database once and retains it for the session.
func (ctx *Context) helpDB() *help.Database {
	if ctx.help != nil {
		return ctx.help
	}

	data, err := ctx.Lib.Read(help.Tag)
	if err != nil {
		dbg.Printf("help: %s: %v", help.Tag, err)

		ctx.help = help.Parse(nil) // an empty database, so we do not retry the read every time

		return ctx.help
	}

	ctx.help = help.Parse(data)
	dbg.Printf("help: %s parsed, %d screens", help.Tag, ctx.help.Sections())

	return ctx.help
}

// helpPoint maps a live pointer position into the 640x480 space the help rectangles are
// written in. A menu screen already IS that space, so the caller only installs a fixup
// for the in game screen, whose widgets move in widescreen.
//
// C++ ported : helpmenu.cpp:412 - HelpMenu_ScaleCursor
func (ctx *Context) helpPoint(x, y int) (int, int) {
	if ctx.HelpFixup == nil {
		return x, y
	}

	return ctx.HelpFixup(x, y)
}

// setHelpCursor switches the pointer to the question mark and returns the function that
// puts the hand back.
func (ctx *Context) setHelpCursor() func() {
	restore := func() { ctx.setCursor(cursors.Hand) }

	ctx.setCursor(cursors.Help)

	return restore
}

// setCursor installs one of the catalog's cursors, loading the catalog on first use.
func (ctx *Context) setCursor(id cursors.ID) {
	if ctx.cursorCat == nil {
		cat, err := cursors.LoadCatalog(ctx.Lib)

		if err != nil {
			dbg.Printf("help: cursors: %v", err)

			return
		}

		ctx.cursorCat = cat
	}

	if c := ctx.cursorCat.Get(id); c != nil {
		ctx.Sys.SetCursor(c.Frame(0), c.HotX, c.HotY, c.Transparent)
	}
}
