// chooseplayer.go : implements pregame team-type and clan assignment.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++: chooseplayermenu.cpp - ChoosePlayerMenu
package menus

import (
	"fmt"

	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/gnw"
	"github.com/suglasp/max-go/internal/platform"
)

// Input codes of the choose player screen: 1000+index with the C++
// control table order (12 type buttons, 4 clan buttons, cancel, help,
// done).
const (
	cpTypeBase = 1000      // 0..11: column*4 + team
	cpClanBase = 1000 + 12 // 12..15: per team clan button
	cpCancel   = 1000 + 16
	cpHelp     = 1000 + 17
	cpDone     = 1000 + 18
)

// cpColumn is one Choose Players type column: its x, the up image tag (the
// down image swaps the trailing U for a D), the press sound and the team type
// the column assigns.
type cpColumn struct {
	x     int
	tag   string
	sfx   string
	ttype game.TeamType
}

// The three type columns in button order.
var cpColumns = [3]cpColumn{
	{175, "CH_HUM_U", "NHUMN0", game.TeamPlayer},
	{285, "CH_CMP_U", "NCOMP0", game.TeamComputer},
	{394, "CH_NON_U", "NNONE0", game.TeamNone},
}

// cpRowY returns the y position of a team's button row.
func cpRowY(team int) int {
	return 67 + 92*team
}

// ChoosePlayer assigns team types and clans in params. Single-player games require one human; hot-seat games require at least two.
//
// C++: ChoosePlayerMenu (chooseplayermenu.cpp) +
// menu_choose_player_menu_loop (menu.cpp)
func ChoosePlayer(ctx *Context, params *game.StartParams, isSinglePlayer bool, playerName string) (bool, error) {
	// Forced starting assignment.
	// C++: menu_choose_player_menu_loop head
	params.Teams[0].Type = game.TeamPlayer
	if isSinglePlayer {
		params.Teams[1].Type = game.TeamComputer
	} else {
		params.Teams[1].Type = game.TeamPlayer
	}
	params.Teams[2].Type = game.TeamNone
	params.Teams[3].Type = game.TeamNone

	for {
		action, err := choosePlayerScreen(ctx, params, isSinglePlayer)
		if err != nil {
			return false, err
		}
		switch {
		case action == cpCancel:
			return false, nil
		case action == cpDone:
			assignTeamNames(ctx, params, isSinglePlayer, playerName)
			return true, nil
		case action >= cpClanBase && action < cpClanBase+game.TeamCount:
			// C++: EventSelectClan: Deinit, clan menu, Init
			team := action - cpClanBase
			clan, err := ClanSelect(ctx, team, params.Teams[team].Clan)
			if err != nil {
				return false, err
			}
			params.Teams[team].Clan = clan
		}
	}
}

// choosePlayerMenu holds the assignment being edited and button handles rebuilt after type changes.
//
// C++: ChoosePlayerMenu (chooseplayermenu.hpp)
type choosePlayerMenu struct {
	ctx            *Context
	params         *game.StartParams
	isSinglePlayer bool

	buttons  []gnw.ButtonID
	typeBtns [12]gnw.ButtonID // indexed column*4 + team, like the C++ buttons array
	clanBtns [game.TeamCount]gnw.ButtonID
}

// choosePlayerScreen builds the screen and runs its input loop until
// an action that needs a full screen rebuild (clan menu) or ends the
// menu. It returns the action code.
func choosePlayerScreen(ctx *Context, params *game.StartParams, isSinglePlayer bool) (int, error) {
	pic, err := ctx.loadBig("CHOSPLYR")
	if err != nil {
		return cpCancel, err
	}
	ctx.drawBigAt(mainWindow, pic, 0, 0)
	ctx.Plat.SetPalette(&pic.Palette)

	m := &choosePlayerMenu{ctx: ctx, params: params, isSinglePlayer: isSinglePlayer}
	defer func() {
		for _, b := range m.buttons {
			ctx.Sys.DeleteButton(b)
		}
	}()
	if err := m.buildWidgets(); err != nil {
		return cpCancel, err
	}

	drawChoosePlayerTitles(ctx, isSinglePlayer)
	ctx.Sys.MouseShow()

	for {
		key := ctx.Sys.GetInput()
		if key == -1 {
			continue
		}
		if action, done, err := m.handleKey(key); done {
			return action, err
		}
	}
}

// buildWidgets creates the 12 sticky type buttons with their radio groups, the
// four clan buttons and the bottom plates, keeping every id on the struct.
//
// C++: ChoosePlayerMenu::ButtonInit (chooseplayermenu.cpp)
func (m *choosePlayerMenu) buildWidgets() error {
	ctx, params := m.ctx, m.params

	for col, c := range cpColumns {
		downTag := c.tag[:len(c.tag)-1] + "D"
		for team := 0; team < game.TeamCount; team++ {
			idx := col*4 + team
			bid, err := ctx.imageButton(mainWindow, c.x, cpRowY(team), c.tag, downTag,
				"", 0, 0, true, cpTypeBase+idx)
			if err != nil {
				return err
			}
			ctx.registerButtonSfx(bid, c.sfx)
			m.buttons = append(m.buttons, bid)
			m.typeBtns[idx] = bid
		}
	}

	// Radio groups for each team across the three columns, then set the
	// rest state from the current assignment.
	for team := 0; team < game.TeamCount; team++ {
		group := []gnw.ButtonID{m.typeBtns[team], m.typeBtns[4+team], m.typeBtns[8+team]}
		ctx.Sys.GroupRadioButtons(group, nil)
		m.setTypeRestState(team, true)
	}

	// Clan buttons with the team's clan logo; teams that are not human get the
	// pressed plate art and are disabled.
	for team := 0; team < game.TeamCount; team++ {
		bid, err := makeClanButton(ctx, params, team)
		if err != nil {
			return err
		}
		ctx.registerButtonSfx(bid, "NCLAN0")
		m.buttons = append(m.buttons, bid)
		m.clanBtns[team] = bid
	}

	// Bottom plate buttons.
	ctx.Sys.Fonts().SetFont(fonts.Font1)
	if err := ctx.addBottomPlates([]bottomPlate{
		{354, "MNUBTN4U", ctx.text("4042", "Cancel"), cpCancel, "NCANC0"},
		{465, "MNUBTN5U", ctx.text("9605", "?"), cpHelp, "NHELP0"},
		{514, "MNUBTN6U", ctx.text("ef0a", "Done"), cpDone, "NDONE0"},
	}, &m.buttons); err != nil {
		return err
	}

	return nil
}

// handleKey answers one input code. It reports the action to hand back and
// whether the screen is finished with it; a clan pick ends the screen so the
// caller can run the clan menu and rebuild.
//
// C++: menu_choose_player_menu_loop - the input switch (menu.cpp)
func (m *choosePlayerMenu) handleKey(key int) (int, bool, error) {
	ctx, params, isSinglePlayer := m.ctx, m.params, m.isSinglePlayer

	switch {
	case key >= cpTypeBase && key < cpTypeBase+12:
		idx := key - cpTypeBase
		col, team := idx/4, idx%4
		newType := cpColumns[col].ttype

		// In a custom (single player) game only one human is
		// allowed: the previous human team takes over this team's
		// old type. C++: EventSelectHuman
		if newType == game.TeamPlayer && isSinglePlayer {
			for other := 0; other < game.TeamCount; other++ {
				if other != team && params.Teams[other].Type == game.TeamPlayer {
					// Release the displaced team's Human button BEFORE its type changes,
					// then press the new one after. Both halves are required.
					m.setTypeRestState(other, false)
					params.Teams[other].Type = params.Teams[team].Type
					m.setTypeRestState(other, true)
					// The displaced computer team needs a disabled clan button.
					if err := m.rebuildClanButton(other); err != nil {
						return cpCancel, true, err
					}
					break
				}
			}
		}
		params.Teams[team].Type = newType

		// C++: UpdateButtons: refresh the team's clan button.
		if err := m.rebuildClanButton(team); err != nil {
			return cpCancel, true, err
		}

	case key >= cpClanBase && key < cpClanBase+game.TeamCount:
		if params.Teams[key-cpClanBase].Type == game.TeamPlayer {
			return key, true, nil // run the clan menu, then rebuild
		}

	case key == cpDone, key == platform.KeyReturn:
		if msg := validatePlayers(ctx, params, isSinglePlayer); msg != "" {
			messageBox(ctx, msg)
			break
		}
		return cpDone, true, nil

	case key == cpCancel, key == platform.KeyEscape:
		return cpCancel, true, nil

	case key == cpHelp, key == '?':
		// The same screen serves a single player game and a hot seat one, and the help
		// database has a section for each. C++: chooseplayermenu.cpp:116.
		section := "HOT_SEAT_SETUP"
		if isSinglePlayer {
			section = "NEW_GAME_SETUP"
		}

		Help(ctx, section)
	}

	return 0, false, nil
}

// rebuildClanButton replaces a team's clan button after its type changes,
// patching the new id into m.buttons in place so the entry's delete defer
// (which ranges over m.buttons) still tears it down.
//
// C++: ChoosePlayerMenu::UpdateButtons.
func (m *choosePlayerMenu) rebuildClanButton(team int) error {
	ctx, params := m.ctx, m.params

	ctx.Sys.DeleteButton(m.clanBtns[team])
	bid, err := makeClanButton(ctx, params, team)
	if err != nil {
		return err
	}
	ctx.registerButtonSfx(bid, "NCLAN0")
	for i, b := range m.buttons {
		if b == m.clanBtns[team] {
			m.buttons[i] = bid
		}
	}
	m.clanBtns[team] = bid
	return nil
}

// setTypeRestState forces the button of the team's CURRENT type up or down. A caller that
// changes a team's type calls it twice: false before the change to release the old column,
// true after it to press the new one. Both halves are needed because SetRestState moves only
// the button it is given, so releasing is never implied. Skip the release and the row keeps
// two buttons lit while its radio group's checked count drifts above the one it allows.
//
// C++: ChoosePlayerMenu::ButtonSetState (chooseplayermenu.cpp)
func (m *choosePlayerMenu) setTypeRestState(team int, down bool) {
	m.ctx.Sys.SetRestState(m.typeBtns[typeColumn(m.params.Teams[team].Type)*4+team], down)
}

// typeColumn maps a team type to its button column.
func typeColumn(t game.TeamType) int {
	switch t {
	case game.TeamPlayer:
		return 0
	case game.TeamComputer:
		return 1
	default:
		return 2
	}
}

// makeClanButton builds one team's clan button: the team plate with
// the clan logo (CLN0LOGO is the "random" logo), disabled and shown
// pressed when the team is not human.
//
// C++: ChoosePlayerMenu::ButtonInit (index 12..15)
func makeClanButton(ctx *Context, params *game.StartParams, team int) (gnw.ButtonID, error) {
	upTag := fmt.Sprintf("CH_TM%d_U", team+1)
	downTag := fmt.Sprintf("CH_TM%d_D", team+1)

	isPlayer := params.Teams[team].Type == game.TeamPlayer
	logo := "CLN0LOGO"
	if isPlayer {
		clan := params.Teams[team].Clan
		if clan >= 1 && clan <= 8 {
			logo = fmt.Sprintf("CLN%dLOGO", clan)
		}
	} else {
		upTag = downTag // teams that are not human show the dark plate
	}

	bid, err := ctx.imageButton(mainWindow, 495, cpRowY(team)-6, upTag, downTag,
		logo, 41, 40, false, cpClanBase+team)
	if err != nil {
		return -1, err
	}
	if !isPlayer {
		ctx.Sys.DisableButton(bid)
	}
	return bid, nil
}

// drawChoosePlayerTitles paints the menu title and column headers.
//
// C++: ChoosePlayerMenu::UpdateButtons title part
func drawChoosePlayerTitles(ctx *Context, isSinglePlayer bool) {
	ctx.Sys.Fonts().SetFont(fonts.Font5)
	title := ctx.text("55c8", "Hot Seat Menu")
	if isSinglePlayer {
		title = ctx.text("5512", "Custom Game Menu")
	}
	ctx.printCentered(title, gfx.NewRect(230, 6, 410, 26), colorGreen)
	ctx.printCentered(ctx.text("5985", "Team"), gfx.NewRect(61, 31, 141, 51), colorGreen)
	ctx.printCentered(ctx.text("9ffa", "Human"), gfx.NewRect(162, 31, 241, 51), colorGreen)
	ctx.printCentered(ctx.text("76ed", "Computer"), gfx.NewRect(272, 31, 351, 51), colorGreen)
	ctx.printCentered(ctx.text("dc93", "None"), gfx.NewRect(380, 31, 459, 51), colorGreen)
	ctx.printCentered(ctx.text("bda5", "Clan"), gfx.NewRect(497, 31, 576, 51), colorGreen)
	ctx.Sys.Fonts().SetFont(fonts.Font1)
}

// validatePlayers returns an error message when the assignment cannot
// start a game, or "".
//
// C++: ChoosePlayerMenu::EventDone
func validatePlayers(ctx *Context, params *game.StartParams, isSinglePlayer bool) string {
	humans, computers := 0, 0
	for _, t := range params.Teams {
		switch t.Type {
		case game.TeamPlayer:
			humans++
		case game.TeamComputer:
			computers++
		}
	}
	if isSinglePlayer {
		if humans+computers < 2 {
			return ctx.text("bc94", "There must be at least two players to begin a new game.")
		}
	} else if humans < 2 {
		return ctx.text("06ec", "There must be at least two human players to begin a hot seat game.")
	}
	return ""
}

// assignTeamNames fills the team names after a successful setup: the
// single human player gets the settings player name, everyone else
// the default team names.
//
// C++: menu_choose_player_menu_loop tail
func assignTeamNames(ctx *Context, params *game.StartParams, isSinglePlayer bool, playerName string) {
	defaults := [game.TeamCount]string{
		ctx.text("f394", "Red Team"), ctx.text("a8a6", "Green Team"),
		ctx.text("a3ee", "Blue Team"), ctx.text("319d", "Gray Team"),
	}
	named := false
	for i := range params.Teams {
		if isSinglePlayer && !named && params.Teams[i].Type == game.TeamPlayer {
			params.Teams[i].Name = playerName
			named = true
			continue
		}
		params.Teams[i].Name = defaults[i]
	}
}
