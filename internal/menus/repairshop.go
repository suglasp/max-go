// repairshop.go : implements the repair and upgrade queue for service buildings.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++: repairshopmenu.cpp - RepairShopMenu
package menus

import (
	"fmt"

	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/gnw"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/repairshopui"
	"github.com/suglasp/max-go/internal/sprites"
)

// Repair Shop lists a service building's friendly mobile units with Activate, Reload, Repair, and Upgrade actions. The right column repeats actions for all units and shows complex raw material.
//
// Like Save/Load (LOADPIC) this is a full screen menu drawn into the main window
// with the background image's own palette; the host opens it through
// openMenuScreen(fullscreen=true) so the live game frame and palette are
// restored on return. The sim side (which units, complex raw, the reload/repair/
// upgrade operations) is resolved by the host through RepairShopOps, keeping this
// package free of the build network/units dependency; the same split the
// Transfer and Save/Load screens use.

// The screen's data contract (the two DTOs + the backend interface) lives in the
// shared repairshopui package so the gamescreen, which resolves the sim state, and
// this menus package, which draws it, use one definition. They are aliased here so
// the screen code keeps its short local names.
type (
	RepairUnitInfo = repairshopui.RepairUnitInfo
	RepairShopInfo = repairshopui.RepairShopInfo
	RepairShopOps  = repairshopui.RepairShopOps
)

// Button value scheme. Per slot buttons encode the on screen slot position p and
// the action a as rsSlotBase + p*4 + a; the right column buttons are fixed.
const rsSlotBase = 1400

const (
	rsAct = iota // per slot action offsets
	rsRel
	rsRep
	rsUpg
)

const (
	rsActivateAll = 1500 + iota
	rsReloadAll
	rsRepairAll
	rsUpgradeAll
	rsDone
	rsUpArrow
	rsDownArrow
	rsHelp
)

// rsSlotButtons are the four per slot buttons on the repair shop screen.
type rsSlotButtons struct{ activate, reload, repair, upgrade gnw.ButtonID }

// repairShopMenu is the Repair Shop screen state: the sim snapshot, the paging
// index, and the button handles the refresh methods drive.
// C++: repairshopmenu.cpp - RepairShopMenu.
type repairShopMenu struct {
	ctx  *Context
	ops  RepairShopOps
	info RepairShopInfo

	slotIndex      int
	slotsPerScreen int

	pic    *sprites.Big
	barBuf *gfx.Buffer
	f5     *fonts.Font

	slots                          []rsSlotButtons
	actAll, relAll, repAll, upgAll gnw.ButtonID
	upArrow, downArrow             gnw.ButtonID
}

// RunRepairShop runs the Repair Shop screen until the player presses Done/ESC or
// triggers an Activate (which deploys a unit and closes the screen in vanilla).
//
// C++: RepairShopMenu::RepairShopMenu + Run (repairshopmenu.cpp)
func RunRepairShop(ctx *Context, ops RepairShopOps) error {
	info := ops.Snapshot()

	frameTag := "DEPOTFRM"
	if info.IsHangar {
		frameTag = "HANGRFRM"
	}
	pic, err := ctx.loadBig(frameTag)
	if err != nil {
		return err
	}
	bar, _ := ctx.loadSimple("VERTRAW")
	var barBuf *gfx.Buffer
	if bar != nil {
		barBuf = bar.Buffer().Clone()
		barBuf.KeyToZero(barBuf.At(0, 0))
	}

	ctx.Plat.SetPalette(&pic.Palette) // the frame carries its own palette

	oldFont := ctx.Sys.Fonts().GetFont()
	ctx.Sys.Fonts().SetFont(fonts.Font5)
	defer ctx.Sys.Fonts().SetFont(oldFont)

	m := &repairShopMenu{
		ctx:            ctx,
		ops:            ops,
		info:           info,
		slotsPerScreen: 6,
		pic:            pic,
		barBuf:         barBuf,
		f5:             ctx.Sys.Fonts().Get(fonts.Font5),
		actAll:         -1, relAll: -1, repAll: -1, upgAll: -1,
		upArrow: -1, downArrow: -1,
	}
	if info.IsHangar {
		m.slotsPerScreen = 4
	}

	// Paint the static background.
	ctx.drawBigAt(mainWindow, pic, 0, 0)

	buttons := m.buildWidgets()
	defer func() {
		for _, b := range buttons {
			ctx.Sys.DeleteButton(b)
		}
	}()

	m.refresh()
	ctx.Sys.MouseShow()

	for {
		key := ctx.Sys.GetInput()
		if key < 0 {
			continue
		}
		if m.handleKey(key) {
			return nil
		}
	}
}

// buildWidgets creates the per slot action buttons and the right hand column,
// and returns their ids so the caller can delete them on the way out.
//
// C++: RepairShopMenu::RepairShopMenu - the button table (repairshopmenu.cpp)
func (m *repairShopMenu) buildWidgets() []gnw.ButtonID {
	// The slots go in first, so they keep the original's creation order.
	return append(m.buildSlotButtons(), m.buildColumnButtons()...)
}

// buildSlotButtons creates the four action buttons of every on screen slot.
func (m *repairShopMenu) buildSlotButtons() []gnw.ButtonID {
	ctx, info := m.ctx, m.info

	var buttons []gnw.ButtonID

	// Per slot buttons (created once; only their enable state and the unit they
	// map to change as the page scrolls). Activate always; the shop trio only for
	// a stationary building.
	m.slots = make([]rsSlotButtons, m.slotsPerScreen)
	for p := 0; p < m.slotsPerScreen; p++ {
		_, _, _, statX, statY := rsSlotGeom(info.IsHangar, p)
		v := rsSlotBase + p*4
		if b, e := ctx.captionButton(mainWindow, statX-12, statY+42, "DPTBAYUP", "DPTBAYDN", "DPTBAYDN",
			ctx.text("2443", "Activate"), v+rsAct); e == nil {
			ctx.registerButtonSfx(b, "MBUTT0")
			m.slots[p].activate = b
			buttons = append(buttons, b)
		}
		if info.IsShop {
			if b, e := ctx.captionButton(mainWindow, statX-12, statY+67, "DPTBAYUP", "DPTBAYDN", "DPTBAYDN",
				ctx.text("bdc0", "Reload"), v+rsRel); e == nil {
				ctx.registerButtonSfx(b, "MBUTT0")
				m.slots[p].reload = b
				buttons = append(buttons, b)
			}
			if b, e := ctx.captionButton(mainWindow, statX+63, statY+42, "DPTBAYUP", "DPTBAYDN", "DPTBAYDN",
				ctx.text("1196", "Repair"), v+rsRep); e == nil {
				ctx.registerButtonSfx(b, "MBUTT0")
				m.slots[p].repair = b
				buttons = append(buttons, b)
			}
			if b, e := ctx.captionButton(mainWindow, statX+63, statY+67, "DPTBAYUP", "DPTBAYDN", "DPTBAYDN",
				ctx.text("7ec0", "Upgrade"), v+rsUpg); e == nil {
				ctx.registerButtonSfx(b, "MBUTT0")
				m.slots[p].upgrade = b
				buttons = append(buttons, b)
			}
		}
	}

	return buttons
}

// buildColumnButtons creates the right hand column: the four "all units"
// actions, the raw material caption, and Done/scroll/help.
func (m *repairShopMenu) buildColumnButtons() []gnw.ButtonID {
	ctx, info, f5 := m.ctx, m.info, m.f5

	var buttons []gnw.ButtonID

	if info.ShowActivateAll {
		if b, e := ctx.captionButton(mainWindow, 511, 252, "DPTMNUUP", "DPTMNUDN", "DPTMNUDN",
			ctx.text("0fbe", "Activate All"), rsActivateAll); e == nil {
			ctx.registerButtonSfx(b, "MBUTT0")
			m.actAll = b
			buttons = append(buttons, b)
		}
	}
	if info.ShowMaterials {
		if b, e := ctx.captionButton(mainWindow, 511, 277, "DPTMNUUP", "DPTMNUDN", "DPTMNUDN",
			ctx.text("e609", "Reload All"), rsReloadAll); e == nil {
			ctx.registerButtonSfx(b, "MBUTT0")
			m.relAll = b
			buttons = append(buttons, b)
		}
		if b, e := ctx.captionButton(mainWindow, 511, 302, "DPTMNUUP", "DPTMNUDN", "DPTMNUDN",
			ctx.text("670f", "Repair All"), rsRepairAll); e == nil {
			ctx.registerButtonSfx(b, "MBUTT0")
			m.repAll = b
			buttons = append(buttons, b)
		}
		if b, e := ctx.captionButton(mainWindow, 511, 327, "DPTMNUUP", "DPTMNUDN", "DPTMNUDN",
			ctx.text("003c", "Upgrade All"), rsUpgradeAll); e == nil {
			ctx.registerButtonSfx(b, "MBUTT0")
			m.upgAll = b
			buttons = append(buttons, b)
		}
		// "Raw materials" caption above the gauge, wrapped + centred in its 72x20
		// box (the single line overflows the box width). C++: Text_TextBox.
		buf := ctx.Sys.Buf(mainWindow)
		ctx.Sys.Lock()
		lines := wrapLines(f5, ctx.text("c46e", "Raw Materials"), 72, 2)
		rowH := f5.Height + 1
		ly := 53 + (20-len(lines)*rowH)/2
		for _, ln := range lines {
			blitInBox(buf, f5, ln, 520, ly, 72, rowH, colorGreen, true)
			ly += rowH
		}
		ctx.Sys.Unlock()
	}
	if b, e := ctx.captionButton(mainWindow, 511, 372, "DPTMNUUP", "DPTMNUDN", "", ctx.text("27af", "Done"), rsDone); e == nil {
		ctx.registerButtonSfx(b, "NDONE0")
		buttons = append(buttons, b)
	}
	if b, e := ctx.captionButton(mainWindow, 504, 426, "DPTUP_UP", "DPTUP_DN", "DPTUP_X", "", rsUpArrow); e == nil {
		ctx.registerButtonSfx(b, "MBUTT0")
		m.upArrow = b
		buttons = append(buttons, b)
	}
	if b, e := ctx.captionButton(mainWindow, 530, 426, "DPTDN_UP", "DPTDN_DN", "DPTDN_X", "", rsDownArrow); e == nil {
		ctx.registerButtonSfx(b, "MBUTT0")
		m.downArrow = b
		buttons = append(buttons, b)
	}
	if b, e := ctx.captionButton(mainWindow, 584, 426, "DPTHP_UP", "DPTHP_DN", "", "", rsHelp); e == nil {
		ctx.registerButtonSfx(b, "NHELP0")
		buttons = append(buttons, b)
	}

	return buttons
}

// handleKey answers one input code and reports whether the screen closes.
//
// C++: RepairShopMenu::Run - the get_input switch (repairshopmenu.cpp)
func (m *repairShopMenu) handleKey(key int) bool {
	ctx, ops := m.ctx, m.ops

	switch {
	case key == rsDone || key == platform.KeyEscape || key == platform.KeyReturn:
		return true
	case key == rsHelp || key == '?':
		section := "DEPOT_SETUP"
		if m.info.IsHangar {
			section = "HANGAR_SETUP"
		}
		Help(ctx, section)
	case key == rsUpArrow:
		if m.slotIndex > 0 {
			m.slotIndex -= m.slotsPerScreen
			if m.slotIndex < 0 {
				m.slotIndex = 0
			}
			m.refresh()
		}
	case key == rsDownArrow:
		if m.slotIndex+m.slotsPerScreen < len(m.info.Units) {
			m.slotIndex += m.slotsPerScreen
			m.refresh()
		}
	case key == rsActivateAll:
		// Activate ejects a stored unit out of the building (it deploys via BLDMRK
		// markers on the map), so the screen CLOSES - the player then clicks a marker.
		ops.ActivateAll()
		return true
	case key == rsReloadAll:
		ops.ReloadAll()
		m.refresh()
	case key == rsRepairAll:
		ops.RepairAll()
		m.refresh()
	case key == rsUpgradeAll:
		ops.UpgradeAll()
		m.refresh()
	case key >= rsSlotBase && key < rsSlotBase+m.slotsPerScreen*4:
		p := (key - rsSlotBase) / 4
		a := (key - rsSlotBase) % 4
		abs := m.slotIndex + p
		if abs >= len(m.info.Units) {
			break
		}
		switch a {
		case rsAct:
			ops.Activate(abs) // ejects the stored unit out via BLDMRK; close the screen
			return true
		case rsRel:
			ops.Reload(abs)
			m.refresh()
		case rsRep:
			ops.Repair(abs)
			m.refresh()
		case rsUpg:
			ops.Upgrade(abs)
			m.refresh()
		}
	}

	return false
}

// drawSlots repaints the slot wells + stats from the page at slotIndex and
// sets each per slot button's enable state.
func (m *repairShopMenu) drawSlots() {
	buf := m.ctx.Sys.Buf(mainWindow)
	m.ctx.Sys.Lock()
	for p := 0; p < m.slotsPerScreen; p++ {
		ulx, uly, wellW, statX, statY := rsSlotGeom(m.info.IsHangar, p)
		abs := m.slotIndex + p
		// Restore the well + stats area from the background frame.
		rsRestore(buf, m.pic.Buffer(), ulx, uly, wellW, 128)
		rsRestore(buf, m.pic.Buffer(), statX, statY, 122, 30)
		if abs < len(m.info.Units) {
			u := m.info.Units[abs]
			if u.Portrait != nil {
				px := ulx + (wellW-u.Portrait.W)/2
				py := uly + (128-u.Portrait.H)/2
				blitIcon(buf, u.Portrait, px, py)
			}
			blitInBox(buf, m.f5, u.Name, ulx+10, uly+3, wellW-20, 18, 0xA2|fonts.FlagOutline, true)
			rsStatRow(buf, m.f5, statX, statY, 122, 15, m.ctx.text("f0dc", "Hits"), u.Hits, u.MaxHits)
			if u.ShowCargo {
				rsStatRow(buf, m.f5, statX, statY+15, 122, 15, m.ctx.text("8688", "Cargo"), u.Cargo, u.MaxCargo)
			} else if u.MaxAmmo > 0 {
				rsStatRow(buf, m.f5, statX, statY+15, 122, 15, m.ctx.text("c163", "Ammo"), u.Ammo, u.MaxAmmo)
			}
		}
	}
	m.ctx.Sys.Unlock()

	for p := 0; p < m.slotsPerScreen; p++ {
		abs := m.slotIndex + p
		var u *RepairUnitInfo
		if abs < len(m.info.Units) {
			u = &m.info.Units[abs]
		}
		rsEnable(m.ctx.Sys, m.slots[p].activate, u != nil && u.CanActivate)
		rsEnable(m.ctx.Sys, m.slots[p].reload, u != nil && u.CanReload)
		rsEnable(m.ctx.Sys, m.slots[p].repair, u != nil && u.CanRepair)
		rsEnable(m.ctx.Sys, m.slots[p].upgrade, u != nil && u.CanUpgrade)
	}
}

// drawBar repaints the complex raw material gauge and its numeric readout.
func (m *repairShopMenu) drawBar() {
	if !m.info.ShowMaterials {
		return
	}
	buf := m.ctx.Sys.Buf(mainWindow)
	m.ctx.Sys.Lock()
	defer m.ctx.Sys.Unlock()
	const bx, by, bw, bh = 546, 106, 20, 115
	rsRestore(buf, m.pic.Buffer(), bx, by, bw, bh)
	fill := 0
	if m.info.RawCapacity > 0 {
		fill = m.info.RawMaterials * bh / m.info.RawCapacity
	}
	fill = clampInt(fill, 0, bh)
	if m.barBuf != nil && fill > 0 {
		w := minInt(bw, m.barBuf.W)
		h := minInt(fill, m.barBuf.H)
		// Anchor the gradient to the bottom of both bar and gauge image.
		buf.Sub(bx, by+bh-h, w, h).TransCopy(m.barBuf.Sub(0, m.barBuf.H-h, w, h))
	}
	rsRestore(buf, m.pic.Buffer(), 529, 76, 54, 12)
	blitInBox(buf, m.f5, fmt.Sprintf("%d", m.info.RawMaterials), 529, 76, 54, 12, colorGreen, true)
}

// updateGlobals sets the *All buttons + scroll arrows enable state.
func (m *repairShopMenu) updateGlobals() {
	rsEnable(m.ctx.Sys, m.upArrow, m.slotIndex > 0)
	rsEnable(m.ctx.Sys, m.downArrow, m.slotIndex+m.slotsPerScreen < len(m.info.Units))
	anyRel, anyRep, anyUpg := false, false, false
	for i := range m.info.Units {
		anyRel = anyRel || m.info.Units[i].CanReload
		anyRep = anyRep || m.info.Units[i].CanRepair
		anyUpg = anyUpg || m.info.Units[i].CanUpgrade
	}
	rsEnable(m.ctx.Sys, m.actAll, len(m.info.Units) > 0)
	rsEnable(m.ctx.Sys, m.relAll, anyRel)
	rsEnable(m.ctx.Sys, m.repAll, anyRep)
	rsEnable(m.ctx.Sys, m.upgAll, anyUpg)
}

// refresh takes a fresh sim snapshot and repaints slots, gauge, and globals.
func (m *repairShopMenu) refresh() {
	m.info = m.ops.Snapshot()
	m.drawSlots()
	m.drawBar()
	m.updateGlobals()
}

// rsSlotGeom returns slot p's well top left (ulx, uly), well width, and the
// stats box top left (statX, statY). C++: the per index ulx/uly arithmetic in
// the RepairShopMenu constructor (HANGAR = 2x2 wide wells, else 3x2).
func rsSlotGeom(isHangar bool, p int) (ulx, uly, wellW, statX, statY int) {
	if isHangar {
		ulx = (p&1)*226 + 18
		uly = (p/2)*235 + 9
		return ulx, uly, 200, ulx + 38, uly + 140
	}
	ulx = (p%3)*155 + 17
	uly = (p/3)*235 + 9
	return ulx, uly, 128, ulx + 3, uly + 140
}

// rsStatRow draws one "label  cur/max" readout: the value coloured green over
// half, gold over a quarter, red below (the original's SI_/EI_ gauge tint),
// the label in gold beside it. C++: ReportStats_DrawRow, reduced to the text.
func rsStatRow(buf *gfx.Buffer, f *fonts.Font, x, y, w, h int, label string, cur, max int) {
	if max <= 0 {
		return
	}
	cur = clampInt(cur, 0, max)
	color := 0x02 // green
	switch {
	case cur*4 <= max:
		color = 0x01 // red
	case cur*2 <= max:
		color = 0xA2 // gold
	}
	blitInBox(buf, f, label, x, y, 40, h, 0xA2, false)
	blitInBox(buf, f, fmt.Sprintf("%d/%d", cur, max), x+42, y, w-42, h, color, false)
}

// rsRestore copies a region of the background frame back into the window buffer,
// clipped. The caller holds the gnw lock. C++: Image::Write (the bg_*_area).
func rsRestore(buf, bg *gfx.Buffer, x, y, w, h int) {
	w = minInt(w, minInt(buf.W, bg.W)-x)
	h = minInt(h, minInt(buf.H, bg.H)-y)
	if w <= 0 || h <= 0 || x < 0 || y < 0 {
		return
	}
	buf.Sub(x, y, w, h).Copy(bg.Sub(x, y, w, h))
}

// rsEnable enables or disables a button (does nothing for an unbuilt -1 id).
func rsEnable(sys *gnw.System, bid gnw.ButtonID, on bool) {
	if bid < 0 {
		return
	}
	if on {
		sys.EnableButton(bid)
	} else {
		sys.DisableButton(bid)
	}
}

// captionButton builds a button from up/down simple image faces with a centred
// bevel and shadow caption (gold at rest, dark orange pressed), optionally a
// disabled face (disTag, captioned dark orange). It auto keys a fuchsia
// transparent corner like plateButton/imageButton. An empty label leaves the
// faces uncaptioned (the scroll arrows / help button).
//
// C++: Button::SetCaption + Button::CopyUpDisabled (button.cpp).
func (ctx *Context) captionButton(win gnw.WinID, x, y int, upTag, downTag, disTag, label string, value int) (gnw.ButtonID, error) {
	upImg, err := ctx.loadSimple(upTag)
	if err != nil {
		return -1, err
	}
	downImg, err := ctx.loadSimple(downTag)
	if err != nil {
		return -1, err
	}
	up := upImg.Buffer().Clone()
	down := downImg.Buffer().Clone()
	var flags uint32
	if keyButtonCorner(up) {
		keyButtonCorner(down)
		flags = gnw.ButtonTransparent
	}
	f := ctx.Sys.Fonts().Font()
	bake := func(b *gfx.Buffer, color fonts.Color) {
		if label == "" {
			return
		}
		tw := f.Width(label)
		textX := (b.W - tw) / 2
		textY := (b.H - f.Height) / 2
		if textX < 0 {
			textX = 0
		}
		if textY < 0 {
			textY = 0
		}
		f.BlitShadowed(b.Sub(textX, textY, b.W-textX, b.H-textY), label, 0, color)
	}
	bake(up, fonts.ColorGold)
	bake(down, fonts.ColorDarkOrange)

	var dis *gfx.Buffer
	if disTag != "" {
		if di, e := ctx.loadSimple(disTag); e == nil {
			dis = di.Buffer().Clone()
			if flags&gnw.ButtonTransparent != 0 {
				keyButtonCorner(dis)
			}
			bake(dis, fonts.ColorDarkOrange)
		}
	}

	bid := ctx.Sys.AddButton(win, x, y, up.W, up.H,
		gnw.NoValue, gnw.NoValue, gnw.NoValue, value, up, down, nil, flags)
	if bid < 0 {
		return -1, fmt.Errorf("menus: AddButton failed for %s", upTag)
	}
	if dis != nil {
		ctx.Sys.SetDisabledImages(bid, dis, dis, nil)
	}
	return bid, nil
}
