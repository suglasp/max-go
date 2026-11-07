// clanselect.go : implements clan selection with portraits and bonus descriptions.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package menus

import (
	"fmt"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/gnw"
	"github.com/suglasp/max-go/internal/platform"
)

// Input codes of the clan select screen.
const (
	csClan0 = 1000 + iota // ..csClan7
	csClan1
	csClan2
	csClan3
	csClan4
	csClan5
	csClan6
	csClan7
	csRandom
	csCancel
	csHelp
	csDone
)

// clanPlate is one clan portrait plate on the select screen: its position and
// the clan's own voice over press sound.
type clanPlate struct {
	x, y int
	sfx  string
}

// clanButtonDefs: the 8 clan plates and their press sounds (each clan
// has its own voice over resource).
var clanButtonDefs = [8]clanPlate{
	{46, 44, "CCHOS0"}, {201, 44, "CCRIM0"}, {355, 44, "CVONG0"}, {510, 44, "CAYER0"},
	{46, 194, "CMUSA0"}, {201, 194, "CSACR0"}, {355, 194, "CKNIG0"}, {510, 194, "CAXIS0"},
}

// clanNameRects are the label boxes under the clan plates.
var clanNameRects = [8]gfx.Rect{
	gfx.NewRect(32, 140, 142, 160), gfx.NewRect(188, 140, 298, 160),
	gfx.NewRect(344, 140, 454, 160), gfx.NewRect(497, 140, 607, 160),
	gfx.NewRect(32, 290, 142, 310), gfx.NewRect(188, 290, 298, 310),
	gfx.NewRect(344, 290, 454, 310), gfx.NewRect(497, 290, 607, 310),
}

// colorSilver is the dimmed text index (0xA2) used for unselected clan
// names and the bonus listing.
const colorSilver = 0xA2

// ClanSelect shows the clan selection screen for a team and returns
// the chosen clan: game.ClanRandom (0) for Random, 1..8 for a clan,
// or currentClan unchanged on cancel.
//
// C++: ClanSelectMenu (clanselectmenu.cpp) + menu_clan_select_menu_loop
func ClanSelect(ctx *Context, currentClan int) (int, error) {
	pic, err := ctx.loadBig("CLANSEL")
	if err != nil {
		return currentClan, err
	}
	ctx.drawBigAt(pic, 0, 0)
	ctx.Plat.SetPalette(&pic.Palette)

	ctx.Sys.Fonts().SetFont(fonts.Font5)
	ctx.printCentered(ctx.text("897f", "Clan Select Menu"), gfx.NewRect(230, 6, 410, 26), colorGreen)

	// Pristine background under the description display.
	// C++: ClanSelectMenu::image (Image::Copy / Image::Write)
	textRect := gfx.NewRect(41, 350, 610, 409)
	textBg := ctx.snapshotArea(textRect)

	var buttons []gnw.ButtonID
	defer func() {
		for _, b := range buttons {
			ctx.Sys.DeleteButton(b)
		}
	}()

	var clanBtns []gnw.ButtonID
	for i, d := range clanButtonDefs {
		upTag := fmt.Sprintf("CH_CN%d_U", i+1)
		downTag := fmt.Sprintf("CH_CN%d_D", i+1)
		logoTag := fmt.Sprintf("CLN%dLOGO", i+1)
		bid, err := ctx.imageButton(mainWindow, d.x, d.y, upTag, downTag, logoTag, 41, 40, true, csClan0+i)
		if err != nil {
			return currentClan, err
		}
		ctx.registerButtonSfx(bid, d.sfx)
		buttons = append(buttons, bid)
		clanBtns = append(clanBtns, bid)
	}
	ctx.Sys.GroupRadioButtons(clanBtns, nil)

	ctx.Sys.Fonts().SetFont(fonts.Font1)
	if err := ctx.addBottomPlates([]bottomPlate{
		{243, "MNUBTN3U", ctx.text("7e91", "Random"), csRandom, "CRAND0"},
		{354, "MNUBTN4U", ctx.text("f5ee", "Cancel"), csCancel, "CCANC0"},
		{465, "MNUBTN5U", ctx.text("eb1b", "?"), csHelp, "CHELP0"},
		{514, "MNUBTN6U", ctx.text("aff0", "Done"), csDone, "CDONE0"},
	}, &buttons); err != nil {
		return currentClan, err
	}

	sel := currentClan
	if sel < 1 || sel > 8 {
		sel = 0
	}
	if sel > 0 {
		ctx.Sys.SetRestState(clanBtns[sel-1], true)
	}
	drawClanTexts(ctx, sel, textRect, textBg)

	ctx.Sys.MouseShow()

	for {
		key := ctx.Sys.GetInput()
		switch {
		case key == -1:
			continue
		case key >= csClan0 && key <= csClan7:
			sel = key - csClan0 + 1
			drawClanTexts(ctx, sel, textRect, textBg)
		case key == csRandom:
			return game.ClanRandom, nil
		case key == csDone, key == platform.KeyReturn:
			return sel, nil
		case key == csCancel, key == platform.KeyEscape:
			return currentClan, nil
		case key == csHelp, key == '?':
			Help(ctx, "CLAN_SETUP")
		}
	}
}

// clanTextColX are the left edges of the two bonus text columns inside
// the black display, taken from the two MenuTitleItem bounds in the C++
// (clanselectmenu.hpp: {41,...} and {330,...}).
var clanTextColX = [2]int{41, 330}

// drawClanTexts paints the clan names under the plates (selected one
// green) and the selected clan's description plus unit bonuses in the
// bottom display.
//
// C++: ClanSelectMenu::SelectMenuItems + DrawClanUpgrades
func drawClanTexts(ctx *Context, sel int, textRect gfx.Rect, textBg *gfx.Buffer) {
	ctx.Sys.Fonts().SetFont(fonts.Font5)

	for i, r := range clanNameRects {
		color := colorSilver
		if i+1 == sel {
			color = colorGreen
		}
		name := ""
		if i < len(ctx.Assets.Clans) {
			name = ctx.Assets.Clans[i].Name
		}
		ctx.printCentered(name, r, color|fonts.FlagOutline)
	}

	ctx.restoreArea(textRect, textBg)
	if sel >= 1 && sel <= len(ctx.Assets.Clans) {
		drawClanUpgrades(ctx, ctx.Assets.Clans[sel-1], textRect)
	}
	ctx.Sys.Fonts().SetFont(fonts.Font1)
}

// drawClanUpgrades places the green clan description and unit bonuses in two columns. The left column fills first, then entries continue in the right column.
//
// clanRow is one line of the clan description panel: its text and colour.
type clanRow struct {
	text  string
	color int
}

// C++: ClanSelectMenu::DrawClanUpgrades (clanselectmenu.cpp)
func drawClanUpgrades(ctx *Context, clan assets.Clan, r gfx.Rect) {
	f := ctx.Sys.Fonts().Font()
	rowH := f.Height + 1

	rows := []clanRow{{clan.Text, colorGreen | fonts.FlagOutline}}
	for _, u := range clan.Upgrades {
		rows = append(rows, clanRow{
			fmt.Sprintf("%s: %s.", u.Unit, u.Mods),
			colorSilver | fonts.FlagOutline,
		})
	}

	maxRows := (r.Height() + 1) / rowH
	if maxRows < 1 {
		maxRows = 1
	}

	for i, row := range rows {
		col := i / maxRows
		if col >= len(clanTextColX) {
			break // only two columns fit the display
		}
		x := clanTextColX[col]
		// A line in the left column may use the full width up to the display
		// edge unless a line in the right column shares its row, in which case
		// it stops at the next column's edge to avoid overlap.
		w := r.LRX - x
		if col+1 < len(clanTextColX) && i+maxRows < len(rows) {
			w = clanTextColX[col+1] - x
		}
		y := r.ULY + (i%maxRows)*rowH
		ctx.Sys.Print(mainWindow, row.text, w, x, y, row.color)
	}
}
