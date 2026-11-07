// tips.go : selects and displays loading-screen hints.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package menus

import (
	"strings"

	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/gnw"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/sprites"
)

// Tips draws the BGRSCRNL/BGRSCRNR frame over the left panel and pages the help text with scroll arrows and Cancel or Escape.
//
// C++: menu_draw_tips / menu_draw_tips_frame (menu.cpp). The original toggles the
// panel in place over the New Game buttons; here it is a small modal that returns
// to the New Game menu (which the dispatch loop shows again).

const (
	// Tips text panel rect (19,185)-(315,421), menu_draw_tips_frame.
	tipsX, tipsY   = 19, 185
	tipsW, tipsH   = 296, 236
	tipsColor      = 0xA2 // the text colour menu_draw_tips_frame blits with
	tipsBtnY       = 438  // scroll arrow row (matches the original 16,438 / 48,438)
	tipsCancelX    = 200
	tipsCancelBtnY = 438

	// Button input codes.
	tipsUp     = 1000
	tipsDown   = 1001
	tipsCancel = 1002
)

// tipsMenu is the Tips screen state: the paged text, where the page starts, and
// the art the panel redraw needs.
//
// C++: menu_draw_tips (menu.cpp)
type tipsMenu struct {
	ctx *Context

	pic        *sprites.Big
	bgrL, bgrR *sprites.Simple
	f5         *fonts.Font
	buf        *gfx.Buffer

	lines   []string
	row     int
	perPage int
	lineH   int
}

// RunTips shows the Tips screen and returns when the player cancels (or ESC).
func RunTips(ctx *Context) error {
	pic, err := ctx.loadBig("MAINPIC")
	if err != nil {
		return err
	}
	// The framed left panel (same art the Game Setup list panel uses).
	bgrL, _ := ctx.loadSimple("BGRSCRNL")
	bgrR, _ := ctx.loadSimple("BGRSCRNR")

	oldFont := ctx.Sys.Fonts().GetFont()
	defer ctx.Sys.Fonts().SetFont(oldFont)
	f5 := ctx.Sys.Fonts().Get(fonts.Font5)

	m := &tipsMenu{
		ctx:   ctx,
		pic:   pic,
		bgrL:  bgrL,
		bgrR:  bgrR,
		f5:    f5,
		buf:   ctx.Sys.Buf(mainWindow),
		lineH: f5.Height + 1,
	}
	m.perPage = tipsH / m.lineH
	if m.perPage < 1 {
		m.perPage = 1
	}

	// The tips body. The master source keys this _(e81f); the shipped 0.7.2 data
	// (the port's reference) keys the same keyboard tips text under ceb6.
	m.lines = wrapTips(f5, ctx.text("ceb6", tipsFallback), tipsW-4)

	// Background + caption (reuse the New Game menu's MAINPIC + title).
	ctx.drawBigAt(pic, 0, 0)
	ctx.Plat.SetPalette(&pic.Palette)
	ctx.drawMenuTitle(ctx.text("04b5", "New Game Menu"))
	ctx.setHandCursor()

	buttons := m.buildWidgets()
	defer func() {
		for _, b := range buttons {
			ctx.Sys.DeleteButton(b)
		}
	}()

	m.drawPanel()
	ctx.Sys.MouseShow()

	for {
		key := ctx.Sys.GetInput()
		if key == -1 {
			continue
		}
		if m.handleKey(key) {
			return nil
		}
	}
}

// copyBg restores a MAINPIC region before redrawing paged text.
func (m *tipsMenu) copyBg(x, y, w, h int) {
	pic, buf := m.pic, m.buf

	sx, sy := x-pic.ULX, y-pic.ULY
	if sx < 0 || sy < 0 {
		return
	}
	w = min(w, min(buf.W-x, pic.Buffer().W-sx))
	h = min(h, min(buf.H-y, pic.Buffer().H-sy))
	if w <= 0 || h <= 0 {
		return
	}
	buf.Sub(x, y, w, h).Copy(pic.Buffer().Sub(sx, sy, w, h))
}

// drawPanel repaints the framed panel and the page of tips text inside it.
func (m *tipsMenu) drawPanel() {
	buf, f5 := m.buf, m.f5

	m.ctx.Sys.Lock()
	defer m.ctx.Sys.Unlock()
	m.copyBg(6, 173, 320, 250)
	if m.bgrL != nil {
		m.bgrL.Draw(buf, 6, 173, false)
	}
	if m.bgrR != nil {
		m.bgrR.Draw(buf, 166, 173, false)
	}
	end := min(m.row+m.perPage, len(m.lines))
	for i := m.row; i < end; i++ {
		blitInBox(buf, f5, m.lines[i], tipsX, tipsY+(i-m.row)*m.lineH, tipsW, m.lineH, tipsColor, false)
	}
}

// buildWidgets adds the scroll arrows and the Cancel plate, and returns their ids.
func (m *tipsMenu) buildWidgets() []gnw.ButtonID {
	ctx := m.ctx

	var buttons []gnw.ButtonID

	addImg := func(x int, up, down string, value int) {
		if b, e := ctx.imageButton(mainWindow, x, tipsBtnY, up, down, "", 0, 0, false, value); e == nil {
			ctx.registerButtonSfx(b, "KCARG0")
			buttons = append(buttons, b)
		}
	}
	addImg(16, "MNUUAROU", "MNUUAROD", tipsUp)   // page up
	addImg(48, "MNUDAROU", "MNUDAROD", tipsDown) // page down

	ctx.Sys.Fonts().SetFont(fonts.Font1)
	if b, e := ctx.plateButton(mainWindow, tipsCancelX, tipsCancelBtnY, "MNUBTN4U", "MNUBTN4D", ctx.text("639d", "Cancel"), tipsCancel); e == nil {
		ctx.registerButtonSfx(b, "NCANC0")
		buttons = append(buttons, b)
	}

	return buttons
}

// handleKey answers one input code and reports whether the screen closes.
func (m *tipsMenu) handleKey(key int) bool {
	switch {
	case key == tipsCancel || key == platform.KeyEscape || key == platform.KeyReturn:
		return true
	case key == tipsUp:
		if m.row > 0 {
			m.row -= m.perPage
			if m.row < 0 {
				m.row = 0
			}
			m.drawPanel()
		}
	case key == tipsDown:
		if m.row+m.perPage < len(m.lines) {
			m.row += m.perPage
			m.drawPanel()
		}
	}

	return false
}

// wrapTips wraps tips text while preserving blank-line paragraph breaks.
func wrapTips(f *fonts.Font, text string, w int) []string {
	var lines []string
	for _, para := range strings.Split(text, "\n") {
		if strings.TrimSpace(para) == "" {
			lines = append(lines, "")
			continue
		}
		lines = append(lines, wrapText(f, para, w)...)
	}
	return lines
}

// tipsFallback is shown when the tips resource string is missing (no game data).
const tipsFallback = "Tips are unavailable: the M.A.X. game data could not be read."
