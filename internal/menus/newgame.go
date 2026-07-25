// newgame.go : implements the scenario, campaign, training, and custom-game selector.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++: newgamemenu.cpp - NewGameMenu
package menus

import (
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/gnw"
	"github.com/suglasp/max-go/internal/platform"
)

// NewGameResult is the player's choice on the New Game submenu.
type NewGameResult int

const (
	NewGameCancel NewGameResult = iota
	NewGameTraining
	NewGameStandAlone
	NewGameCustom
	NewGameScenarios
	NewGameCampaign
	NewGameTips
)

// MultiplayerResult is the player's choice on the Multiplayer submenu.
type MultiplayerResult int

const (
	MultiCancel MultiplayerResult = iota
	MultiHost
	MultiJoin
	MultiHotSeat
	MultiScenarios
	MultiLoadHotSeat
)

// columnButton is one entry of a MAINPIC submenu button column.
type columnButton struct {
	big   bool
	x, y  int
	label string
	key   int
	sfx   string
}

// NewGameMenu shows the New Game submenu and returns the selection. The Tips toggle remains unavailable because the help system does not expose it.
//
// C++: menu_new_game_menu_loop (menu.cpp)
func NewGameMenu(ctx *Context) (NewGameResult, error) {
	defs := []columnButton{
		{true, 385, 175, ctx.text("182b", "Training Missions"), 'T', "MBUTT0"},
		{true, 385, 210, ctx.text("02c9", "Stand Alone Missions"), 'S', "MBUTT0"},
		{true, 385, 245, ctx.text("3d29", "Custom Game"), 'U', "MBUTT0"},
		{true, 385, 280, ctx.text("884f", "Custom Scenarios"), 'M', "MBUTT0"},
		{true, 385, 315, ctx.text("878a", "Campaign Game"), 'A', "MBUTT0"},
		{true, 385, 350, ctx.text("3838", "Tips"), 'I', "MBUTT0"},
		{false, 435, 420, ctx.text("7eec", "Cancel"), 'C', "NCANC0"},
	}
	key, err := runColumnMenu(ctx, ctx.text("04b5", "New Game Menu"), defs)
	if err != nil {
		return NewGameCancel, err
	}
	switch key {
	case 'T':
		return NewGameTraining, nil
	case 'S':
		return NewGameStandAlone, nil
	case 'U':
		return NewGameCustom, nil
	case 'M':
		return NewGameScenarios, nil
	case 'A':
		return NewGameCampaign, nil
	case 'I':
		return NewGameTips, nil
	default:
		return NewGameCancel, nil
	}
}

// MultiplayerMenu shows the Multiplayer submenu. Only Hot Seat is available because networking is out of scope.
//
// C++: menu_multiplayer_menu_loop (menu.cpp)
func MultiplayerMenu(ctx *Context) (MultiplayerResult, error) {
	defs := []columnButton{
		{true, 385, 175, ctx.text("c7a9", "Host Network Game"), 'H', "MBUTT0"},
		{true, 385, 210, ctx.text("7064", "Join Network Game"), 'J', "MBUTT0"},
		{true, 385, 280, ctx.text("f731", "Hot Seat Game"), 'O', "MBUTT0"},
		{true, 385, 315, ctx.text("bf00", "Hot Seat Scenarios"), 'T', "MBUTT0"},
		{true, 385, 350, ctx.text("847e", "Load Hot Seat Game"), 'L', "MBUTT0"},
		{false, 435, 420, ctx.text("e13e", "Cancel"), 'C', "NCANC0"},
	}
	key, err := runColumnMenu(ctx, ctx.text("f2b3", "Multiplayer Menu"), defs)
	if err != nil {
		return MultiCancel, err
	}
	switch key {
	case 'H':
		return MultiHost, nil
	case 'J':
		return MultiJoin, nil
	case 'O':
		return MultiHotSeat, nil
	case 'T':
		return MultiScenarios, nil
	case 'L':
		return MultiLoadHotSeat, nil
	default:
		return MultiCancel, nil
	}
}

// runColumnMenu draws a MAINPIC screen with a title, the random unit
// portrait and a column of plate buttons, then returns the key code of
// the chosen entry (KeyEscape for cancel).
//
// C++: the shared shape of menu_new_game_menu_loop /
// menu_multiplayer_menu_loop (menu.cpp)
func runColumnMenu(ctx *Context, title string, defs []columnButton) (int, error) {
	pic, err := ctx.loadBig("MAINPIC")
	if err != nil {
		return platform.KeyEscape, err
	}
	ctx.drawBigAt(mainWindow, pic, 0, 0)
	ctx.Plat.SetPalette(&pic.Palette)
	ctx.drawMenuTitle(title)
	ctx.setHandCursor()

	ctx.Sys.Fonts().SetFont(fonts.Font1)

	var buttons []gnw.ButtonID
	defer func() {
		for _, b := range buttons {
			ctx.Sys.DeleteButton(b)
		}
	}()

	for _, d := range defs {
		upTag, downTag := "BLANK_UP", "BLANK_DN"
		if !d.big {
			upTag, downTag = "SBLNK_UP", "SBLNK_DN"
		}
		bid, err := ctx.plateButton(mainWindow, d.x, d.y, upTag, downTag, d.label, d.key)
		if err != nil {
			return platform.KeyEscape, err
		}
		ctx.registerButtonSfx(bid, d.sfx)
		buttons = append(buttons, bid)
	}

	drawPortrait(ctx)
	portraitBtn := ctx.Sys.AddButton(mainWindow, 16, 182, 300, 240,
		gnw.NoValue, gnw.NoValue, gnw.NoValue, platform.KeyUp, nil, nil, nil, 0)
	ctx.registerButtonSfx(portraitBtn, "KCARG0")
	buttons = append(buttons, portraitBtn)

	ctx.Sys.MouseShow()

	for {
		key := toUpper(ctx.Sys.GetInput())
		switch key {
		case -1:
			continue
		case platform.KeyEscape:
			return platform.KeyEscape, nil
		case platform.KeyUp:
			drawPortrait(ctx)
			continue
		}
		for _, d := range defs {
			if key == d.key {
				return key, nil
			}
		}
	}
}
