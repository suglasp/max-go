// cargodraw.go : paints the purchase screen: the two selector lists, the unit
// detail panel and the two vertical worm bars.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++: cargomenu.cpp - AbstractUpgradeMenu::DrawUnitInfo + DrawUnitStats
package menus

import (
	"fmt"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/sprites"
)

// drawStaticText paints the header captions and the chrome yellow separator
// lines under each list title. The title is plain flat green like every menu
// title (no outline). C++: the Text_TextBox + draw_line calls in the CargoMenu ctor.
func (m *cargoMenu) drawStaticText() {
	ctx := m.ctx
	f5 := m.f5
	buf := m.buf

	ctx.Sys.Lock()
	blitInBox(buf, f5, ctx.text("eb71", "Purchase Menu"), 473, 7, 158, 18, colorGreen, true)
	// Purchased (centred tan) + underline.
	blitInBox(buf, f5, ctx.text("5fa1", "Purchased"), 337, 22, 118, 12, cargoTan, true)
	buf.Sub(337, 34, 455-337, 1).Fill(cargoChrome)
	// Available (left tan) + Cost (right yellow) + underline.
	blitInBox(buf, f5, ctx.text("e674", "Available"), 482, 57, 120, 12, cargoTan, false)
	costHdr := ctx.text("056f", "Cost")
	blitInBox(buf, f5, costHdr, 625-f5.Width(costHdr), 57, f5.Width(costHdr), 12, cargoGold, false)
	buf.Sub(482, 69, 625-482, 1).Fill(cargoChrome)
	// "Description" label beside its checkbox, gold shadowed like the C++
	// Text_TextBox(WindowInfo*) default (the checkbox button is added with the widgets).
	blitTextBoxShadowed(buf, f5, ctx.text("57a6", "Description"), 209, 264, 80, 17, fonts.ColorGold, true)
	m.drawBarLabels(buf)
	ctx.Sys.Unlock()
}

// drawRow paints one list row: the icon (centred at x+16), the name in tan at x+35,
// and (purchase list) the gold cost right aligned in yellow. A selected row gets the
// white corner bracket marker, not a fill. C++: ReportStats_DrawListItem +
// PurchaseTypeSelector::Draw (cost in YELLOW).
func (m *cargoMenu) drawRow(x, y, w int, t assets.UnitType, label string, cost int, sel bool) {
	m.sc.DrawIcon(m.buf, t, m.lo.Team, x+16, y+16)
	nameW := w - 35
	if cost > 0 {
		nameW -= 16 // leave room for the right aligned cost column
	}
	lines := wrapTwoLines(m.f5, label, nameW)
	if len(lines) == 1 {
		blitInBox(m.buf, m.f5, lines[0], x+35, y, nameW, cgRowH, cargoTan, false)
	} else {
		top := y + (cgRowH-2*(m.f5.Height+1))/2
		blitInBox(m.buf, m.f5, lines[0], x+35, top, nameW, m.f5.Height+1, cargoTan, false)
		blitInBox(m.buf, m.f5, lines[1], x+35, top+m.f5.Height+1, nameW, m.f5.Height+1, cargoTan, false)
	}
	if cost > 0 {
		s := fmt.Sprintf("%d", cost)
		cw := m.f5.Width(s)
		blitInBox(m.buf, m.f5, s, x+w-4-cw, y, cw, cgRowH, cargoGold, false)
	}
	if sel {
		drawSelMarker(m.buf, x, y, cgRowH)
	}
}

// drawBuyList / drawCargoList repaint the two selectors from their scroll.
func (m *cargoMenu) drawBuyList() {
	m.ctx.Sys.Lock()
	defer m.ctx.Sys.Unlock()
	m.buf.Sub(cgBuyX, cgBuyY, cgBuyW, cgBuyH).Copy(m.pic.Buffer().Sub(cgBuyX, cgBuyY, cgBuyW, cgBuyH))
	for r := 0; r < m.buyRows && m.buyScroll+r < len(cargoRoster); r++ {
		e := cargoRoster[m.buyScroll+r]
		m.drawRow(cgBuyX, cgBuyY+r*cgRowH, cgBuyW, e.t, e.name, cargoUnitCost(m.at, e.t), m.active == cgBuyList && m.buyScroll+r == m.buySel)
	}
}

// drawCargoList repaints the middle Purchased selector from its scroll.
func (m *cargoMenu) drawCargoList() {
	m.ctx.Sys.Lock()
	defer m.ctx.Sys.Unlock()
	m.buf.Sub(cgCargoX, cgCargoY, cgCargoW, cgCargoH).Copy(m.pic.Buffer().Sub(cgCargoX, cgCargoY, cgCargoW, cgCargoH))
	for r := 0; r < m.cargoRows && m.cargoScroll+r < len(m.lo.Units); r++ {
		t := m.lo.Units[m.cargoScroll+r]
		m.drawRow(cgCargoX, cgCargoY+r*cgRowH, cgCargoW, t, rosterName(t), 0, m.active == cgCargoList && m.cargoScroll+r == m.cargoSel)
	}
}

// statIcon caches a keyed gauge icon (I_ARMOR/I_HITS/... keyed transparent).
func (m *cargoMenu) statIcon(tag string) *gfx.Buffer {
	if b, ok := m.statIcons[tag]; ok {
		return b
	}
	var b *gfx.Buffer
	if tag != "" {
		if img, e := m.ctx.loadSimple(tag); e == nil {
			c := img.Buffer().Clone()
			c.KeyToZero(c.At(0, 0))
			b = c
		}
	}
	m.statIcons[tag] = b
	return b
}

// loadPortrait caches the P_* BIG image per roster unit, loaded on first use.
func (m *cargoMenu) loadPortrait(t assets.UnitType) *sprites.Big {
	if b, ok := m.portraitCache[t]; ok {
		return b
	}
	var b *sprites.Big
	if tag := unitPortrait[t]; tag != "" {
		if img, e := m.ctx.loadBig(tag); e == nil {
			b = img
		}
	}
	m.portraitCache[t] = b
	return b
}

// drawDetail repaints the left preview (the unit's large portrait) and the
// stats panel. C++: AbstractUpgradeMenu::DrawUnitInfo + DrawUnitStats.
func (m *cargoMenu) drawDetail() {
	m.ctx.Sys.Lock()
	defer m.ctx.Sys.Unlock()
	m.buf.Sub(cgPrevX, cgPrevY, cgPrevW, cgPrevH).Copy(m.pic.Buffer().Sub(cgPrevX, cgPrevY, cgPrevW, cgPrevH))
	m.buf.Sub(cgStatX, cgStatY, cgStatW, cgStatH).Copy(m.pic.Buffer().Sub(cgStatX, cgStatY, cgStatW, cgStatH))
	// Also clear the stepper / upgrade cost column (283..361) so old steppers,
	// cost numbers and coins don't pile up under the new ones. Stops before the
	// gold bar at x=361, and short of the Cancel/?/Done button row at y=452 so
	// the clear can't erase the Cancel button's top left corner.
	m.buf.Sub(283, cgStatY, 78, 452-cgStatY-2).Copy(m.pic.Buffer().Sub(283, cgStatY, 78, 452-cgStatY-2))
	t, ok := m.selectedType()
	if !ok {
		return
	}
	if big := m.loadPortrait(t); big != nil {
		bw := minInt(big.W, cgPrevW)
		bh := minInt(big.H, cgPrevH)
		m.buf.Sub(cgPrevX, cgPrevY, bw, bh).Copy(big.Buffer().Sub(0, 0, bw, bh))
	} else {
		// No portrait resource: fall back to the unit icon + name.
		m.sc.DrawIcon(m.buf, t, m.lo.Team, cgPrevX+cgPrevW/2, cgPrevY+cgPrevH/2)
		blitInBox(m.buf, m.f5, rosterName(t), cgPrevX, cgPrevY+cgPrevH-24, cgPrevW, 20, colorGreen|fonts.FlagOutline, true)
	}
	// Description checkbox: overlay the unit's description text on the portrait.
	// C++: AbstractUpgradeMenu::DrawUnitInfo draws base_unit.GetDescription().
	// PLACEHOLDER text: the real per unit description lives in the unit data
	// (MAX.RES / lang_english.ini) and is wired with the unit data loader.
	if m.descMode {
		desc := fmt.Sprintf("This is a %s. (unit description text is not wired yet - it comes from the unit data in MAX.RES / lang_english.ini)", rosterName(t))
		for i, ln := range wrapText(m.f5, desc, cgPrevW-20) {
			// C++: Text_TextBox(..., GNW_TEXT_OUTLINE | 0xA2), tan with an outline.
			blitInBox(m.buf, m.f5, ln, cgPrevX+10, cgPrevY+8+i*(m.f5.Height+2), cgPrevW-20, m.f5.Height+2, cargoTan|fonts.FlagOutline, false)
		}
	}
	cv, bv := m.values(t)
	m.upRows = m.upRows[:0]
	if cv != nil {
		// Stat rows in the original's order (see cargoStatRows): each a yellow
		// value, tan label and gauge row with a red separator, valued from cur
		// (the upgraded values). A row marked upg gets a '<' '>' stepper +
		// upgrade cost; non upgradeable rows get no stepper.
		for i, r := range cargoStatRows(m.at, t, cv, bv) {
			ry := cgStatY + 2 + i*19
			if r.value > 0 { // value, yellow, right aligned ending at statX+24
				s := fmt.Sprintf("%d", r.value)
				vw := m.f5.Width(s)
				blitInBox(m.buf, m.f5, s, cgStatX+24-vw, ry, vw, 17, cargoGold, false)
			}
			blitInBox(m.buf, m.f5, r.label, cgStatX+28, ry, 48, 17, cargoTan, false)
			drawGauge(m.buf, m.statIcon(r.icon), r.count, cgStatX+76, ry, cgStatW-76, cgStatX, cgStatX+cgStatW, cgStatY+cgStatH)
			m.buf.Sub(cgStatX+2, ry+17, cgStatW-4, 1).Fill(colorRed) // separator
			// Upgrade stepper on each row (only for upgradeable rows): '<' '>' +
			// the gold upgrade cost. C++: the upgrade_controls / cost_background.
			if r.upgradeable {
				blitIcon(m.buf, m.statIcon("UPGLFT_U"), 283, ry)
				blitIcon(m.buf, m.statIcon("UPGRGT_U"), 301, ry)
				uc := game.UpgradeCost(r.attr, cv.Get(r.attr), 0, bv.Get(r.attr))
				if uc > 0 && uc < game.UpgradeCostLimit {
					cs := fmt.Sprintf("%d", uc)
					blitInBox(m.buf, m.f5, cs, 343-m.f5.Width(cs), ry+3, m.f5.Width(cs), 11, cargoChrome, false)
					blitIcon(m.buf, m.statIcon("I_GOLD"), 345, ry+4)
				}
				m.upRows = append(m.upRows, cargoStepRow{attr: r.attr, y: ry})
			}
		}
	}
	// The stepper column clear clips the caption row (y 283..299); repaint it.
	m.drawBarLabels(m.buf)
}

// fillVBar fills a vertical worm bar (from the bottom up) to fill/max of (x,y,w,h),
// taking the bottom rows of the worm art. C++: LoadVerticalBar.
func (m *cargoMenu) fillVBar(x, y, w, h int, bar *gfx.Buffer, fill, max int) {
	if bar == nil || max <= 0 || fill <= 0 {
		return
	}
	fh := h * fill / max
	if fh > h {
		fh = h
	}
	sw := minInt(w, bar.W)
	sh := minInt(fh, bar.H)
	m.buf.Sub(x+(w-sw)/2, y+h-sh, sw, sh).TransCopy(bar.Sub(0, bar.H-sh, sw, sh))
}

// drawBarLabels paints the static gold shadowed "Cost" / "Credit" / "Cargo"
// captions above the stepper cost column and the two worm bars. Redrawn after
// every bar/detail clear since those clear rects overlap the caption row.
// The caller holds the gnw lock. C++: the three Text_TextBox(&dialog_window,
// ...) calls at y=283 in the CargoMenu constructor.
func (m *cargoMenu) drawBarLabels(buf *gfx.Buffer) {
	blitTextBoxShadowed(buf, m.f5, m.ctx.text("ab7e", "Cost"), 320, 283, 48, 16, fonts.ColorGold, true)
	blitTextBoxShadowed(buf, m.f5, m.ctx.text("3201", "Credit"), 358, 283, 48, 16, fonts.ColorGold, true)
	blitTextBoxShadowed(buf, m.f5, m.ctx.text("fc09", "Cargo"), 409, 283, 48, 16, fonts.ColorGold, true)
}

// drawBars repaints the two vertical worm bars: the team's Credit reserve
// (VERTGOLD, team_gold/start_gold, chrome yellow value) and the selected unit's
// Cargo (VERTRAW/FUEL/GOLD, load/storage, white value like the scrollbar
// readouts), the values sitting above the gold shadowed captions.
// C++: AbstractUpgradeMenu::DrawUnitStats gold bar + the CargoMenu
// LimitedScrollbar amount box (scrollbar.cpp draws its value in 0xFF).
func (m *cargoMenu) drawBars() {
	m.ctx.Sys.Lock()
	defer m.ctx.Sys.Unlock()
	// Credit / gold. Keep the clear within the gold slot (361..401) so it does
	// not eat the upgrade cost coin to its left (~345..358).
	m.buf.Sub(cgGoldX, cgGoldY, cgGoldW, cgGoldH).Copy(m.pic.Buffer().Sub(cgGoldX, cgGoldY, cgGoldW, cgGoldH))
	m.fillVBar(cgGoldX, cgGoldY+25, cgGoldW, cgGoldH-25, m.statIcon("VERTGOLD"), m.lo.TeamGold, m.lo.StartGold)
	blitInBox(m.buf, m.f5, fmt.Sprintf("%d", m.lo.TeamGold), cgGoldX, cgGoldY, cgGoldW, 11, cargoChrome, true)
	// Cargo of the selected Purchased unit (if it carries cargo).
	m.buf.Sub(cgCrgX-12, cgCrgY, cgCrgW+24, cgCrgH).Copy(m.pic.Buffer().Sub(cgCrgX-12, cgCrgY, cgCrgW+24, cgCrgH))
	if m.cargoSel < len(m.lo.Units) {
		ut := m.lo.Units[m.cargoSel]
		st := 0
		if int(ut) < len(m.at) {
			st = m.at[ut].Storage
		}
		if st > 0 {
			tag := "VERTRAW"
			switch economy.CargoTypeOf(ut) {
			case economy.CargoFuel:
				tag = "VERTFUEL"
			case economy.CargoGold:
				tag = "VERTGOLD"
			}
			amt := m.lo.Cargos[m.cargoSel]
			m.fillVBar(cgCrgX, cgCrgY+25, cgCrgW, cgCrgH-25, m.statIcon(tag), amt, st)
			blitInBox(m.buf, m.f5, fmt.Sprintf("%d", amt), cgCrgX, cgCrgY, cgCrgW, 11, colorWhite, true)
		}
	}
	m.drawBarLabels(m.buf)
}
