// transfer.go : implements resource transfers between adjacent units.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++: transfermenu.cpp - TransferMenu
package menus

import (
	"fmt"

	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/gnw"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/screendata"
	"github.com/suglasp/max-go/internal/sprites"
)

// XFERPIC moves raw, fuel, or gold between adjacent units or complexes. Its slider shows the target's resulting storage; positive values give and negative values take.
//
// The source/target unit icons (ReportStats_DrawListItemIcon) are rendered by
// the host from the unit sprite cache and passed in as 32x32 buffers. The C++
// LimitedScrollbar is folded into this screen's value math
// as a standalone widget.

// TransferInfo now lives in internal/screendata, shared with the gamescreen host.

// Slider geometry: the transfer bar starts at (sliderX, sliderY) and is sliderW
// by sliderH, which the widget builder, the painter and the click handler share.
const sliderX, sliderY, sliderW, sliderH = 44, 89, 223, 16

// transferMenu is the transfer screen state: the two units' cargo picture, the
// signed amount the slider edits, and the value limits derived from both units'
// capacity. value is what the entry returns on Done.
// C++: transfermenu.cpp - TransferMenu.
type transferMenu struct {
	ctx  *Context
	win  gnw.WinID
	ox   int // window left edge, to map slider clicks
	info screendata.TransferInfo

	value   int // signed: positive = source gives, negative = source takes
	giveMax int
	takeMax int
	// Scrollbar mapping fields: capacity is the full target storage span the
	// slider represents, zeroOffset shifts an empty target to the left edge.
	freeCapacity int
	zeroOffset   int
	capacity     int

	barTag            string
	bar, lfAro, rtAro *gfx.Buffer
	pic               *sprites.Big
	f5                *fonts.Font
}

// RunTransfer runs the XFERPIC dialog for the two units and returns the signed
// amount to move from source to target (positive = source gives, negative =
// source takes), or 0 if the player cancelled. center is the region to centre
// the dialog in.
//
// C++: TransferMenu::TransferMenu + Run (transfermenu.cpp)
func RunTransfer(ctx *Context, info screendata.TransferInfo, center gfx.Rect) (int, error) {
	barTag := map[economy.CargoType]string{
		economy.CargoRaw:  "SMBRRAW",
		economy.CargoFuel: "SMBRFUEL",
		economy.CargoGold: "SMBRGOLD",
	}[info.Cargo]
	if barTag == "" {
		Message(ctx, "These units cannot exchange cargo.")
		return 0, nil
	}

	pic, err := ctx.loadBig("XFERPIC")
	if err != nil {
		return 0, err
	}

	ox := max(center.ULX+(center.Width()+1-pic.W)/2, 0)
	oy := max(center.ULY+(center.Height()+1-pic.H)/2, 0)
	win := ctx.Sys.AddWindow(ox, oy, pic.W, pic.H, 0, gnw.WindowMoveOnTop|gnw.WindowModal)
	if win < 0 {
		return 0, fmt.Errorf("menus: transfer window alloc failed")
	}
	defer ctx.Sys.DeleteWindow(win)

	oldFont := ctx.Sys.Fonts().GetFont()
	defer ctx.Sys.Fonts().SetFont(oldFont)

	m := &transferMenu{
		ctx:    ctx,
		win:    win,
		ox:     ox,
		info:   info,
		barTag: barTag,
		pic:    pic,
		f5:     ctx.Sys.Fonts().Get(fonts.Font5),
	}

	// The slider value ranges over [takeMax .. giveMax] and starts at giveMax.
	m.giveMax, m.takeMax = transferLimits(info)
	m.value = m.giveMax
	m.freeCapacity = info.TargetCapacity - info.TargetMaterials
	m.zeroOffset = -info.TargetMaterials
	m.capacity = m.freeCapacity - m.zeroOffset // == TargetCapacity

	m.loadArt()
	m.drawStaticText()

	buttons := m.buildWidgets()
	defer func() {
		for _, b := range buttons {
			ctx.Sys.DeleteButton(b)
		}
	}()

	m.drawDynamic()
	ctx.Sys.MouseShow()

	for {
		key := ctx.Sys.GetInput()
		if key == -1 {
			continue
		}
		if amount, done := m.handleKey(key); done {
			return amount, nil
		}
	}
}

// loadArt loads the material fill art (right anchored gradient, like
// LoadHorizontalBar) and the give/take direction arrows, keying out each one's
// transparent backdrop.
func (m *transferMenu) loadArt() {
	keyed := func(tag string) *gfx.Buffer {
		img, e := m.ctx.loadSimple(tag)
		if e != nil {
			return nil
		}
		b := img.Buffer().Clone()
		b.KeyToZero(b.At(0, 0))
		return b
	}
	m.bar = keyed(m.barTag)
	m.lfAro = keyed("XFRLFARO") // source takes (value < 0)
	m.rtAro = keyed("XFRRTARO") // source gives (value >= 0)
}

// drawStaticText paints the panel background, the two unit icons and their names.
func (m *transferMenu) drawStaticText() {
	ctx, pic, f5, info := m.ctx, m.pic, m.f5, m.info
	// Static panel + the two unit names (centred, black, FONT_5).
	winBuf := ctx.Sys.Buf(m.win)
	ctx.Sys.Lock()
	winBuf.Sub(0, 0, pic.W, pic.H).Copy(pic.Buffer().Sub(0, 0, pic.W, pic.H))
	// Unit icons in the two wells (32x32, centred at 104,36 / 207,36 like
	// ReportStats_DrawListItemIcon), then the singular names below them.
	blitIcon(winBuf, info.SourceIcon, 88, 20)
	blitIcon(winBuf, info.TargetIcon, 191, 20)
	blitInBox(winBuf, f5, info.SourceName, 10, 52, 110, 30, 0, true)
	blitInBox(winBuf, f5, info.TargetName, 191, 52, 110, 30, 0, true)
	ctx.Sys.Unlock()
}

// transferLimits returns the most the source can give the target (giveMax) and the most it can take back from it
// (takeMax, zero or negative), each bounded by the free room on the receiving side and the stock on the giving side.
//
// C++ ported : transfermenu.cpp - TransferMenu::TransferMenu
func transferLimits(info screendata.TransferInfo) (giveMax, takeMax int) {
	giveMax = min(info.TargetCapacity-info.TargetMaterials, info.SourceMaterials)
	takeMax = -min(info.SourceCapacity-info.SourceMaterials, info.TargetMaterials)

	return giveMax, takeMax
}

// sliderFill returns how many pixels of a width wide slider show the target's level after moving value: zeroOffset is
// the target's stock negated and capacity its full storage. The fill is clamped to the slider.
//
// C++ ported : scrollbar.cpp - LimitedScrollbar::RefreshScreen
func sliderFill(value, zeroOffset, capacity, width int) int {
	fill := 0
	if capacity > 0 {
		fill = (value - zeroOffset) * width / capacity
	}

	return max(0, min(fill, width))
}

// drawDynamic repaints the slider fill, the direction arrow and the three
// running totals (amount / source after / target after).
// C++: Scrollbar::RefreshScreen + TransferMenu::UpdateIndicators.
func (m *transferMenu) drawDynamic() {
	ctx, pic, f5, info, value := m.ctx, m.pic, m.f5, m.info, m.value
	buf := ctx.Sys.Buf(m.win)
	ctx.Sys.Lock()
	defer ctx.Sys.Unlock()

	buf.Sub(sliderX, sliderY, sliderW, sliderH).Copy(pic.Buffer().Sub(sliderX, sliderY, sliderW, sliderH))
	fill := sliderFill(value, m.zeroOffset, m.capacity, sliderW)
	if m.bar != nil && fill > 0 {
		bar := m.bar
		w := min(fill, bar.W)
		h := min(sliderH, bar.H)
		buf.Sub(sliderX, sliderY, w, h).TransCopy(bar.Sub(bar.W-w, 0, w, h)) // right anchored source
	}

	buf.Sub(141, 43, 31, 16).Copy(pic.Buffer().Sub(141, 43, 31, 16))
	if aro := m.rtAro; value < 0 {
		aro = m.lfAro
		if aro != nil {
			buf.Sub(141, 43, min(aro.W, 31), min(aro.H, 16)).TransCopy(aro.Sub(0, 0, min(aro.W, 31), min(aro.H, 16)))
		}
	} else if aro != nil {
		buf.Sub(141, 43, min(aro.W, 31), min(aro.H, 16)).TransCopy(aro.Sub(0, 0, min(aro.W, 31), min(aro.H, 16)))
	}

	drawTransferTotal(buf, pic.Buffer(), f5, fmt.Sprintf("%d", absInt(value)), 141, 15, 29, 20)
	drawTransferTotal(buf, pic.Buffer(), f5, fmt.Sprintf("%d", absInt(info.SourceMaterials-value)), 28, 28, 60, 16)
	drawTransferTotal(buf, pic.Buffer(), f5, fmt.Sprintf("%d", absInt(info.TargetMaterials+value)), 223, 28, 60, 16)
}

// buildWidgets creates the two nudge arrows, the slider click region and the
// bottom Cancel/Help/Done row, returning the ids so the caller can delete them
// on exit.
func (m *transferMenu) buildWidgets() []gnw.ButtonID {
	ctx, win := m.ctx, m.win
	var buttons []gnw.ButtonID

	// Nudge buttons (right +1 / left -1) and the slider click region.
	if rt, e := ctx.imageButton(win, 278, 89, "XFRRGT_U", "XFRRGT_D", "", 0, 0, false, 1000); e == nil {
		ctx.registerButtonSfx(rt, "KCARG0")
		buttons = append(buttons, rt)
	}
	if lf, e := ctx.imageButton(win, 17, 89, "XFRLFT_U", "XFRLFT_D", "", 0, 0, false, 1001); e == nil {
		ctx.registerButtonSfx(lf, "KCARG0")
		buttons = append(buttons, lf)
	}
	if sc := ctx.Sys.AddButton(win, sliderX-3, sliderY, sliderW+6, sliderH,
		gnw.NoValue, gnw.NoValue, gnw.NoValue, 1002, nil, nil, nil, 0); sc >= 0 {
		buttons = append(buttons, sc)
	}

	// Cancel / Help / Done across the bottom. The caption font matches the
	// original (FONT_5); pin it so the menu reads the same opened standalone or
	// in game (the in game current font would otherwise change the size).
	ctx.Sys.Fonts().SetFont(fonts.Font5)
	if c, e := ctx.plateButton(win, 82, 125, "XFRCAN_U", "XFRCAN_D", ctx.text("36f9", "Cancel"), 1098); e == nil {
		ctx.registerButtonSfx(c, "NCANC0")
		buttons = append(buttons, c)
	}
	if h, e := ctx.imageButton(win, 147, 125, "XFRHLP_U", "XFRHLP_D", "", 0, 0, false, 1097); e == nil {
		ctx.registerButtonSfx(h, "NHELP0")
		buttons = append(buttons, h)
	}
	if d, e := ctx.plateButton(win, 174, 125, "XFRDNE_U", "XFRDNE_D", ctx.text("c366", "Done"), 1099); e == nil {
		ctx.registerButtonSfx(d, "NDONE0")
		buttons = append(buttons, d)
	}
	return buttons
}

// handleKey runs one input event of the transfer loop. done reports that the
// screen is finished, with amount the signed quantity to move (0 on Cancel).
func (m *transferMenu) handleKey(key int) (amount int, done bool) {
	ctx := m.ctx
	switch {
	case key == 1099 || key == platform.KeyReturn:
		return m.value, true // commit the signed amount
	case key == 1098 || key == platform.KeyEscape:
		return 0, true // cancel: transfer nothing
	case key == 1097 || key == '?':
		Help(ctx, "TRANSFER_SETUP")
	case key == 1000: // right arrow: source gives one more
		if m.value < m.giveMax {
			m.value++
			m.drawDynamic()
		}
	case key == 1001: // left arrow: source takes one more
		if m.value > m.takeMax {
			m.value--
			m.drawDynamic()
		}
	case key == 1002: // slider click: jump to the clicked fraction
		x := ctx.Plat.Mouse().X - m.ox - sliderX
		nv := m.zeroOffset
		if sliderW > 0 {
			nv = (sliderW/2+m.capacity*x)/sliderW + m.zeroOffset
		}
		nv = max(m.zeroOffset, min(nv, m.freeCapacity))
		nv = max(m.takeMax, min(nv, m.giveMax))
		if nv != m.value {
			m.value = nv
			m.drawDynamic()
		}
	}
	return 0, false
}

// blitIcon draws a prerendered icon buffer transparently at (x, y), clipped to
// dst; a nil icon (sprite missing) leaves the well as the panel painted it.
// drawTransferTotal reprints one running total: restore its background box
// from the panel art, then centre the number in it.
func drawTransferTotal(buf, bg *gfx.Buffer, f5 *fonts.Font, s string, x, y, w, h int) {
	buf.Sub(x, y, w, h).Copy(bg.Sub(x, y, w, h))
	blitInBox(buf, f5, s, x, y, w, h, colorWhite, true)
}

func blitIcon(dst *gfx.Buffer, icon *gfx.Buffer, x, y int) {
	if icon == nil {
		return
	}
	w := min(icon.W, dst.W-x)
	h := min(icon.H, dst.H-y)
	if w <= 0 || h <= 0 {
		return
	}
	dst.Sub(x, y, w, h).TransCopy(icon.Sub(0, 0, w, h))
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
