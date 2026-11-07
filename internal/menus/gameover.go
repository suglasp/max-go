// gameover.go : implements the victory and defeat screen with team statistics.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package menus

import (
	"fmt"

	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/gnw"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/screendata"
	"github.com/suglasp/max-go/internal/winloss"
)

// The GameOver data transfer objects (GameOverData, GameOverTeam) now live in
// internal/screendata, shared with the gamescreen host.

// gameOverDecorator draws the content specific to the outcome of the game over screen.
// Called with the window lock held.
// C++: VICTORY_STATE_* branch in menu_draw_game_over_screen (menu.cpp)
type gameOverDecorator func(ctx *Context, buf *gfx.Buffer, f *fonts.Font, data screendata.GameOverData)

// rankString returns the vanilla placement label for a team rank.
// C++: menu_draw_game_over_screen - _(6b25)/_(aaea)/etc. (menu.cpp)
func rankString(ctx *Context, r int) string {
	switch r {
	case 0:
		return ctx.text("153d", "Eliminated")
	case 1:
		return ctx.text("6b25", "1st place")
	case 2:
		return ctx.text("aaea", "2nd place")
	case 3:
		return ctx.text("bb8a", "3rd place")
	case 4:
		return ctx.text("cdd0", "4th place")
	}
	return ""
}

// gameOverTeamColors are the palette indices used for team number headers.
var gameOverTeamColors = [4]uint8{0x02, 0x23, 0x3B, 0x67}

// RunGameOver shows the statistics screen at the end of the game as a clean standalone
// state: a random ENDGAME background, the stats table, ENDARM decoration, and
// an ENDOK button. No game world is visible underneath.
//
// Dispatches to drawGameOverWin, drawGameOverLose, or drawGameOverDeath based
// on data.Outcome.
//
// C++: menu.cpp - menu_wrap_up_game + menu_draw_game_over_screen
func RunGameOver(ctx *Context, data screendata.GameOverData) error {
	// Random ENDGAME1-9 background chosen by turn counter (the original's method).
	bgTag := fmt.Sprintf("ENDGAME%d", 1+data.TurnCounter%9)
	bg, err := ctx.loadBig(bgTag)
	if err != nil {
		dbg.Printf("game over screen: loadBig %s failed: %v (nothing drawn)", bgTag, err)

		return err
	}

	// Trace the inputs so a "score screen not shown" report can be pinned to the data
	// (empty teams) versus the display, without guessing. See tracking sec F2.
	dbg.Printf("game over screen: bg=%s outcome=%v teams=%d", bgTag, data.Outcome, len(data.Teams))

	ctx.Plat.SetPalette(&bg.Palette)
	ctx.drawBigAt(bg, 0, 0)

	ctx.Sys.Fonts().SetFont(fonts.Font5)
	f := ctx.Sys.Fonts().Get(fonts.Font5)
	buf := ctx.Sys.Buf(mainWindow)

	// Select the content decorator specific to the outcome.
	var decorate gameOverDecorator
	switch data.Outcome {
	case winloss.PlayerWon:
		decorate = drawGameOverWin
	case winloss.PlayerLost:
		decorate = drawGameOverLose
	default:
		decorate = drawGameOverDeath
	}

	ctx.Sys.Lock()
	// "Game Over" title is common to all outcomes.
	drawGameOverTitle(buf, f, ctx.text("71f1", "Game Over"))
	// Content specific to the outcome (team columns, narrative, etc.).
	decorate(ctx, buf, f, data)
	ctx.Sys.Unlock()

	// ENDARM decorative bar and ENDOK button are common chrome.
	bid := drawGameOverChrome(ctx, buf)

	ctx.Sys.MouseShow()
	defer ctx.Sys.MouseHide()
	ctx.Sys.FlushInput()

	for {
		key := ctx.Sys.GetInput()
		if key == platform.KeyReturn || key == platform.KeyEscape {
			dbg.Printf("game over screen: dismissed by key %d", key)

			break
		}
	}

	ctx.Sys.DeleteButton(bid)
	return nil
}

// drawGameOverWin draws the content shown when the player has won.
// For skirmish: rank table. For campaign: "Mission Success" + narrative text would go here.
// C++: menu_draw_game_over_screen VICTORY_STATE_WON / generic won path
func drawGameOverWin(ctx *Context, buf *gfx.Buffer, f *fonts.Font, data screendata.GameOverData) {
	drawGameOverTeamColumns(ctx, buf, f, data)
	drawGameOverLabels(ctx, buf, f)
}

// drawGameOverLose draws the content shown when the player has lost.
// For skirmish: rank table. For campaign: "Mission Failed" + narrative text would go here.
// C++: menu_draw_game_over_screen VICTORY_STATE_LOST / generic lost path
func drawGameOverLose(ctx *Context, buf *gfx.Buffer, f *fonts.Font, data screendata.GameOverData) {
	drawGameOverTeamColumns(ctx, buf, f, data)
	drawGameOverLabels(ctx, buf, f)
}

// drawGameOverDeath uses the defeat layout when all units are eliminated.
// C++: menu_draw_game_over_screen - eliminated / no units remaining
func drawGameOverDeath(ctx *Context, buf *gfx.Buffer, f *fonts.Font, data screendata.GameOverData) {
	drawGameOverTeamColumns(ctx, buf, f, data)
	drawGameOverLabels(ctx, buf, f)
}

// drawGameOverTitle draws the centred "Game Over" heading at y=10.
// C++: menu_draw_game_over_screen - _(71f1) at (x=100, w=440, y=10)
func drawGameOverTitle(buf *gfx.Buffer, f *fonts.Font, title string) {
	const whiteColor = 0xFF
	titleW := f.Width(title)
	titleX := 100 + (440-titleW)/2
	if titleX < 0 {
		titleX = 0
	}
	f.Blit(buf.Sub(titleX, 10, buf.W-titleX, f.Height), title, titleW+1, whiteColor)
}

// drawGameOverTeamColumns draws the header and stat rows for each team.
// C++: menu_draw_game_over_screen - column loop
func drawGameOverTeamColumns(ctx *Context, buf *gfx.Buffer, f *fonts.Font, data screendata.GameOverData) {
	const silverColor = 0xA2
	const whiteColor = 0xFF

	ulx := 80
	for i, team := range data.Teams {
		ulx += 110
		lrx := ulx + 90
		colW := lrx - ulx
		col := int(gameOverTeamColors[i%len(gameOverTeamColors)])

		// Team number at y=44.
		numStr := fmt.Sprintf("%d", i+1)
		numW := f.Width(numStr)
		numX := ulx + (colW-numW)/2
		f.Blit(buf.Sub(numX, 44, buf.W-numX, f.Height), numStr, numW+1, col)

		// Team name centred at y=61.
		nameW := f.Width(team.Name)
		nameX := ulx + (colW-nameW)/2
		if nameX < 0 {
			nameX = ulx
		}
		f.Blit(buf.Sub(nameX, 61, buf.W-nameX, f.Height), team.Name, nameW+1, silverColor)

		// The points row. In a MISSION whose objective decided the outcome, the player's
		// cell says so in words instead - the mission was not about the score.
		// C++ ported : menu.cpp - _(bb4a) / _(fc74) in place of the points + placing
		if data.Scripted && team.Player {
			verdict := ctx.text("fc74", "Mission Failed")
			if data.Outcome == winloss.PlayerWon {
				verdict = ctx.text("bb4a", "Mission Success")
			}

			vW := f.Width(verdict)
			vX := ulx + (colW-vW)/2
			if vX < 0 {
				vX = ulx
			}

			f.Blit(buf.Sub(vX, 116, buf.W-vX, f.Height), verdict, vW+1, whiteColor)
		} else {
			// Points at y=116.
			pts := fmt.Sprintf("%d", team.Points)
			pW := f.Width(pts)
			pX := ulx + (colW-pW)/2
			f.Blit(buf.Sub(pX, 116, buf.W-pX, f.Height), pts, pW+1, whiteColor)

			// Placement string at y=132.
			place := rankString(ctx, team.Rank)
			if place != "" {
				plW := f.Width(place)
				plX := ulx + (colW-plW)/2
				if plX < 0 {
					plX = ulx
				}
				f.Blit(buf.Sub(plX, 132, buf.W-plX, f.Height), place, plW+1, whiteColor)
			}
		}

		// Stat rows: live TeamStats totals tracked by the sim during play.
		// Upgrades appends " Gold" (_(6249)) after the number, matching the original's layout.
		goldSuffix := " " + ctx.text("6249", "Gold")
		statRows := [7]gameOverStatRow{
			{166, team.FactoriesBuilt, ""},
			{198, team.MinesBuilt, ""},
			{230, team.BuildingsBuilt, ""},
			{246, team.BuildingsLost, ""},
			{278, team.UnitsBuilt, ""},
			{294, team.UnitsLost, ""},
			{326, team.GoldOnUpgrades, goldSuffix},
		}
		for _, r := range statRows {
			s := fmt.Sprintf("%d", r.value) + r.suffix
			sw := f.Width(s)
			sx := ulx + (colW-sw)/2
			if sx < ulx {
				sx = ulx
			}
			f.Blit(buf.Sub(sx, r.y, buf.W-sx, f.Height), s, sw+1, silverColor)
		}
	}
}

// gameOverStatRow is one right column stat line: its y position, the value
// printed and an optional suffix (" Gold" for the upgrades row).
type gameOverStatRow struct {
	y      int
	value  int
	suffix string
}

// gameOverLabelRow is one left column label line: its y position plus the lang
// file key and English fallback.
type gameOverLabelRow struct {
	y             int
	key, fallback string
}

// drawGameOverLabels draws the row labels in the left column.
// C++: menu_draw_game_over_screen - label column (x=0, w=172)
func drawGameOverLabels(ctx *Context, buf *gfx.Buffer, f *fonts.Font) {
	const silverColor = 0xA2

	labelRows := [8]gameOverLabelRow{
		{116, "77de", "Points"},
		{166, "7564", "Factories Built"},
		{198, "c9c5", "Mines Built"},
		{230, "7b17", "Buildings Built"},
		{246, "51e1", "Lost"},
		{278, "3d2c", "Units Built"},
		{294, "5c73", "Lost"},
		{326, "1b9b", "Upgrades"},
	}
	// Align labels to the right at x=172 (the right boundary of the label column) so
	// they sit adjacent to the data columns. Our bitmap Font5 is smaller than the
	// original's TTF, so centering left long labels uncomfortably close to the screen edge.
	const labelRightEdge = 172
	const labelMinX = 6
	for _, l := range labelRows {
		lbl := ctx.text(l.key, l.fallback)
		lw := f.Width(lbl)
		lx := labelRightEdge - lw
		if lx < labelMinX {
			lx = labelMinX
		}
		f.Blit(buf.Sub(lx, l.y, buf.W-lx, f.Height), lbl, labelRightEdge-lx, silverColor)
	}
}

// drawGameOverChrome blits the ENDARM decorative bar and registers the ENDOK
// button. Returns the button ID so the caller can delete it on exit.
// C++: menu_draw_game_over_screen - DecodeSimpleImage(ENDARM) + Button(ENDOK)
func drawGameOverChrome(ctx *Context, buf *gfx.Buffer) gnw.ButtonID {
	// ENDARM: transparent_color = Pix[0] (NOT index 0); apply KeyToZero before TransCopy.
	if arm, err := ctx.loadSimple("ENDARM"); err == nil {
		armBuf := arm.Buffer().Clone()
		armBuf.KeyToZero(arm.TransparentColor())
		ax := 273 - arm.ULX
		ay := 480 - arm.H - arm.ULY
		ctx.Sys.Lock()
		aw := min(armBuf.W, buf.W-ax)
		ah := min(armBuf.H, buf.H-ay)
		if ax >= 0 && ay >= 0 && aw > 0 && ah > 0 {
			buf.Sub(ax, ay, aw, ah).TransCopy(armBuf.Sub(0, 0, aw, ah))
		}
		ctx.Sys.Unlock()
	}

	// ENDOK button "OK" at (293, 458); rvalue = KeyEscape so ESC presses it.
	ctx.Sys.Fonts().SetFont(fonts.Font5)
	bid, _ := ctx.plateButton(mainWindow, 293, 458, "ENDOK_U", "ENDOK_D",
		ctx.text("5653", "OK"), platform.KeyEscape)
	ctx.registerButtonSfx(bid, "MBUTT0") // the small menu button blip (mengens4.wav), like the menu bar
	return bid
}
