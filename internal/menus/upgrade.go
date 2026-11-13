// upgrade.go : implements the research-driven unit attribute upgrade screen.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++: upgrademenu.cpp - UpgradeMenu
package menus

import (
	"fmt"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/gnw"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/sprites"
	unitspr "github.com/suglasp/max-go/internal/units"
)

// UPGRADE lets the player buy permanent per-attribute upgrades with team gold. Done commits changes; Cancel
// or Escape restores values and gold. It is centred under the game palette.
//
// C++: upgrademenu.cpp/abstractupgrademenu.cpp - UpgradeMenu.

// upgradeSnap captures a unit's working values at menu open, for Cancel/ESC
// rollback and the per attribute sell back floor.
type upgradeSnap struct {
	u   *game.UpgradeUnit
	cur *game.UnitValues
}

// upgradeFilter is the live state of the five category toggle buttons. The rest
// states match the original statics: ground/air/sea/building shown, combat off.
// C++: AbstractUpgradeMenu::button_*_rest_state.
type upgradeFilter struct{ ground, air, sea, building, combat bool }

// Panel geometry: the unit selector list (a 32px icon + name row per unit, the
// original UnitTypeSelector), the stats panel, the cost column, the gold slot,
// and the stepper arrow strip. C++: ReportStats_DrawListItem rects.
const (
	ugListX, ugListY, ugListW, ugListH, ugRowH = 482, 80, 143, 290, 32

	ugStatsX, ugStatsY, ugStatsW, ugStatsH = 11, 293, 250, 174
	ugCostX                                = 321
	ugGoldX, ugGoldY, ugGoldW, ugGoldH     = 366, 276, 40, 140 // UPGRADE gold slot opening is x~366..385
	ugArrowsX                              = 283
)

// upgradeMenu is the Upgrade screen state: the unit set with its rollback
// snapshots, the filtered view, and the art/caches the draw methods share.
// C++: upgrademenu.cpp - UpgradeMenu (abstractupgrademenu.cpp - AbstractUpgradeMenu).
type upgradeMenu struct {
	ctx       *Context
	units     []*game.UpgradeUnit
	gold      *int
	startGold int
	snaps     []upgradeSnap

	filter  upgradeFilter
	visible []*game.UpgradeUnit // filtered view of units
	sel     int
	scroll  int

	win gnw.WinID
	oy  int // window top edge, to map mouse clicks into list rows

	pic     *sprites.Big
	f5      *fonts.Font
	goldImg *sprites.Simple
	vertBar *gfx.Buffer
	visRows int

	portraits map[assets.UnitType]*gfx.Buffer
	sc        *unitspr.SpriteCache

	rowButtons []gnw.ButtonID // dynamic per selection arrow buttons

}

// RunUpgrade runs the Upgrade screen over the given unit types, spending from
// *gold. It edits each unit's Cur in place and *gold live, restoring both on
// Cancel/ESC, and reports whether the player committed (Done).
//
// C++: AbstractUpgradeMenu::Run (abstractupgrademenu.cpp)
func RunUpgrade(ctx *Context, units []*game.UpgradeUnit, gold *int, center gfx.Rect) (committed bool, err error) {
	if len(units) == 0 {
		Message(ctx, "No upgradable units.")
		return false, nil
	}
	pic, err := ctx.loadBig("UPGRADE")
	if err != nil {
		return false, err
	}

	ox := max(center.ULX+(center.Width()+1-pic.W)/2, 0)
	oy := max(center.ULY+(center.Height()+1-pic.H)/2, 0)
	win := ctx.Sys.AddWindow(ox, oy, pic.W, pic.H, 0, gnw.WindowMoveOnTop|gnw.WindowModal)
	if win < 0 {
		return false, fmt.Errorf("menus: upgrade window alloc failed")
	}
	defer ctx.Sys.DeleteWindow(win)

	oldFont := ctx.Sys.Fonts().GetFont()
	defer ctx.Sys.Fonts().SetFont(oldFont)
	// Select the menu font up front: plate button captions render with the GNW
	// global current font at creation time, so this must precede any button
	// creation: otherwise an in game caller's larger font leaks into the labels.
	ctx.Sys.Fonts().SetFont(fonts.Font5)
	f5 := ctx.Sys.Fonts().Get(fonts.Font5)

	// Snapshot Cur values + gold so Cancel/ESC can roll the purchases back.
	startGold := *gold
	snaps := make([]upgradeSnap, len(units))
	for i, u := range units {
		snaps[i] = upgradeSnap{u, u.Cur.Clone()}
	}

	m := &upgradeMenu{
		ctx:       ctx,
		units:     units,
		gold:      gold,
		startGold: startGold,
		snaps:     snaps,
		filter:    upgradeFilter{ground: true, air: true, sea: true, building: true, combat: false},
		win:       win,
		oy:        oy,
		pic:       pic,
		f5:        f5,
		visRows:   ugListH / ugRowH,
		portraits: map[assets.UnitType]*gfx.Buffer{},
		sc:        unitspr.NewSpriteCache(ctx.Lib, &pic.Palette),
	}

	buttons := m.buildWidgets()
	defer func() {
		for _, b := range buttons {
			ctx.Sys.DeleteButton(b)
		}
	}()

	// State driving the view: the filtered list plus the per row upgrade arrow
	// buttons (rebuilt whenever the selection changes, since which attributes a
	// unit has varies by type).
	m.visible = filterUnits(units, m.filter)
	defer m.clearRowButtons()

	// I_GOLD icon for the cost column (optional; nil when missing).
	const goldIcon = "I_GOLD"
	m.goldImg, _ = ctx.loadSimple(goldIcon)
	// VERTGOLD is the vertical gold reserve bar art; key its transparent corner.
	if vg, e := ctx.loadSimple("VERTGOLD"); e == nil {
		m.vertBar = vg.Buffer().Clone()
		m.vertBar.KeyToZero(m.vertBar.At(0, 0))
	}

	m.drawList()
	m.drawDetail()
	ctx.Sys.MouseShow()

	for {
		key := ctx.Sys.GetInput()
		if key == -1 {
			continue
		}
		if done, commit := m.handleKey(key); done {
			return commit, nil
		}
	}
}

// buildWidgets paints the static panel and captions and creates every button and
// click region of the Upgrade screen, returning the ids for the caller to free.
// C++: the Text_TextBox + Button setup in the UpgradeMenu ctor (upgrademenu.cpp).
func (m *upgradeMenu) buildWidgets() []gnw.ButtonID {
	ctx := m.ctx
	win := m.win
	pic := m.pic
	f5 := m.f5

	// Static panel + title (plain flat green) + gold shadowed captions,
	// matching the C++ Text_TextBox styles (upgrademenu.cpp ctor).
	winBuf := ctx.Sys.Buf(win)
	ctx.Sys.Lock()
	winBuf.Sub(0, 0, pic.W, pic.H).Copy(pic.Buffer().Sub(0, 0, pic.W, pic.H))
	blitInBox(winBuf, f5, ctx.text("0b85", "Upgrades"), 327, 7, 158, 18, colorGreen, true)
	blitTextBoxShadowed(winBuf, f5, ctx.text("7a2c", "Description"), 209, 264, 80, 17, fonts.ColorGold, true)
	// Unit list header + orange separator above the selector, matching the cargo
	// screen's "Available" header band. C++: AbstractUpgradeMenu list area.
	blitInBox(winBuf, f5, ctx.text("a8b0", "Units"), 482, 57, 143, 12, cargoTan, false)
	winBuf.Sub(482, 69, 625-482, 1).Fill(cargoChrome)
	// (The "Cost"/"Credit" captions + the reserve value are drawn over the gold
	// bar in drawDetail, whose clears overlap their row.)
	ctx.Sys.Unlock()

	var buttons []gnw.ButtonID

	// Scroll buttons (BLDUP_ / BLDDWN_) and bottom Cancel/Help/Done. The up button
	// sits 1px above the down; the pair is nudged +1 down (as on the cargo screen).
	if up, e := ctx.imageButton(win, 471, 387, "BLDUP__U", "BLDUP__D", "", 0, 0, false, 1100); e == nil {
		ctx.registerButtonSfx(up, "KCARG0")
		buttons = append(buttons, up)
	}
	if dn, e := ctx.imageButton(win, 491, 388, "BLDDWN_U", "BLDDWN_D", "", 0, 0, false, 1101); e == nil {
		ctx.registerButtonSfx(dn, "KCARG0")
		buttons = append(buttons, dn)
	}
	// Invisible click region over the unit list so a bare row click registers
	// (a click on the full screen window produces no input otherwise).
	// C++: the UnitTypeSelector clickable widget.
	if b := ctx.Sys.AddButton(win, ugListX, ugListY, ugListW, ugListH,
		gnw.NoValue, gnw.NoValue, gnw.NoValue, 1300, nil, nil, nil, 0); b >= 0 {
		buttons = append(buttons, b)
	}
	mkPlate := func(x int, up, down, label string, value int, sfx string) {
		if b, e := ctx.plateButton(win, x, 452, up, down, label, value); e == nil {
			ctx.registerButtonSfx(b, sfx)
			buttons = append(buttons, b)
		}
	}
	mkPlate(357, "BLDCAN_U", "BLDCAN_D", ctx.text("c6aa", "Cancel"), 1002, "NCANC0")
	// Description checkbox (sticky) next to the "Description" label.
	if b, e := ctx.imageButton(win, 292, 264, "BLDDES_U", "BLDDES_D", "", 0, 0, true, 1200); e == nil {
		ctx.Sys.SetButtonToggle(b)                    // a real toggle: a second click releases it
		ctx.Sys.SetRestState(b, unitDescriptionShown) // shared by the build and upgrade menus, starts on
		ctx.registerButtonSfx(b, "KCARG0")
		buttons = append(buttons, b)
	}
	if h, e := ctx.imageButton(win, 420, 452, "BLDHLP_U", "BLDHLP_D", "", 0, 0, false, 1001); e == nil {
		ctx.registerButtonSfx(h, "NHELP0")
		buttons = append(buttons, h)
	}
	mkPlate(447, "BLDONE_U", "BLDONE_D", ctx.text("bd89", "Done"), 1000, "NDONE0")

	// Category filter toggles (sticky) along y=411.
	filts := []upgradeFilterButton{
		{467, "SELGRD_U", "SELGRD_D", 1003, 1004, &m.filter.ground},
		{500, "SELAIR_U", "SELAIR_D", 1005, 1006, &m.filter.air},
		{533, "SELSEA_U", "SELSEA_D", 1007, 1008, &m.filter.sea},
		{565, "SELBLD_U", "SELBLD_D", 1009, 1010, &m.filter.building},
		{598, "SELCBT_U", "SELCBT_D", 1011, 1012, &m.filter.combat},
	}
	for _, fb := range filts {
		// Plain (not sticky) buttons: toggling is handled in the loop so the
		// visible list rebuilds; the rest face reflects the filter state.
		if b, e := ctx.imageButton(win, fb.x, 411, fb.up, fb.down, "", 0, 0, false, fb.on); e == nil {
			ctx.registerButtonSfx(b, "KCARG0")
			buttons = append(buttons, b)
		}
	}

	return buttons
}

// handleKey runs one input event of the Upgrade screen. It reports done once the
// player leaves, and whether the purchases were committed (Done) or rolled back.
// C++: the AbstractUpgradeMenu::Run event switch (abstractupgrademenu.cpp).
func (m *upgradeMenu) handleKey(key int) (done, commit bool) {
	ctx := m.ctx
	gold := m.gold
	snaps := m.snaps

	switch {
	case key == 1000 || key == platform.KeyReturn: // Done: commit (live edits stand)
		return true, true
	case key == 1002 || key == platform.KeyEscape: // Cancel: roll back
		*gold = m.startGold
		for _, s := range snaps {
			copyValues(s.u.Cur, s.cur)
		}
		return true, false
	case key == 1001 || key == '?':
		Help(ctx, "UPGRADES_SETUP")
	case key == 1200: // Description checkbox: toggle the portrait text overlay
		unitDescriptionShown = !unitDescriptionShown // C++: button_description_rest_state is one static for all menus
		m.drawDetail()
	case key == 1100: // scroll up
		if m.scroll > 0 {
			m.scroll--
			m.drawList()
		}
	case key == 1101: // scroll down
		if m.scroll+m.visRows < len(m.visible) {
			m.scroll++
			m.drawList()
		}
	case key == 1003, key == 1004:
		m.filter.ground = !m.filter.ground
		m.rebuild()
	case key == 1005, key == 1006:
		m.filter.air = !m.filter.air
		m.rebuild()
	case key == 1007, key == 1008:
		m.filter.sea = !m.filter.sea
		m.rebuild()
	case key == 1009, key == 1010:
		m.filter.building = !m.filter.building
		m.rebuild()
	case key == 1011, key == 1012:
		m.filter.combat = !m.filter.combat
		m.rebuild()
	case key >= 2000 && key < 2000+int(game.AttribCount): // left arrow: sell back a step
		upgradeStep(m.visible, m.sel, key-2000, gold, -1, snaps)
		m.drawDetail()
	case key >= 2100 && key < 2100+int(game.AttribCount): // right arrow: buy a step
		upgradeStep(m.visible, m.sel, key-2100, gold, +1, snaps)
		m.drawDetail()
	case key == 1300: // click in the unit list selects a row
		if r := (ctx.Plat.Mouse().Y - m.oy - ugListY) / ugRowH; r >= 0 && m.scroll+r < len(m.visible) {
			// Picking a row is audible, even when it was already the selected one.
			// C++: UnitTypeSelector::Select plays KCARG0 (unittypeselector.cpp);
			// AbstractUpgradeMenu drives its unit list through that same selector.
			ctx.playSfx()
			m.sel = m.scroll + r
			m.drawList()
			m.drawDetail()
		}
	}

	return false, false
}

// clearRowButtons deletes the dynamic per row stepper arrows before a redraw.
func (m *upgradeMenu) clearRowButtons() {
	for _, b := range m.rowButtons {
		m.ctx.Sys.DeleteButton(b)
	}
	m.rowButtons = m.rowButtons[:0]
}

// loadPortrait caches the selected unit's large P_* image for the top left
// panel, loaded on first use. C++: AbstractUpgradeMenu::DrawUnitInfo.
func (m *upgradeMenu) loadPortrait(t assets.UnitType) *gfx.Buffer {
	if b, ok := m.portraits[t]; ok {
		return b
	}
	var b *gfx.Buffer
	if img := m.ctx.loadPortrait(t); img != nil {
		b = img.Buffer().Clone() // the upgrade screen keeps its own copy
	}
	m.portraits[t] = b
	return b
}

// drawList paints the right hand unit selector: a 32px icon + name row per
// unit (the original UnitTypeSelector), the selected row marked with the
// corner bracket selector. C++: ReportStats_DrawListItem.
func (m *upgradeMenu) drawList() {
	buf := m.ctx.Sys.Buf(m.win)
	m.ctx.Sys.Lock()
	defer m.ctx.Sys.Unlock()
	buf.Sub(ugListX, ugListY, ugListW, ugListH).Copy(m.pic.Buffer().Sub(ugListX, ugListY, ugListW, ugListH))
	for r := 0; r < m.visRows && m.scroll+r < len(m.visible); r++ {
		i := m.scroll + r
		y := ugListY + r*ugRowH
		m.sc.DrawIcon(buf, m.visible[i].Type, 0, ugListX+16, y+16)
		lines := wrapTwoLines(m.f5, m.visible[i].Name, ugListW-35)
		if len(lines) == 1 {
			blitInBox(buf, m.f5, lines[0], ugListX+35, y, ugListW-35, ugRowH, cargoTan, false)
		} else {
			top := y + (ugRowH-2*(m.f5.Height+1))/2
			blitInBox(buf, m.f5, lines[0], ugListX+35, top, ugListW-35, m.f5.Height+1, cargoTan, false)
			blitInBox(buf, m.f5, lines[1], ugListX+35, top+m.f5.Height+1, ugListW-35, m.f5.Height+1, cargoTan, false)
		}
		if i == m.sel {
			drawSelMarker(buf, ugListX, y, ugRowH)
		}
	}
}

// drawDetail paints the stats panel, per attribute upgrade rows (arrows +
// cost), and the gold reserve bar for the currently selected unit.
func (m *upgradeMenu) drawDetail() {
	m.clearRowButtons()
	buf := m.ctx.Sys.Buf(m.win)
	m.ctx.Sys.Lock()
	// Clear the stats/arrow/cost/gold panels back to the frame art (the arrow
	// strip too, erasing stale arrows from a previous longer unit). The clear
	// height stops short of the button row at y=452 so it never touches the
	// Cancel button's left edge, which the arrow column overlaps.
	// C++: the stats_/button_/cost_/gold_background Write calls.
	const clearH = 452 - ugStatsY - 2 // arrow/cost column: stop above the button row
	buf.Sub(ugStatsX, ugStatsY, ugStatsW, ugStatsH).Copy(m.pic.Buffer().Sub(ugStatsX, ugStatsY, ugStatsW, ugStatsH))
	buf.Sub(ugArrowsX, ugStatsY, ugCostX+ugGoldW-ugArrowsX, clearH).Copy(m.pic.Buffer().Sub(ugArrowsX, ugStatsY, ugCostX+ugGoldW-ugArrowsX, clearH))
	buf.Sub(358, ugGoldY, ugGoldW+10, ugGoldH).Copy(m.pic.Buffer().Sub(358, ugGoldY, ugGoldW+10, ugGoldH))

	var stepRows []upgradeStepRow
	var u *game.UpgradeUnit
	if m.sel < len(m.visible) {
		u = m.visible[m.sel]
	}
	m.drawPortrait(buf, u)
	if u != nil {
		for i, r := range upgradeStatRows(u) {
			ry := ugStatsY + 2 + i*19
			if r.value > 0 {
				s := fmt.Sprintf("%d", r.value)
				blitInBox(buf, m.f5, s, ugStatsX+24-m.f5.Width(s), ry, m.f5.Width(s), 17, cargoGold, false)
			}
			blitInBox(buf, m.f5, r.label, ugStatsX+28, ry, 48, 17, cargoTan, false)
			drawGauge(buf, m.ctx.statIcon(r.icon), r.count, ugStatsX+76, ry, ugStatsW-76, ugStatsX, ugStatsX+ugStatsW, ugStatsY+ugStatsH)
			buf.Sub(ugStatsX+2, ry+17, ugStatsW-4, 1).Fill(colorRed)
			if r.upgradeable {
				cost := game.UpgradeCost(r.attr, u.Cur.Get(r.attr), u.ResearchFactor[r.attr], u.Base.Get(r.attr))
				if cost > 0 && cost < game.UpgradeCostLimit {
					blitRight(buf, m.f5, fmt.Sprintf("%d", cost), 334, ry+4, cargoChrome)
					if m.goldImg != nil {
						m.goldImg.Draw(buf, 336, ry+3, true)
					}
				}
				stepRows = append(stepRows, upgradeStepRow{upgradeAttribIndex(u, r.attr), ry})
			}
		}
	}

	// Gold reserve bar: VERTGOLD art filled proportionally from the bottom
	// (C++ LoadVerticalBar). Falls back to a flat fill if the art is missing.
	if m.startGold > 0 {
		h := (ugGoldH - 25) * *m.gold / m.startGold
		if h > ugGoldH-25 {
			h = ugGoldH - 25
		}
		if h > 0 {
			if m.vertBar != nil {
				sw := min(ugGoldW, m.vertBar.W)
				sh := min(h, m.vertBar.H)
				buf.Sub(ugGoldX+5, ugGoldY+ugGoldH-sh, sw, sh).TransCopy(m.vertBar.Sub(0, m.vertBar.H-sh, sw, sh))
			} else {
				buf.Sub(ugGoldX, ugGoldY+25+(ugGoldH-25-h), ugGoldW, h).Fill(byte(optGold))
			}
		}
	}
	m.ctx.Sys.Unlock()

	// Add a left/right arrow pair on each upgradeable stat row (after unlocking,
	// since imageButton takes the lock itself). The button value carries the
	// attribute's index in u.Attribs, so the 2000+/2100+ handlers are unchanged.
	for _, sr := range stepRows {
		if l, e := m.ctx.imageButton(m.win, ugArrowsX, sr.y, "UPGLFT_U", "UPGLFT_D", "", 0, 0, false, 2000+sr.attribIndex); e == nil {
			m.ctx.registerButtonSfx(l, "KCARG0")
			m.rowButtons = append(m.rowButtons, l)
		}
		if r, e := m.ctx.imageButton(m.win, ugArrowsX+18, sr.y, "UPGRGT_U", "UPGRGT_D", "", 0, 0, false, 2100+sr.attribIndex); e == nil {
			m.ctx.registerButtonSfx(r, "KCARG0")
			m.rowButtons = append(m.rowButtons, r)
		}
	}

	// Gold reserve value (chrome yellow, centred on the C++ gold slot 361..401)
	// above the gold shadowed "Cost" / "Credit" captions; redrawn here because
	// the panel clears above overlap their row.
	// C++: AbstractUpgradeMenu::DrawUnitStats (COLOR_CHROME_YELLOW) + the
	// Text_TextBox(&dialog_window, ...) caption calls (upgrademenu.cpp).
	buf2 := m.ctx.Sys.Buf(m.win)
	m.ctx.Sys.Lock()
	blitInBox(buf2, m.f5, fmt.Sprintf("%d", *m.gold), 361, ugGoldY, 40, 11, cargoChrome, true)
	blitTextBoxShadowed(buf2, m.f5, m.ctx.text("f199", "Cost"), 320, 283, 48, 16, fonts.ColorGold, true)
	blitTextBoxShadowed(buf2, m.f5, m.ctx.text("fb18", "Credit"), 358, 284, 48, 17, fonts.ColorGold, true)
	m.ctx.Sys.Unlock()
}

// drawPortrait paints the top left panel for the selected unit: the frame art,
// its P_* image, and the description overlay when the checkbox is set. The
// caller holds the lock. C++: AbstractUpgradeMenu::DrawUnitInfo.
func (m *upgradeMenu) drawPortrait(buf *gfx.Buffer, u *game.UpgradeUnit) {
	// Portrait (top left panel, 11,13,300x240): the selected unit's P_* image.
	buf.Sub(11, 13, 300, 240).Copy(m.pic.Buffer().Sub(11, 13, 300, 240))
	if u != nil {
		if big := m.loadPortrait(u.Type); big != nil {
			bw, bh := min(big.W, 300), min(big.H, 240)
			buf.Sub(11, 13, bw, bh).Copy(big.Sub(0, 0, bw, bh))
		}
		// Description checkbox: overlay the unit's description over the portrait.
		// C++: AbstractUpgradeMenu::DrawUnitInfo (GNW_TEXT_OUTLINE|0xA2).
		if desc := m.ctx.unitDescription(u.Type); desc != "" {
			for i, ln := range wrapLines(m.f5, desc, 280, 0) {
				blitInBox(buf, m.f5, ln, 21, 21+i*(m.f5.Height+2), 280, m.f5.Height+2, cargoTan|fonts.FlagOutline, false)
			}
		}
	}
}

// rebuild refilters the visible list after a category toggle and repaints.
func (m *upgradeMenu) rebuild() {
	m.visible = filterUnits(m.units, m.filter)
	if m.sel >= len(m.visible) {
		m.sel = 0
	}
	if m.scroll > len(m.visible)-1 {
		m.scroll = 0
	}
	m.drawList()
	m.drawDetail()
}

// upgradeFilterButton is one category toggle on the filter strip: art tags, the
// on/off key values, and the filter flag it drives.
type upgradeFilterButton struct {
	x        int
	up, down string
	on, off  int
	state    *bool
}

// upgradeStepRow marks a stat panel row that takes the left/right stepper
// arrows: the attribute's index in u.Attribs (the button value offset) and the
// row's y position.
type upgradeStepRow struct {
	attribIndex int
	y           int
}

// The stat panel row type is menus.cargoStatRow (statrows.go), shared with the
// purchase screen: label, backing attribute, value, gauge count, icon, and the
// upgradeable flag.

// upgradeAttribIndex returns the index of attribute a in u.Attribs, or -1 when
// the unit does not carry it as an upgradable attribute.
func upgradeAttribIndex(u *game.UpgradeUnit, a game.Attrib) int {
	for i, x := range u.Attribs {
		if x == a {
			return i
		}
	}
	return -1
}

// upgradeRowFor makes one stat row; it is upgradeable when the unit carries
// attribute a in its upgradable set.
func upgradeRowFor(u *game.UpgradeUnit, label string, a game.Attrib, value, count int, icon string) cargoStatRow {
	return cargoStatRow{label, a, value, count, icon, upgradeAttribIndex(u, a) >= 0}
}

// upgradeStatRows builds the stat panel rows for unit u in the original
// UnitStats_DrawStats order, values from the unit's current (upgraded) values.
// Rows whose attribute is upgradeable get a '<' '>' stepper + upgrade cost.
func upgradeStatRows(u *game.UpgradeUnit) []cargoStatRow {
	cur, base := u.Cur, u.Base
	cargoIcon := "I_RAW"
	switch economy.CargoTypeOf(u.Type) {
	case economy.CargoFuel:
		cargoIcon = "I_FUEL"
	case economy.CargoGold:
		cargoIcon = "I_GOLD"
	}

	var rows []cargoStatRow
	if base.Get(game.AttribAttack) > 0 {
		v := cur.Get(game.AttribAttack)
		rows = append(rows, upgradeRowFor(u, "Attack", game.AttribAttack, v, v, "I_HRDATK"))
		if base.Get(game.AttribRounds) > 0 {
			sv := min(cur.Get(game.AttribRounds), cur.Get(game.AttribAmmo))
			rows = append(rows, upgradeRowFor(u, "Shots", game.AttribRounds, sv, sv, "I_SHOTS"))
		}
		if base.Get(game.AttribRange) > 0 {
			v := cur.Get(game.AttribRange)
			rows = append(rows, upgradeRowFor(u, "Range", game.AttribRange, v, v, "I_RANGE"))
		}
		if base.Get(game.AttribAmmo) > 0 {
			v := cur.Get(game.AttribAmmo)
			rows = append(rows, upgradeRowFor(u, "Ammo", game.AttribAmmo, v, v, "I_AMMO"))
		}
	}
	if base.Get(game.AttribStorage) > 0 {
		v := cur.Get(game.AttribStorage)
		rows = append(rows, upgradeRowFor(u, "Cargo", game.AttribStorage, v, v/10, cargoIcon))
	}
	if raw := economy.RawConsumptionRate(u.Type, 1); raw > 0 {
		rows = append(rows, cargoStatRow{"Uses", game.AttribCount, 0, raw, "I_RAW", false})
	}
	if base.Get(game.AttribArmor) > 0 {
		v := cur.Get(game.AttribArmor)
		rows = append(rows, upgradeRowFor(u, "Armor", game.AttribArmor, v, v, "I_ARMOR"))
	}
	if base.Get(game.AttribHits) > 0 {
		v := cur.Get(game.AttribHits)
		rows = append(rows, upgradeRowFor(u, "Hits", game.AttribHits, v, v, "I_HITS"))
	}
	if base.Get(game.AttribScan) > 0 {
		v := cur.Get(game.AttribScan)
		rows = append(rows, upgradeRowFor(u, "Scan", game.AttribScan, v, v, "I_SCAN"))
	}
	if base.Get(game.AttribSpeed) > 0 {
		v := cur.Get(game.AttribSpeed)
		rows = append(rows, upgradeRowFor(u, "Speed", game.AttribSpeed, v, v, "I_SPEED"))
	}
	costVal := economy.RawConsumptionRate(assets.LandPlt, 1) * cur.Get(game.AttribTurns)
	rows = append(rows, cargoStatRow{"Cost", game.AttribCount, costVal, costVal, "I_GOLD", false})

	return rows
}

// upgradeStep applies one buy (+1) or sell back (-1) of attribute index ai for
// the selected unit, respecting the gold reserve, the per step cost limit, and
// the floor (the value captured at menu open, purchases made earlier in this
// session can be undone, but upgrades from before this session cannot).
func upgradeStep(visible []*game.UpgradeUnit, sel, ai int, gold *int, dir int, snaps []upgradeSnap) {
	if sel >= len(visible) {
		return
	}
	u := visible[sel]
	if ai < 0 || ai >= len(u.Attribs) {
		return
	}
	a := u.Attribs[ai]
	base := u.Base.Get(a)
	rf := u.ResearchFactor[a]
	cur := u.Cur.Get(a)
	step := game.UpgradeStep(a, base)
	if dir > 0 {
		cost := game.UpgradeCost(a, cur, rf, base)
		if cost >= game.UpgradeCostLimit || cost > *gold {
			return
		}
		u.Cur.Set(a, cur+step)
		*gold -= cost
	} else {
		floor := floorValue(snaps, u, a)
		if cur-step < floor {
			return
		}
		refund := game.UpgradeCost(a, cur-step, rf, base)
		u.Cur.Set(a, cur-step)
		*gold += refund
	}
}

// floorValue returns the value attribute a had when the menu opened for unit u
// (the lowest the player may sell back to).
func floorValue(snaps []upgradeSnap, u *game.UpgradeUnit, a game.Attrib) int {
	for _, s := range snaps {
		if s.u == u {
			return s.cur.Get(a)
		}
	}
	return u.Cur.Get(a)
}

// filterUnits returns the units passing the category filter, mirroring
// AbstractUpgradeMenu::IsUnitFiltered (the mobility class must be enabled, and
// when combat only is on the type must have an attack value).
func filterUnits(units []*game.UpgradeUnit, f upgradeFilter) []*game.UpgradeUnit {
	var out []*game.UpgradeUnit
	for _, u := range units {
		if (u.Ground && f.ground) || (u.Air && f.air) || (u.Sea && f.sea) || (u.Building && f.building) {
			if !f.combat || u.Combat {
				out = append(out, u)
			}
		}
	}
	return out
}

// copyValues overwrites dst's attributes from src (used to roll back on Cancel).
func copyValues(dst, src *game.UnitValues) {
	for a := game.Attrib(0); a < game.AttribCount; a++ {
		dst.Set(a, src.Get(a))
	}
}
