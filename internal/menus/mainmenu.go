// mainmenu.go : implements the top-level New Game, Load, Options, Credits, and Quit menu.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++: mainmenu.cpp - MainMenu
package menus

import (
	"time"

	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/gnw"
	"github.com/suglasp/max-go/internal/platform"
)

// MainMenuResult is the player's choice on the main menu.
type MainMenuResult int

const (
	MainExit MainMenuResult = iota
	MainNewGame
	MainLoadGame
	MainMultiplayer
	MainSetup
	MainIntro
	MainCredits
	MainDemo // the attract demo: no button, returned after attractTimeout of idle
)

// attractTimeout is how long the main menu sits untouched (no key, no mouse
// movement) before MainMenu returns MainDemo and the caller plays a demo.
//
// C++: ATTRACT_MODE_TIMEOUT (menu.cpp)
const attractTimeout = 60 * time.Second

// menuMusicTag is the main-menu background track. PlayMusic is idempotent by tag, so starting it at boot and menu entry does not restart it.
const menuMusicTag = "MAIN_MSC"

// String returns the menu label of the result (the debug log prints the main menu choice with it).
func (r MainMenuResult) String() string {
	switch r {
	case MainNewGame:
		return "New Game"
	case MainLoadGame:
		return "Load Game"
	case MainMultiplayer:
		return "Multiplayer Game"
	case MainSetup:
		return "Setup"
	case MainIntro:
		return "Introduction"
	case MainCredits:
		return "Credits"
	case MainDemo:
		return "Demo"
	default:
		return "Exit"
	}
}

// menuPortraits are the unit portrait resources eligible for the
// random main menu portrait: P_TRANSP through P_TRANSP+67, minus the
// ones the original loop rejects (P_SHIELD, P_LIFESP, P_RECCTR,
// P_MASTER, P_JUGGER and the alien portraits).
//
// C++: menu_draw_menu_portrait (menu.cpp), resourcetable.hpp
var menuPortraits = []string{
	"P_TRANSP", "P_POWSTN", "P_POWGEN", "P_BARRCK", "P_RADAR", "P_SMSTOR",
	"P_SMFUEL", "P_SMVLT", "P_DEPOT", "P_HANGAR", "P_DOCK", "P_CONNEC",
	"P_ROAD", "P_LANDPD", "P_SHIPYD", "P_LGHTPL", "P_HVYPLT", "P_AIRPLT",
	"P_HABITA", "P_RESEAR", "P_GREENH", "P_TRNHLL", "P_WATER", "P_GUNTUR",
	"P_FXAA", "P_ARTYTR", "P_FXROCK", "P_BLOCK", "P_BRIDGE", "P_MINING",
	"P_LANDMN", "P_SEAMIN", "P_CONTRC", "P_SCOUT", "P_TANK", "P_ARTY",
	"P_ROCKET", "P_MISSIL", "P_AA", "P_MNELAY", "P_SURVEY", "P_SCANNR",
	"P_SPLYTR", "P_GOLDTR", "P_ENGINR", "P_BULLDZ", "P_REPAIR", "P_FUELTR",
	"P_COLNST", "P_COMMAN", "P_INFANT", "P_ESCORT", "P_CORVET", "P_GUNBT",
	"P_SUB", "P_SEATRN", "P_MSLCR", "P_SEAMNL", "P_CARGOS", "P_FIGHTR",
	"P_BOMBER", "P_AIRTRN", "P_AWAC",
}

// exitSoundHold lets the Exit button's NDONE0 blip finish before the audio device closes. Escape exits without waiting.
const exitSoundHold = 100 * time.Millisecond

// mainMenuButton describes one entry of the menu button column.
//
// C++: main_menu_buttons (menu.cpp)
type mainMenuButton struct {
	big    bool
	x, y   int
	label  string
	key    int    // input code: clicking emits it, typing it works too
	sfx    string // press sound resource
	result MainMenuResult
}

// MainMenu shows the main menu and returns the player's selection.
//
// C++: main_menu (menu.cpp)
func MainMenu(ctx *Context) (MainMenuResult, error) {
	defs := mainMenuButtons(ctx)

	// Background art carries the menu palette.
	pic, err := ctx.loadBig("MAINPIC")
	if err != nil {
		return MainExit, err
	}
	ctx.drawBigAt(pic, 0, 0)
	ctx.Plat.SetPalette(&pic.Palette)
	ctx.drawMenuTitle(ctx.text("35f9", "Main Menu"))
	ctx.setHandCursor()

	buttons, err := buildMainMenuWidgets(ctx, defs)
	defer func() {
		for _, b := range buttons {
			ctx.Sys.DeleteButton(b)
		}
	}()
	if err != nil {
		return MainExit, err
	}

	drawCopyright(ctx)
	ctx.Sys.MouseShow()

	if ctx.Audio != nil {
		ctx.Audio.PlayMusic(menuMusicTag, true)
	}

	// Attract mode: a full minute untouched starts a demo. A key resets the
	// countdown; on the idle frames mouse movement or a held button does.
	// C++: main_menu (menu.cpp) - the ATTRACT_MODE_TIMEOUT check.
	deadline := time.Now().Add(attractTimeout)
	lastMouse := ctx.Plat.Mouse()

	for {
		key := toUpper(ctx.Sys.GetInput())

		if key != -1 {
			deadline = time.Now().Add(attractTimeout)
		}

		switch key {
		case -1:
			if m := ctx.Plat.Mouse(); m.X != lastMouse.X || m.Y != lastMouse.Y || m.Buttons != 0 {
				lastMouse = m
				deadline = time.Now().Add(attractTimeout)
			} else if time.Now().After(deadline) {
				dbg.Printf("main menu: idle for %v, starting the attract demo", attractTimeout)

				return MainDemo, nil
			}

			continue
		case platform.KeyEscape:
			return MainExit, nil
		case platform.KeyUp:
			drawPortrait(ctx)
			continue
		}
		for _, d := range defs {
			if key == d.key {
				dbg.Printf("main menu: %s", d.result)

				if d.result == MainExit {
					time.Sleep(exitSoundHold) // let the Exit blip finish before the caller quits
				}

				return d.result, nil
			}
		}
	}
}

// mainMenuButtons is the menu button column. The labels come from
// lang_english.ini, with English fallbacks; the sfx tags match the
// MENU_BUTTON_DEF table.
//
// C++: main_menu_buttons (menu.cpp)
func mainMenuButtons(ctx *Context) []mainMenuButton {
	return []mainMenuButton{
		{true, 385, 175, ctx.text("28f9", "New Game"), 'N', "MBUTT0", MainNewGame},
		{true, 385, 210, ctx.text("dc41", "Load Game"), 'L', "MBUTT0", MainLoadGame},
		{true, 385, 245, ctx.text("8339", "Multiplayer Game"), 'M', "MBUTT0", MainMultiplayer},
		{true, 385, 280, ctx.text("7013", "Setup"), 'S', "MBUTT0", MainSetup},
		{true, 385, 315, ctx.text("b605", "Introduction"), 'I', "MBUTT0", MainIntro},
		{true, 385, 350, ctx.text("2a6b", "Credits"), 'C', "MBUTT0", MainCredits},
		{false, 435, 420, ctx.text("d17d", "Exit"), 'E', "NDONE0", MainExit},
	}
}

// buildMainMenuWidgets creates the menu button column and the portrait region,
// and returns their ids for the caller to delete.
func buildMainMenuWidgets(ctx *Context, defs []mainMenuButton) ([]gnw.ButtonID, error) {
	ctx.Sys.Fonts().SetFont(fonts.Font1)

	var buttons []gnw.ButtonID

	for _, d := range defs {
		upTag, downTag := "BLANK_UP", "BLANK_DN"
		if !d.big {
			upTag, downTag = "SBLNK_UP", "SBLNK_DN"
		}

		bid, err := ctx.plateButton(mainWindow, d.x, d.y, upTag, downTag, d.label, d.key)

		if err != nil {
			return buttons, err
		}
		ctx.registerButtonSfx(bid, d.sfx)
		buttons = append(buttons, bid)
	}

	// The portrait region acts as a button redrawing a new portrait.
	drawPortrait(ctx)
	portraitBtn := ctx.Sys.AddButton(mainWindow, 16, 182, 300, 240, gnw.NoValue, gnw.NoValue, gnw.NoValue, platform.KeyUp, nil, nil, nil, 0)
	ctx.registerButtonSfx(portraitBtn, "KCARG0")
	buttons = append(buttons, portraitBtn)

	return buttons, nil
}

// drawPortrait paints the framed random unit portrait.
//
// C++: menu_draw_menu_portrait (menu.cpp)
func drawPortrait(ctx *Context) {
	if l, err := ctx.loadSimple("BGRSCRNL"); err == nil {
		ctx.drawSimpleAt(mainWindow, l, 6, 173, false)
	}
	if r, err := ctx.loadSimple("BGRSCRNR"); err == nil {
		ctx.drawSimpleAt(mainWindow, r, 166, 173, false)
	}
	tag := menuPortraits[ctx.Rand.Intn(len(menuPortraits))]
	img, err := ctx.loadBig(tag)
	if err != nil {
		dbg.Printf("portrait %s: %v", tag, err)
		return
	}
	ctx.drawBigAt(img, 16, 182)
}

// drawCopyright prints the bottom credit line.
//
// C++: draw_copyright_label (menu.cpp)
func drawCopyright(ctx *Context) {
	ctx.Sys.Fonts().SetFont(fonts.Font5)
	ctx.Sys.Print(mainWindow, "Copyright 1996 Interplay Productions. v1.04 (max-go Golang port)", 620, 10, 469, 0)
	ctx.Sys.Fonts().SetFont(fonts.Font1)
}
