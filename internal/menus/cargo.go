// cargo.go : implements the cargo purchase and loadout screen.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++: cargomenu.cpp - CargoMenu
package menus

import (
	"errors"
	"fmt"
	"strings"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/gnw"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/sprites"
	"github.com/suglasp/max-go/internal/units"
)

// Cargo screen text colours (color.h): tan headers (0xA2) and the yellow gold
// figure (COLOR_YELLOW).
const (
	cargoTan    = 0xA2
	cargoGold   = 0x04 // COLOR_YELLOW
	cargoChrome = 0x05 // COLOR_CHROME_YELLOW (the orange separator lines)
)

// CARGOPIC shows the buyable roster, purchased mission supplies, and selected unit details. Buy spends gold and Delete refunds it.

// unitPortrait maps a unit type to its large P_* portrait BIG image, shown in the top
// left preview of the cargo/upgrade screens. Covers buildings and units; a type with no
// entry shows no portrait. C++: base_unit.GetPortrait() by AbstractUpgradeMenu::DrawUnitInfo.
var unitPortrait = map[assets.UnitType]string{
	// Buildings / structures.
	assets.PowerStn: "P_POWSTN", assets.PowGen: "P_POWGEN", assets.Barracks: "P_BARRCK",
	assets.ShieldGn: "P_SHIELD", assets.Radar: "P_RADAR", assets.ADump: "P_SMSTOR",
	assets.FDump: "P_SMFUEL", assets.GoldSm: "P_SMVLT", assets.Depot: "P_DEPOT",
	assets.Hangar: "P_HANGAR", assets.Dock: "P_DOCK", assets.Cnct4W: "P_CONNEC",
	assets.Road: "P_ROAD", assets.LandPad: "P_LANDPD", assets.Shipyard: "P_SHIPYD",
	assets.LightPlt: "P_LGHTPL", assets.LandPlt: "P_HVYPLT",
	assets.AirPlt: "P_AIRPLT", assets.Habitat: "P_HABITA", assets.Research: "P_RESEAR",
	assets.Greenhse: "P_GREENH", assets.RecCentr: "P_RECCTR", assets.TrainHal: "P_TRNHLL",
	// SUPRTPLT is the alien Life Support artifact despite its enum name; its right-click image is P_LIFESP rather than P_SUPRTP.
	assets.SuprtPlt: "P_LIFESP",
	assets.WtrPltfm: "P_WATER", assets.GunTurrt: "P_GUNTUR", assets.AntiAir: "P_FXAA",
	assets.ArtyTrrt: "P_ARTYTR", assets.AntiMssl: "P_FXROCK", assets.Block: "P_BLOCK",
	assets.Bridge: "P_BRIDGE", assets.MiningSt: "P_MINING", assets.LandMine: "P_LANDMN",
	assets.SeaMine: "P_SEAMIN", assets.Master: "P_MASTER",
	// The Gold Refinery (COMMTWR) photo is stored under the misleading tag
	// "P_TRANSP" (it is the building image with the tower and dish, not a transparent
	// placeholder); the original's GetPortrait() returns it and LoadBigImage shows it.
	assets.CommTwr: "P_TRANSP",
	// Mobile units.
	assets.Constrct: "P_CONTRC", assets.Scout: "P_SCOUT", assets.Tank: "P_TANK",
	assets.Artillry: "P_ARTY", assets.RocktLch: "P_ROCKET", assets.MisslLch: "P_MISSIL",
	assets.SpFlak: "P_AA", assets.MineLayr: "P_MNELAY", assets.Surveyor: "P_SURVEY",
	assets.Scanner: "P_SCANNR", assets.SplyTrck: "P_SPLYTR", assets.GoldTrck: "P_GOLDTR",
	assets.Engineer: "P_ENGINR", assets.Bulldozr: "P_BULLDZ", assets.Repair: "P_REPAIR",
	assets.FuelTrck: "P_FUELTR", assets.ClnTrans: "P_COLNST", assets.Commando: "P_COMMAN",
	assets.Infantry: "P_INFANT", assets.FastBoat: "P_ESCORT", assets.Corvette: "P_CORVET",
	assets.Battlshp: "P_GUNBT", assets.Submarne: "P_SUB", assets.SeaTrans: "P_SEATRN",
	assets.MsslBoat: "P_MSLCR", assets.SeaMnLyr: "P_SEAMNL", assets.CargoShp: "P_CARGOS",
	assets.Fighter: "P_FIGHTR", assets.Bomber: "P_BOMBER", assets.AirTrans: "P_AIRTRN",
	assets.Awac: "P_AWAC", assets.Juggrnt: "P_JUGGER", assets.AlnTank: "P_ALNTAN",
	assets.AlnAsGun: "P_ALNASG", assets.AlnPlane: "P_ALNPLA",
}

// ErrCancelled is returned when RunCargo exits through Cancel or Escape so the caller returns to the previous screen.
var ErrCancelled = errors.New("menus: cancelled")

// View state: which list is active.
const (
	cgBuyList   = iota // right panel (roster)
	cgCargoList        // middle panel (chosen units)
)

// Panel geometry (from the CARGOPIC layout / cargomenu.cpp window rects).
const (
	cgBuyX, cgBuyY, cgBuyW, cgBuyH         = 482, 80, 143, 290 // panel + Cost column run to x=625
	cgCargoX, cgCargoY, cgCargoW, cgCargoH = 337, 45, 118, 185
	cgPrevX, cgPrevY, cgPrevW, cgPrevH     = 11, 13, 300, 240 // portrait fills the full 300px panel
	cgStatX, cgStatY, cgStatW, cgStatH     = 11, 293, 250, 174
	cgRowH                                 = 32
	// Vertical worm bars: Credit/gold (VERTGOLD) and the selected unit's
	// Cargo (VERTRAW/FUEL/GOLD). C++ gold_background (361,276,40,140) + the
	// cargo scrollbar slot. The top 25px hold the value + label; the worm
	// fills the rest from the bottom.
	cgGoldX, cgGoldY, cgGoldW, cgGoldH = 361, 276, 40, 140
	cgCrgX, cgCrgY, cgCrgW, cgCrgH     = 421, 276, 20, 140
)

// cargoMenu is the purchase screen state: the loadout under edit with its
// rollback snapshot, the two list selections, the per type value tables, and
// the art/caches the draw methods share. C++: cargomenu.cpp - CargoMenu.
type cargoMenu struct {
	ctx   *Context
	lo    *CargoLoadout // edited in place; entry is the Cancel rollback
	entry CargoLoadout

	active                int // cgBuyList or cgCargoList
	buySel, buyScroll     int
	cargoSel, cargoScroll int
	descMode              bool // Description checkbox: overlay the unit's description on the portrait
	buyRows, cargoRows    int

	pic *sprites.Big
	f5  *fonts.Font
	buf *gfx.Buffer
	sc  *units.SpriteCache
	at  []assets.UnitAttributes

	baseVals, curVals map[assets.UnitType]*game.UnitValues // immutable specs and displayed upgrade values
	statIcons         map[string]*gfx.Buffer
	portraitCache     map[assets.UnitType]*sprites.Big
	upRows            []cargoStepRow // visible attribute steppers and their row positions

	buyBtn, deleteBtn gnw.ButtonID

	dbl menuDoubleClick
}

// RunCargo edits a CARGOPIC loadout in place. Done commits changes; Cancel or Escape restores the entry state and returns ErrCancelled.
func RunCargo(ctx *Context, lo *CargoLoadout) error {
	pic, err := ctx.loadBig("CARGOPIC")
	if err != nil {
		return err
	}
	ctx.drawBigAt(mainWindow, pic, 0, 0)
	ctx.Plat.SetPalette(&pic.Palette)

	oldFont := ctx.Sys.Fonts().GetFont()
	defer ctx.Sys.Fonts().SetFont(oldFont)

	m := &cargoMenu{
		ctx:           ctx,
		lo:            lo,
		active:        cgBuyList,
		buyRows:       cgBuyH / cgRowH,
		cargoRows:     cgCargoH / cgRowH,
		pic:           pic,
		f5:            ctx.Sys.Fonts().Get(fonts.Font5),
		buf:           ctx.Sys.Buf(mainWindow),
		sc:            units.NewSpriteCache(ctx.Lib, &pic.Palette),
		at:            ctx.Assets.Attribs,
		baseVals:      map[assets.UnitType]*game.UnitValues{},
		curVals:       map[assets.UnitType]*game.UnitValues{},
		statIcons:     map[string]*gfx.Buffer{},
		portraitCache: map[assets.UnitType]*sprites.Big{},
		buyBtn:        -1,
		deleteBtn:     -1,
	}
	// Rollback snapshot for Cancel.
	m.entry = *lo
	m.entry.Units = append([]assets.UnitType(nil), lo.Units...)
	m.entry.Cargos = append([]int(nil), lo.Cargos...)

	buttons := m.buildWidgets()
	defer func() {
		for _, b := range buttons {
			ctx.Sys.DeleteButton(b)
		}
	}()

	m.drawBuyList()
	m.drawCargoList()
	m.drawDetail()
	m.drawBars()
	m.updateBuyButton()
	m.updateDeleteButton()
	ctx.Sys.MouseShow()

	for {
		key := ctx.Sys.GetInput()
		if key == -1 {
			continue
		}
		if done, err := m.handleKey(key); done {
			return err
		}
	}
}

// buildWidgets paints the static header captions and creates every button and
// click region of the purchase screen, returning the ids for the caller to free.
// C++: the Text_TextBox + Button setup in the CargoMenu ctor (cargomenu.cpp).
func (m *cargoMenu) buildWidgets() []gnw.ButtonID {
	ctx := m.ctx

	m.drawStaticText()

	var buttons []gnw.ButtonID
	addImg := func(x, y int, up, down string, value int, sfx string) {
		if b, e := ctx.imageButton(mainWindow, x, y, up, down, "", 0, 0, false, value); e == nil {
			if sfx != "" {
				ctx.registerButtonSfx(b, sfx)
			}
			buttons = append(buttons, b)
		}
	}
	// The BLD*/CRG* button faces are blank; the captions are drawn on them with
	// FONT_5 (only BLDHLP carries a baked "?"). Pin the font so they read the
	// same regardless of the caller's current font.
	// C++: Button::SetCaption on the AbstractUpgradeMenu/CargoMenu buttons.
	ctx.Sys.Fonts().SetFont(fonts.Font5)
	addPlate := func(x, y int, up, down, label string, value int, sfx string) {
		if b, e := ctx.plateButton(mainWindow, x, y, up, down, label, value); e == nil {
			if sfx != "" {
				ctx.registerButtonSfx(b, sfx)
			}
			buttons = append(buttons, b)
		}
	}
	// Scroll arrows (no caption) and the help "?" (baked into the art).
	// Up button sits 1px above its paired down button; the whole pair is nudged
	// +1 down from the panel baseline so they read level.
	addImg(471, 387, "BLDUP__U", "BLDUP__D", 1110, "KCARG0") // roster up
	addImg(491, 388, "BLDDWN_U", "BLDDWN_D", 1111, "KCARG0") // roster down
	addImg(327, 240, "BLDUP__U", "BLDUP__D", 1112, "KCARG0") // cargo list up
	addImg(348, 241, "BLDDWN_U", "BLDDWN_D", 1113, "KCARG0") // cargo list down
	addImg(413, 424, "BLDUP__U", "BLDUP__D", 1104, "KCARG0") // cargo amount +5
	addImg(433, 425, "BLDDWN_U", "BLDDWN_D", 1105, "KCARG0") // cargo amount -5
	addImg(420, 452, "BLDHLP_U", "BLDHLP_D", 1001, "NHELP0") // help (baked "?")
	// Description checkbox (sticky): toggles the description text overlay on the
	// portrait. C++: button_description (BLDDES) at 292,264.
	if b, e := ctx.imageButton(mainWindow, 292, 264, "BLDDES_U", "BLDDES_D", "", 0, 0, true, 1200); e == nil {
		ctx.Sys.SetButtonToggle(b) // a real toggle: a second click releases it
		ctx.registerButtonSfx(b, "KCARG0")
		buttons = append(buttons, b)
	}
	// Captioned buttons: Buy / Delete / Cancel / Done.
	if bid, e := ctx.cargoPlateButtonWithDisabled(mainWindow, 590, 386, "CRGBUY_U", "CRGBUY_D", ctx.text("64e3", "Buy"), 1102); e == nil {
		m.buyBtn = bid
		ctx.registerButtonSfx(bid, "KCARG0")
		buttons = append(buttons, bid)
	}
	if bid, e := ctx.cargoPlateButtonWithDisabled(mainWindow, 412, 240, "CRGDEL_U", "CRGDEL_D", ctx.text("4d73", "Delete"), 1103); e == nil {
		m.deleteBtn = bid
		ctx.registerButtonSfx(bid, "KCARG0")
		buttons = append(buttons, bid)
	}
	addPlate(357, 452, "BLDCAN_U", "BLDCAN_D", ctx.text("c6aa", "Cancel"), 1002, "NCANC0")
	addPlate(447, 452, "BLDONE_U", "BLDONE_D", ctx.text("bd89", "Done"), 1000, "NDONE0")
	// Invisible click regions over each list so a row click registers (a bare
	// click on the full screen window produces no input otherwise).
	// C++: the UnitTypeSelector / CargoSelector clickable widgets.
	if b := ctx.Sys.AddButton(mainWindow, cgBuyX, cgBuyY, cgBuyW, cgBuyH,
		gnw.NoValue, gnw.NoValue, gnw.NoValue, 1300, nil, nil, nil, 0); b >= 0 {
		buttons = append(buttons, b)
	}
	if b := ctx.Sys.AddButton(mainWindow, cgCargoX, cgCargoY, cgCargoW, cgCargoH,
		gnw.NoValue, gnw.NoValue, gnw.NoValue, 1301, nil, nil, nil, 0); b >= 0 {
		buttons = append(buttons, b)
	}
	// Upgrade stepper columns: '<' (downgrade/refund) and '>' (buy upgrade). A
	// click finds the row under the cursor; only rows with a drawn stepper
	// (recorded in upRows) act, so rows that cannot be upgraded stay unchanged.
	if b := ctx.Sys.AddButton(mainWindow, 283, cgStatY, 18, cgStatH,
		gnw.NoValue, gnw.NoValue, gnw.NoValue, 2500, nil, nil, nil, 0); b >= 0 {
		buttons = append(buttons, b)
	}
	if b := ctx.Sys.AddButton(mainWindow, 301, cgStatY, 18, cgStatH,
		gnw.NoValue, gnw.NoValue, gnw.NoValue, 2501, nil, nil, nil, 0); b >= 0 {
		buttons = append(buttons, b)
	}
	// Click region over the Cargo worm bar: clicking sets the load to the clicked
	// height. C++: the LimitedScrollbar slider in CargoMenu.
	if b := ctx.Sys.AddButton(mainWindow, cgCrgX, cgCrgY+25, cgCrgW, cgCrgH-25,
		gnw.NoValue, gnw.NoValue, gnw.NoValue, 1106, nil, nil, nil, 0); b >= 0 {
		buttons = append(buttons, b)
	}

	return buttons
}

// handleKey runs one input event of the purchase screen. It reports done once
// the player leaves, with nil on Done (commit) and ErrCancelled on Cancel/ESC.
// C++: the AbstractUpgradeMenu::Run event switch (cargomenu.cpp).
func (m *cargoMenu) handleKey(key int) (done bool, err error) {
	ctx := m.ctx
	lo := m.lo
	at := m.at

	switch {
	case key == 1000 || key == platform.KeyReturn: // Done: commit
		lo.StartGold = m.entry.StartGold
		lo.Upgrades = m.commitUpgrades()

		return true, nil
	case key == 1002 || key == platform.KeyEscape: // Cancel: roll back + signal back out
		*lo = m.entry
		return true, ErrCancelled
	case key == 1001 || key == '?':
		Help(ctx, "CARGO_SETUP")
	case key == 1200: // Description checkbox: toggle the portrait text overlay
		m.descMode = !m.descMode
		m.drawDetail()
	case key == 1110: // roster scroll up
		if m.buyScroll > 0 {
			m.buyScroll--
			m.drawBuyList()
		}
	case key == 1111: // roster scroll down
		if m.buyScroll+m.buyRows < len(cargoRoster) {
			m.buyScroll++
			m.drawBuyList()
		}
	case key == 1112: // cargo scroll up
		if m.cargoScroll > 0 {
			m.cargoScroll--
			m.drawCargoList()
		}
	case key == 1113: // cargo scroll down
		if m.cargoScroll+m.cargoRows < len(lo.Units) {
			m.cargoScroll++
			m.drawCargoList()
		}
	case key == 1104: // cargo amount +5
		if m.cargoSel < len(lo.Units) {
			m.loadCargoTo(lo.Cargos[m.cargoSel] + 5)
		}
	case key == 1105: // cargo amount -5
		if m.cargoSel < len(lo.Units) {
			m.loadCargoTo(lo.Cargos[m.cargoSel] - 5)
		}
	case key == 1106: // click in the cargo bar: set load to the clicked height
		if m.cargoSel < len(lo.Units) && int(lo.Units[m.cargoSel]) < len(at) {
			st := at[lo.Units[m.cargoSel]].Storage
			top, bot := cgCrgY+25, cgCrgY+25+(cgCrgH-25)
			my := clampInt(ctx.Plat.Mouse().Y, top, bot)
			m.loadCargoTo(st * (bot - my) / (bot - top))
		}
	case key == 1102: // Buy
		m.buySelected()
	case key == 1103: // Delete (only a bought unit, not a protected mission supply one)
		m.deleteSelected()
	case key == 1300: // click in the Available (roster) list
		if r := (ctx.Plat.Mouse().Y - cgBuyY) / cgRowH; r >= 0 && m.buyScroll+r < len(cargoRoster) {
			// Picking a row is audible, even when it was already the selected one.
			// C++: UnitTypeSelector::Select plays KCARG0, inherited by the purchase
			// screen's two selectors (purchasetypeselector / cargoselector).
			ctx.playSfx("KCARG0")
			m.active, m.buySel = cgBuyList, m.buyScroll+r
			m.drawBuyList()
			m.drawCargoList()
			m.drawDetail()
			m.updateBuyButton()
			if m.dbl.Click(key, m.buySel, platform.Ticks()) {
				m.buySelected()
			}
		}
	case key == 1301: // click in the Purchased (cargo) list
		if r := (ctx.Plat.Mouse().Y - cgCargoY) / cgRowH; r >= 0 && m.cargoScroll+r < len(lo.Units) {
			ctx.playSfx("KCARG0")
			m.active, m.cargoSel = cgCargoList, m.cargoScroll+r
			m.drawBuyList()
			m.drawCargoList()
			m.drawDetail()
			m.drawBars() // the Cargo bar tracks the Purchased selection
			m.updateDeleteButton()
			if m.dbl.Click(key, m.cargoSel, platform.Ticks()) {
				m.deleteSelected()
			}
		}
	case key == 2500, key == 2501: // attribute stepper: '<' refund / '>' upgrade
		m.stepAttribute(key)
	}

	return false, nil
}

// wrapTwoLines greedily splits label into at most two lines that each fit width
// w (the original's Text_TextBox auto wrap in the 32px list rows), so long names
// like "Rocket Launcher" stack instead of truncating.
func wrapTwoLines(f *fonts.Font, label string, w int) []string {
	if f.Width(label) <= w {
		return []string{label}
	}
	var l1, l2 string
	for _, word := range strings.Fields(label) {
		switch {
		case l1 == "":
			l1 = word
		case l2 == "" && f.Width(l1+" "+word) <= w:
			l1 += " " + word
		case l2 == "":
			l2 = word
		default:
			l2 += " " + word
		}
	}
	if l2 == "" {
		return []string{l1}
	}
	return []string{l1, l2}
}

// wrapText greedily splits text into lines that each fit width w (unlimited
// lines): for the multi line description overlay.
func wrapText(f *fonts.Font, text string, w int) []string {
	var lines []string
	cur := ""
	for _, word := range strings.Fields(text) {
		try := word
		if cur != "" {
			try = cur + " " + word
		}
		if f.Width(try) <= w || cur == "" {
			cur = try
		} else {
			lines = append(lines, cur)
			cur = word
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

// cargoPlateButtonWithDisabled builds a Cargo screen captioned button with a
// real disabled face. The generic plateButton helper has no disabled image hook,
// but Buy/Delete both need to visibly gray out when their action is unavailable.
func (ctx *Context) cargoPlateButtonWithDisabled(win gnw.WinID, x, y int, upTag, downTag, label string, value int) (gnw.ButtonID, error) {
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
	disabled := up.Clone()
	var flags uint32
	if keyButtonCorner(up) {
		keyButtonCorner(down)
		keyButtonCorner(disabled)
		flags = gnw.ButtonTransparent
	}

	f := ctx.Sys.Fonts().Font()
	tw := f.Width(label)
	textX := (up.W - tw) / 2
	textY := (up.H - f.Height) / 2
	if textX < 0 {
		textX = 0
	}
	if textX > up.W-1 {
		textX = up.W - 1
	}
	f.BlitShadowed(up.Sub(textX, textY, up.W-textX, up.H-textY), label, 0, fonts.ColorGold)
	f.BlitShadowed(down.Sub(textX, textY, down.W-textX, down.H-textY), label, 0, fonts.ColorDarkOrange)
	f.BlitShadowed(disabled.Sub(textX, textY, disabled.W-textX, disabled.H-textY), label, 0, fonts.ColorDarkOrange)

	bid := ctx.Sys.AddButton(win, x, y, up.W, up.H,
		gnw.NoValue, gnw.NoValue, gnw.NoValue, value, up, down, nil, flags)
	if bid < 0 {
		return -1, fmt.Errorf("menus: AddButton failed for %q", label)
	}
	ctx.Sys.SetDisabledImages(bid, disabled, disabled, nil)
	return bid, nil
}
