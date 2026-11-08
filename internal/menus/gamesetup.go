// gamesetup.go : implements mission selection for single-player scenarios.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++: gamesetupmenu.cpp - GameSetupMenu wizard
package menus

import (
	"fmt"

	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/gnw"
	"github.com/suglasp/max-go/internal/missionfile"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/sprites"
)

// MAINPIC lists missions on the left and the selected briefing on the right, with scrolling and Start, Cancel, and Help controls.

// Layout constants are the original GameSetupMenu control rects
// (gamesetupmenu.hpp game_setup_menu_controls / game_setup_menu_titles).
const (
	gsRowCount = 12 // GAME_SETUP_MENU_MISSION_COUNT

	gsListX    = 18 // mission list column (18,184)-(312,423)
	gsListY0   = 184
	gsRowPitch = 20
	gsRowH     = 19
	gsListW    = 294 // 312-18

	gsTitleX, gsTitleY, gsTitleW, gsTitleH = 400, 174, 180, 21  // selected title bar (400,174)-(580,195)
	gsDescX, gsDescY, gsDescW, gsDescH     = 354, 215, 266, 201 // briefing text box (354,215)-(620,416)

	gsBtnY = 438 // bottom button row

	// Input codes emitted by the screen's buttons.
	gsRowBase  = 2001 // row i -> gsRowBase+i (i in 0..11)
	gsListUp   = 2100
	gsListDown = 2101
	gsBriefUp  = 2110
	gsBriefDn  = 2111
	gsCancel   = 1000
	gsHelp     = 1001
	gsStart    = 1002
)

// colorYellow is COLOR_YELLOW, the highlight for the selected list row;
// colorLockedRow dims a campaign mission that is still locked.
const (
	colorYellow    = 0x04
	colorLockedRow = 0x08 // dark grey: a locked (not yet unlocked) row
)

// gameSetupMenu is the mission select screen state: the mission list, the
// selection and paging, and the panel art the draw methods share.
// C++: gamesetupmenu.cpp - GameSetupMenu.
type gameSetupMenu struct {
	ctx      *Context
	missions []missionfile.Mission
	unlocked int

	sel         int
	firstOnPage int // 1-based index of the first mission shown on this page
	descLines   []string
	descRow     int

	pic                  *sprites.Big
	bgrL, bgrR           *sprites.Simple
	subtitle, bigL, bigR *sprites.Simple
	f5                   *fonts.Font
	buf                  *gfx.Buffer
	lineH, descPerPage   int
}

// RunGameSetup shows missions and returns the selected index with start=true, or start=false on Cancel or Escape.
//
// initialSel is the row to select in advance (and the page to open on), pass the last
// choice so stepping back from the setup screen restores it, or 0 for the first.
//
// unlocked caps how many missions are selectable from the top (campaign
// progression): missions at index >= unlocked are shown greyed and can't be
// picked. Pass 0 (or >= len(missions)) to leave every mission selectable.
// C++: GameSetupMenu::DrawMissionList disabling rows where last_campaign <= index.
//
// C++: GameSetupMenu::Run (gamesetupmenu.cpp).
func RunGameSetup(ctx *Context, title string, missions []missionfile.Mission, initialSel, unlocked int, helpSection string) (selected int, start bool, err error) {
	if unlocked <= 0 || unlocked > len(missions) {
		unlocked = len(missions)
	}
	pic, err := ctx.loadBig("MAINPIC")
	if err != nil {
		return -1, false, err
	}
	// Panel frame art; absent art just leaves the MAINPIC background showing (the
	// text still draws). The left list sits in the BGRSCRNL/BGRSCRNR portrait frame
	// panels (menu.cpp menu_draw_menu_portrait_frame); the right briefing sits in
	// the BIGSCRNL/BIGSCRNR info panels (gamesetupmenu.cpp), with SUBTITLE as the
	// selected title bar.
	bgrL, _ := ctx.loadSimple("BGRSCRNL")
	bgrR, _ := ctx.loadSimple("BGRSCRNR")
	subtitle, _ := ctx.loadSimple("SUBTITLE")
	bigL, _ := ctx.loadSimple("BIGSCRNL")
	bigR, _ := ctx.loadSimple("BIGSCRNR")

	oldFont := ctx.Sys.Fonts().GetFont()
	defer ctx.Sys.Fonts().SetFont(oldFont)
	f5 := ctx.Sys.Fonts().Get(fonts.Font5)

	m := &gameSetupMenu{
		ctx:         ctx,
		missions:    missions,
		unlocked:    unlocked,
		sel:         -1,
		firstOnPage: 1,
		pic:         pic,
		bgrL:        bgrL,
		bgrR:        bgrR,
		subtitle:    subtitle,
		bigL:        bigL,
		bigR:        bigR,
		f5:          f5,
		buf:         ctx.Sys.Buf(mainWindow),
		lineH:       f5.Height + 1,
	}
	m.descPerPage = gsDescH / m.lineH
	if m.descPerPage < 1 {
		m.descPerPage = 1
	}

	if len(missions) > 0 {
		m.sel = initialSel
		if m.sel < 0 || m.sel >= len(missions) {
			m.sel = 0
		}
		if m.locked(m.sel) { // a step back or last choice may now point past the unlocked set
			m.sel = unlocked - 1
		}
	}
	if m.sel >= 0 {
		m.firstOnPage = (m.sel/gsRowCount)*gsRowCount + 1
	}
	m.reflow()

	// Background + caption.
	ctx.drawBigAt(pic, 0, 0)
	ctx.Plat.SetPalette(&pic.Palette)
	ctx.drawMenuTitle(title)
	ctx.setHandCursor()

	buttons := m.buildWidgets()
	defer func() {
		for _, b := range buttons {
			ctx.Sys.DeleteButton(b)
		}
	}()

	m.drawList()
	m.drawDesc()
	ctx.Sys.MouseShow()

	for {
		key := ctx.Sys.GetInput()
		if key == -1 {
			continue
		}
		if selected, start, done := m.handleKey(key, helpSection); done {
			return selected, start, nil
		}
	}
}

// buildWidgets creates the mission row click regions and the bottom button row,
// returning the ids so the caller can delete them on exit.
func (m *gameSetupMenu) buildWidgets() []gnw.ButtonID {
	ctx := m.ctx
	var buttons []gnw.ButtonID

	// 12 transparent click regions over the list rows (drawn text changes per page).
	for i := 0; i < gsRowCount; i++ {
		ry := gsListY0 + i*gsRowPitch
		bid := ctx.Sys.AddButton(mainWindow, gsListX, ry, gsListW, gsRowH,
			gnw.NoValue, gnw.NoValue, gnw.NoValue, gsRowBase+i, nil, nil, nil, 0)
		if bid >= 0 {
			ctx.registerButtonSfx(bid, "MBUTT0")
			buttons = append(buttons, bid)
		}
	}

	ctx.Sys.Fonts().SetFont(fonts.Font1) // plate captions use Font1
	addImg := func(x int, up, down string, value int) {
		if b, e := ctx.imageButton(mainWindow, x, gsBtnY, up, down, "", 0, 0, false, value); e == nil {
			ctx.registerButtonSfx(b, "MBUTT0")
			buttons = append(buttons, b)
		}
	}
	addCap := func(x int, up, down, label string, value int, sfx string) {
		if b, e := ctx.plateButton(mainWindow, x, gsBtnY, up, down, label, value); e == nil {
			ctx.registerButtonSfx(b, sfx)
			buttons = append(buttons, b)
		}
	}
	addImg(16, "MNUUAROU", "MNUUAROD", gsListUp)   // list page up
	addImg(48, "MNUDAROU", "MNUDAROD", gsListDown) // list page down
	addImg(563, "MNUUAROU", "MNUUAROD", gsBriefUp) // briefing scroll up
	addImg(596, "MNUDAROU", "MNUDAROD", gsBriefDn) // briefing scroll down
	addCap(200, "MNUBTN4U", "MNUBTN4D", ctx.text("639d", "Cancel"), gsCancel, "NCANC0")
	addCap(312, "MNUBTN5U", "MNUBTN5D", ctx.text("da8e", "?"), gsHelp, "NHELP0")
	addCap(361, "MNUBTN6U", "MNUBTN6D", ctx.text("f0a3", "Start"), gsStart, "NDONE0")
	return buttons
}

// handleKey runs one input event of the mission list loop. done reports that the
// screen is finished, with selected and start carrying its result.
func (m *gameSetupMenu) handleKey(key int, helpSection string) (selected int, start, done bool) {
	ctx := m.ctx
	missions := m.missions
	switch {
	case key == gsCancel || key == platform.KeyEscape:
		return -1, false, true
	case key == gsStart || key == platform.KeyReturn:
		if m.sel >= 0 && m.sel < len(missions) && !m.locked(m.sel) {
			return m.sel, true, true
		}
	case key == gsHelp || key == '?':
		// The same list serves training, scenarios, the campaign and the multiplayer
		// scenarios, and the help database has a section per flavour; the caller names
		// the one it opened. C++: gamesetupmenu.cpp:237 - EventHelp switches on the
		// mission category.
		Help(ctx, helpSection)
	case key >= gsRowBase && key < gsRowBase+gsRowCount:
		gi := m.firstOnPage - 1 + (key - gsRowBase)
		if gi >= 0 && gi < len(missions) && gi != m.sel && !m.locked(gi) {
			m.sel = gi
			m.reflow()
			m.drawList()
			m.drawDesc()
		}
	case key == gsListUp:
		if m.firstOnPage > gsRowCount {
			m.firstOnPage -= gsRowCount
			m.drawList()
		}
	case key == gsListDown:
		if m.firstOnPage+gsRowCount <= len(missions) {
			m.firstOnPage += gsRowCount
			m.drawList()
		}
	case key == gsBriefUp:
		if m.descRow > 0 {
			m.descRow -= m.descPerPage
			if m.descRow < 0 {
				m.descRow = 0
			}
			m.drawDesc()
		}
	case key == gsBriefDn:
		if m.descRow+m.descPerPage < len(m.descLines) {
			m.descRow += m.descPerPage
			m.drawDesc()
		}
	}
	return -1, false, false
}

// locked reports whether a mission index is still campaign locked.
func (m *gameSetupMenu) locked(gi int) bool { return gi >= m.unlocked }

// reflow rewraps the selected mission's briefing text and rewinds the scroll.
func (m *gameSetupMenu) reflow() {
	m.descLines = nil
	m.descRow = 0
	if m.sel >= 0 && m.sel < len(m.missions) {
		m.descLines = wrapLines(m.f5, m.missions[m.sel].Description, gsDescW-4, 0)
	}
}

// copyBg restores a MAINPIC region before redraw so text does not stack. It clamps to the buffer.
func (m *gameSetupMenu) copyBg(x, y, w, h int) {
	sx, sy := x-m.pic.ULX, y-m.pic.ULY
	if sx < 0 || sy < 0 {
		return
	}
	w = min(w, min(m.buf.W-x, m.pic.Buffer().W-sx))
	h = min(h, min(m.buf.H-y, m.pic.Buffer().H-sy))
	if w <= 0 || h <= 0 {
		return
	}
	m.buf.Sub(x, y, w, h).Copy(m.pic.Buffer().Sub(sx, sy, w, h))
}

// drawList repaints the mission list page: panel frames, row titles, separators.
func (m *gameSetupMenu) drawList() {
	m.ctx.Sys.Lock()
	defer m.ctx.Sys.Unlock()
	// Restore the stone background under the list, then lay the black list
	// panel (BGRSCRNL at 6,173 + BGRSCRNR at 166,173) so the rows sit framed.
	// Height stops short of the bottom button row (y=438).
	m.copyBg(6, 173, 320, 250)
	if m.bgrL != nil {
		m.bgrL.Draw(m.buf, 6, 173, false)
	}
	if m.bgrR != nil {
		m.bgrR.Draw(m.buf, 166, 173, false)
	}
	for i := 0; i < gsRowCount; i++ {
		gi := m.firstOnPage - 1 + i
		ry := gsListY0 + i*gsRowPitch
		if gi >= 0 && gi < len(m.missions) {
			color := colorGreen
			switch {
			case m.locked(gi):
				color = colorLockedRow // not yet unlocked in the campaign
			case gi == m.sel:
				color = colorYellow
			}
			blitInBox(m.buf, m.f5, fmt.Sprintf("%d. %s", gi+1, m.missions[gi].Title),
				gsListX+2, ry, gsListW-4, gsRowH, color, false)
		}
		if i < gsRowCount-1 { // green separator under each row but the last
			if y := ry + gsRowH; y < m.buf.H {
				m.buf.Sub(gsListX, y, gsListW, 1).Fill(colorGreen)
			}
		}
	}
}

// drawDesc repaints the selected mission's title bar and briefing text panel.
func (m *gameSetupMenu) drawDesc() {
	m.ctx.Sys.Lock()
	defer m.ctx.Sys.Unlock()
	m.copyBg(gsTitleX, gsTitleY, gsTitleW, gsTitleH)
	m.copyBg(gsDescX, gsDescY, gsDescW, gsDescH)
	if m.bigL != nil {
		m.bigL.Draw(m.buf, 342, 202, false)
	}
	if m.bigR != nil {
		m.bigR.Draw(m.buf, 482, 202, false)
	}
	if m.subtitle != nil {
		m.subtitle.Draw(m.buf, gsTitleX, gsTitleY, false)
	}
	if m.sel >= 0 && m.sel < len(m.missions) {
		blitInBox(m.buf, m.f5, m.missions[m.sel].Title, gsTitleX, gsTitleY, gsTitleW, gsTitleH,
			colorGreen|fonts.FlagOutline, true)
		end := min(m.descRow+m.descPerPage, len(m.descLines))
		for i := m.descRow; i < end; i++ {
			blitInBox(m.buf, m.f5, m.descLines[i], gsDescX+2, gsDescY+(i-m.descRow)*m.lineH,
				gsDescW-4, m.lineH, cargoTan, false)
		}
	}
}
