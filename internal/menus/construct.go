// construct.go : implements the build-type picker for Constructor and Engineer placement orders.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++: constructmenu.cpp - ConstructMenu
package menus

import (
	"fmt"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/build"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/gnw"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/screendata"
	"github.com/suglasp/max-go/internal/sprites"
	"github.com/suglasp/max-go/internal/units"
)

// CONBUILD lets a Constructor or Engineer choose a buildable type. It shows the portrait, stats, X1/X2/X4
// rates, cost, and build list; Constructors build 2x2 structures and Engineers build 1x1 structures.
//
// DEVIATIONS (carried over): the per unit description text is a placeholder (the
// real strings need the unit data to lang key map, like the cargo/upgrade screens);
// the Path build along a path mode returns a flag but the path placement itself
// is unavailable; Help is a stub.

// buildEntry is one row in a builder's construction roster.
type buildEntry struct {
	t    assets.UnitType
	name string
}

// constructorBuild is the Constructor's roster: the large structures, in the exact
// order of the official manual's "Constructors can build:" list (page 28), which is
// the authoritative buildable set. C++: Builder_GetBuildableUnits(CONSTRCT) (Lua data).
//
// Row 7 "Gold Refinery" IS unit type COMMTWR (a legacy enum misnomer carried by both
// the original and the port): sprite GOLDR17, popup InitGoldRefinery, refines 5 raw
// gold into 5 credits/turn. The 1x1 "Gold Vault" (GOLDSM, gold storage) is the
// SEPARATE engineer building below. Intentionally absent (none are in the manual's
// constructor list): SUPRTPLT (Life Support), RECCENTR (Recreation Center) and
// SHIELDGN (Shield Generator) are alien derelict artifact buildings placed by the
// neutral alien faction, not buildable by the constructor.
var constructorBuild = []buildEntry{
	{assets.MiningSt, "Mining Station"}, {assets.PowerStn, "Power Station"},
	{assets.LightPlt, "Light Vehicle Plant"}, {assets.LandPlt, "Heavy Vehicle Plant"},
	{assets.AirPlt, "Air Units Plant"}, {assets.Shipyard, "Shipyard"},
	{assets.CommTwr, "Gold Refinery"},
	{assets.Depot, "Depot"}, {assets.Hangar, "Hangar"}, {assets.Dock, "Dock"},
	{assets.Habitat, "Habitat"}, {assets.Research, "Research Centre"},
	{assets.Greenhse, "Eco-sphere"}, {assets.TrainHal, "Training Hall"},
	{assets.Barracks, "Barracks"},
}

// engineerBuild is the Engineer's roster: the small (1x1) structures, in the
// screenshot's order. C++: Builder_GetBuildableUnits(ENGINEER) (Lua data).
var engineerBuild = []buildEntry{
	{assets.ADump, "Storage Unit"}, {assets.FDump, "Fuel Tank"},
	{assets.GoldSm, "Gold Vault"}, {assets.PowGen, "Power Generator"},
	{assets.Cnct4W, "Connector"}, {assets.Road, "Road"}, {assets.Radar, "Radar"},
	{assets.GunTurrt, "Gun Turret"}, {assets.AntiAir, "Anti Aircraft"},
	{assets.ArtyTrrt, "Artillery"}, {assets.AntiMssl, "Missile Launcher"},
	{assets.LandPad, "Landing Pad"}, {assets.Block, "Block"},
	{assets.Bridge, "Bridge"}, {assets.WtrPltfm, "Water Platform"},
}

// constructRoster returns the buildable roster for a builder type, plus whether
// it shows the Path button (engineer only). nil when the type is not a builder.
func constructRoster(builder assets.UnitType) ([]buildEntry, bool) {
	switch builder {
	case assets.Constrct:
		return constructorBuild, false
	case assets.Engineer:
		return engineerBuild, true
	default:
		return nil, false
	}
}

// The construction menu's result type is screendata.BuildSelection, shared with
// the gamescreen host (it was the local screendata.BuildSelection before the fold).

// turnsAndCost ports UnitInfo::GetTurnsToBuild: the number of turns and the total
// material a builder spends to build t at the requested speed, capping the speed
// each turn by what the (depleting) material allows. C++: unitinfo.cpp
// UnitInfo::GetTurnsToBuild(unit_type, speed, &turns).
func turnsAndCost(builder, t assets.UnitType, speed, storage int, base int) (turns, cost int) {
	rem := base
	local := storage
	for rem > 0 {
		sp := speed
		if lim := build.MaxBuildRate(builder, rem, local); sp > lim {
			sp = lim
		}
		if sp < 1 {
			sp = 1
		}
		r := economy.RawConsumptionRate(builder, sp)
		cost += r
		turns++
		local -= r
		rem -= sp
	}
	return turns, cost
}

// Panel geometry (shared with CARGOPIC; the build rate column + turns/cost
// replace the cargo screen's middle Purchased list and gold bars). Button
// positions are the exact MobileBuildMenu rects (buildmenu.cpp:829-840).
const (
	// Buildable list = the selector sub window (buildmenu.cpp:847-850
	// ulx/uly/lrx/lry). full = lrx-ulx is the width ReportStats_DrawListItem
	// wraps the name into.
	cbListX, cbListY, cbListLRY        = 483, 60, 429
	cbListW                            = 598 - 483 // 115 (full)
	cbRowH                             = 32
	cbIconCX                           = cbListX + 16            // icon centre (ReportStats_DrawListItem: ulx+16)
	cbNameX                            = cbListX + 35            // name text box (ulx+35)
	cbNameW                            = cbListW - 35            // 80 (full-35)
	cbTurnsRX                          = cbListX + 135           // turns number right edge (BuildUnitTypeSelector::Draw +135)
	cbPanelW                           = cbTurnsRX - cbListX + 4 // restore width: icon+name+turns column
	cbPrevX, cbPrevY, cbPrevW, cbPrevH = 11, 13, 300, 240        // portrait
	cbStatX, cbStatY, cbStatW, cbStatH = 11, 293, 250, 174       // stats
	// Turns/cost number boxes, right edges (buildmenu.cpp turns_background_x*
	// = (378,_,20,11), cost_background_x* = (414,_,30,11)); rows at y 349/374/399.
	cbTurnsRightX, cbCostRightX = 398, 444
)

// constructMenu is the construction picker's screen state: the roster and
// selection, the rate gating, and the art/caches the draw methods share.
// C++: buildmenu.cpp - MobileBuildMenu.
type constructMenu struct {
	ctx     *Context
	builder assets.UnitType
	team    int
	storage int
	roster  []buildEntry

	sel, scroll int
	rate        int
	maxRate     int // the fastest rate the builder's storage/turns allow for the selected item

	pic      *sprites.Big
	f5       *fonts.Font
	buf      *gfx.Buffer
	sc       *units.SpriteCache
	at       []assets.UnitAttributes
	listRows int

	statIcons     map[string]*gfx.Buffer
	portraitCache map[assets.UnitType]*sprites.Big

	rateX1, rateX2, rateX4 gnw.ButtonID
}

// RunConstruction shows the construction menu for builder (Constrct/Engineer) of
// the given team holding storage material, and returns the chosen building +
// build rate. ok is false on Cancel/ESC.
//
// C++: BuildMenu_Menu -> MobileBuildMenu::Run (buildmenu.cpp).
func RunConstruction(ctx *Context, builder assets.UnitType, team, storage int) (screendata.BuildSelection, bool, error) {
	roster, hasPath := constructRoster(builder)
	if roster == nil {
		return screendata.BuildSelection{}, false, fmt.Errorf("construction: %s is not a builder", builder.Tag())
	}
	pic, err := ctx.loadBig("CONBUILD")
	if err != nil {
		return screendata.BuildSelection{}, false, err
	}
	ctx.drawBigAt(mainWindow, pic, 0, 0)
	ctx.Plat.SetPalette(&pic.Palette)

	oldFont := ctx.Sys.Fonts().GetFont()
	defer ctx.Sys.Fonts().SetFont(oldFont)

	m := &constructMenu{
		ctx:           ctx,
		builder:       builder,
		team:          team,
		storage:       storage,
		roster:        roster,
		rate:          1,
		maxRate:       1,
		pic:           pic,
		f5:            ctx.Sys.Fonts().Get(fonts.Font5),
		buf:           ctx.Sys.Buf(mainWindow),
		sc:            units.NewSpriteCache(ctx.Lib, &pic.Palette),
		at:            ctx.Assets.Attribs,
		listRows:      (cbListLRY - cbListY) / cbRowH, // 11
		statIcons:     map[string]*gfx.Buffer{},
		portraitCache: map[assets.UnitType]*sprites.Big{},
	}

	buttons := m.buildWidgets(hasPath)
	defer func() {
		for _, b := range buttons {
			ctx.Sys.DeleteButton(b)
		}
	}()

	m.drawList()
	m.drawDetail()
	m.refreshRates()
	ctx.Sys.MouseShow()

	for {
		key := ctx.Sys.GetInput()
		if key == -1 {
			continue
		}
		if sel, done, ok := m.handleKey(key); done {
			return sel, ok, nil
		}
	}
}

// buildWidgets lays out the construction menu's title captions and buttons: the
// list scroll pair, help, the Description toggle, the X1/X2/X4 radio rate set,
// the engineer's Path button, Cancel/Done, and the list click region. It returns
// the button ids the caller deletes on exit. C++: buildmenu.cpp MobileBuildMenu ctor.
func (m *constructMenu) buildWidgets(hasPath bool) []gnw.ButtonID {
	ctx := m.ctx
	// Title (plain flat green) + gold shadowed captions, matching the C++
	// Text_TextBox styles (buildmenu.cpp MobileBuildMenu ctor).
	ctx.Sys.Lock()
	blitInBox(m.buf, m.f5, ctx.text("3a76", "Construction Menu"), 327, 7, 158, 18, colorGreen, true)
	blitTextBoxShadowed(m.buf, m.f5, ctx.text("526e", "Description"), 209, 264, 80, 17, fonts.ColorGold, true)
	blitTextBoxShadowed(m.buf, m.f5, ctx.text("53cf", "Turns"), 368, 330, 41, 13, fonts.ColorGold, true)
	blitTextBoxShadowed(m.buf, m.f5, ctx.text("204a", "Cost"), 409, 330, 41, 13, fonts.ColorGold, true)
	ctx.Sys.Unlock()

	var buttons []gnw.ButtonID
	addImg := func(x, y int, up, down string, value int, sfx string) {
		if b, e := ctx.imageButton(mainWindow, x, y, up, down, "", 0, 0, false, value); e == nil {
			if sfx != "" {
				ctx.registerButtonSfx(b, sfx)
			}
			buttons = append(buttons, b)
		}
	}
	ctx.Sys.Fonts().SetFont(fonts.Font5)
	addPlate := func(x, y int, up, down, label string, value int, sfx string) gnw.ButtonID {
		b, e := ctx.plateButton(mainWindow, x, y, up, down, label, value)
		if e != nil {
			return -1
		}
		if sfx != "" {
			ctx.registerButtonSfx(b, sfx)
		}
		buttons = append(buttons, b)
		return b
	}
	addImg(471, 441, "BLDUP__U", "BLDUP__D", 1110, "KCARG0") // list up
	addImg(491, 441, "BLDDWN_U", "BLDDWN_D", 1111, "KCARG0") // list down
	addImg(370, 452, "BLDHLP_U", "BLDHLP_D", 1001, "NHELP0") // help (baked "?")
	// Description checkbox (toggle).
	if b, e := ctx.imageButton(mainWindow, 292, 264, "BLDDES_U", "BLDDES_D", "", 0, 0, true, 1200); e == nil {
		ctx.Sys.SetButtonToggle(b)
		ctx.Sys.SetRestState(b, unitDescriptionShown) // shared by the build and upgrade menus, starts on
		ctx.registerButtonSfx(b, "KCARG0")
		buttons = append(buttons, b)
	}
	m.rateX1 = addPlate(292, 345, "BLDBLD_U", "BLDBLD_D", ctx.text("6b67", "Build X1"), 1201, "KCARG0")
	m.rateX2 = addPlate(292, 369, "BLD2X_U", "BLD2X_D", ctx.text("68dc", "Build X2"), 1202, "KCARG0")
	m.rateX4 = addPlate(292, 394, "BLD4X_U", "BLD4X_D", ctx.text("8433", "Build X4"), 1204, "KCARG0")
	// The three build rate buttons are a radio set: the chosen rate stays pressed
	// (drawn with its dark orange "down" caption) instead of springing back up.
	// C++: win_group_radio_buttons over the X1/X2/X4 buttons (buildmenu.cpp).
	var rateBtns []gnw.ButtonID
	for _, b := range []gnw.ButtonID{m.rateX1, m.rateX2, m.rateX4} {
		if b >= 0 {
			rateBtns = append(rateBtns, b)
		}
	}
	if len(rateBtns) > 0 {
		ctx.Sys.GroupRadioButtons(rateBtns, nil)
		ctx.Sys.SetRestState(m.rateX1, true) // default to X1, matching rate = 1
	}
	if hasPath {
		addPlate(347, 427, "BLDPTH_U", "BLDPTH_D", ctx.text("62fa", "Path"), 1300, "KCARG0")
	}
	addPlate(307, 452, "BLDCAN_U", "BLDCAN_D", ctx.text("1976", "Cancel"), 1002, "NCANC0")
	addPlate(397, 452, "BLDONE_U", "BLDONE_D", ctx.text("11a6", "Done"), 1000, "NDONE0")
	// Click region over the list.
	if b := ctx.Sys.AddButton(mainWindow, cbListX, cbListY, cbListW, m.listRows*cbRowH,
		gnw.NoValue, gnw.NoValue, gnw.NoValue, 1500, nil, nil, nil, 0); b >= 0 {
		buttons = append(buttons, b)
	}
	return buttons
}

// handleKey processes one input key for the construction menu. done is true when
// the menu should close, returning sel + ok to the caller; otherwise it mutates
// the selection/rate/scroll and repaints, and the loop keeps running.
func (m *constructMenu) handleKey(key int) (sel screendata.BuildSelection, done, ok bool) {
	switch {
	case key == 1000 || key == platform.KeyReturn: // Done: build at the selected rate
		if m.sel < len(m.roster) {
			return screendata.BuildSelection{Type: m.roster[m.sel].t, Rate: m.rate}, true, true
		}
	case key == 1002 || key == platform.KeyEscape: // Cancel
		return screendata.BuildSelection{}, true, false
	case key == 1001 || key == '?':
		Help(m.ctx, "CONSTRUCTOR_BUILD_SETUP")
	case key == 1200: // Description toggle
		unitDescriptionShown = !unitDescriptionShown // C++: button_description_rest_state is one static for all menus
		m.drawDetail()
	case key == 1201: // Build X1
		m.rate = 1
		m.drawRates()
	case key == 1202: // Build X2
		m.rate = 2
		m.drawRates()
	case key == 1204: // Build X4
		m.rate = 4
		m.drawRates()
	case key == 1300: // Path (engineer): build at the selected rate in path mode
		if m.sel < len(m.roster) {
			return screendata.BuildSelection{Type: m.roster[m.sel].t, Rate: m.rate, Path: true}, true, true
		}
	case key == 1110: // list up
		if m.scroll > 0 {
			m.scroll--
			m.drawList()
		}
	case key == 1111: // list down
		if m.scroll+m.listRows < len(m.roster) {
			m.scroll++
			m.drawList()
		}
	case key == 1500: // click a list row
		if r := (m.ctx.Plat.Mouse().Y - cbListY) / cbRowH; r >= 0 && m.scroll+r < len(m.roster) {
			// Picking a row is audible, even when it was already the selected one.
			// C++: UnitTypeSelector::Select plays KCARG0 (unittypeselector.cpp).
			m.ctx.playSfx("KCARG0")
			m.sel = m.scroll + r
			m.clampScroll()
			m.drawList()
			m.drawDetail()
			m.refreshRates() // the new item may allow fewer/more rates
		}
	}
	return screendata.BuildSelection{}, false, false
}

// turnsBase returns a buildable's base turns to build from ATTRIBS.
func (m *constructMenu) turnsBase(t assets.UnitType) int {
	if int(t) >= 0 && int(t) < len(m.at) {
		return m.at[t].TurnsToBuild
	}
	return 0
}

// statIcon caches a keyed gauge icon (I_HITS/I_ARMOR/...).
func (m *constructMenu) statIcon(tag string) *gfx.Buffer {
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

// loadPortrait caches a buildable's portrait art; nil when the type has none.
func (m *constructMenu) loadPortrait(t assets.UnitType) *sprites.Big {
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

// drawList repaints the buildable list: icon + name + turns (red when the
// builder can't afford the build at rate 1). C++: BuildUnitTypeSelector::Draw.
func (m *constructMenu) drawList() {
	m.ctx.Sys.Lock()
	defer m.ctx.Sys.Unlock()
	m.buf.Sub(cbListX, cbListY, cbPanelW, m.listRows*cbRowH).Copy(m.pic.Buffer().Sub(cbListX, cbListY, cbPanelW, m.listRows*cbRowH))
	rate1 := economy.RawConsumptionRate(m.builder, 1)
	for r := 0; r < m.listRows && m.scroll+r < len(m.roster); r++ {
		e := m.roster[m.scroll+r]
		y := cbListY + r*cbRowH
		m.sc.DrawIcon(m.buf, e.t, m.team, cbIconCX, y+16)
		// Name: wrapped to nameW, the whole block vertically centred in
		// the 32px row (C++: ReportStats_DrawListItem -> Text_TextBox(name,
		// ulx+35, uly, full-35, 32, color, false) auto wraps and centres vertically).
		lines := wrapText(m.f5, e.name, cbNameW)
		lh := m.f5.Height + 1
		top := y + (cbRowH-len(lines)*lh)/2
		for li, ln := range lines {
			blitInBox(m.buf, m.f5, ln, cbNameX, top+li*lh, cbNameW, lh, cargoTan, false)
		}
		tn := m.turnsBase(e.t)
		col := cargoTan
		if tn*rate1 > m.storage { // can't afford -> red (insufficient material)
			col = colorRed
		}
		s := fmt.Sprintf("%d", tn)
		blitInBox(m.buf, m.f5, s, cbTurnsRX-m.f5.Width(s), y, m.f5.Width(s), cbRowH, col, false)
		if m.scroll+r == m.sel {
			drawSelMarker(m.buf, cbListX, y, cbRowH)
		}
	}
}

// drawDetail repaints the portrait + stat rows for the selected buildable.
func (m *constructMenu) drawDetail() {
	m.ctx.Sys.Lock()
	defer m.ctx.Sys.Unlock()
	m.buf.Sub(cbPrevX, cbPrevY, cbPrevW, cbPrevH).Copy(m.pic.Buffer().Sub(cbPrevX, cbPrevY, cbPrevW, cbPrevH))
	m.buf.Sub(cbStatX, cbStatY, cbStatW, cbStatH).Copy(m.pic.Buffer().Sub(cbStatX, cbStatY, cbStatW, cbStatH))
	if m.sel >= len(m.roster) {
		return
	}
	t := m.roster[m.sel].t
	drawBuildPortrait(m.buf, m.f5, m.sc, t, m.team, m.roster[m.sel].name, cbPrevX, cbPrevY, cbPrevW, cbPrevH, m.ctx.unitDescription(t), m.loadPortrait)
	if int(t) >= len(m.at) {
		return
	}
	rows := buildStatRows(game.NewUnitValues(m.at[t]), t, m.builder, m.turnsBase(t))
	drawBuildStatRows(m.buf, m.f5, rows, cbStatX, cbStatY, cbStatW, cbStatH, m.statIcon)
}

// drawRates repaints the turns + cost numbers beside each Build X1/X2/X4
// button (the selected rate in green, the rest tan). C++: AbstractBuildMenu's
// turns/cost ReportStats_DrawNumber block.
func (m *constructMenu) drawRates() {
	m.ctx.Sys.Lock()
	defer m.ctx.Sys.Unlock()
	m.buf.Sub(374, 347, 76, 66).Copy(m.pic.Buffer().Sub(374, 347, 76, 66))
	if m.sel >= len(m.roster) {
		return
	}
	base := m.turnsBase(m.roster[m.sel].t)
	drawBuildRates(m.buf, m.f5, m.roster[m.sel].t, m.builder, m.storage, m.maxRate, m.rate, base, [3]int{349, 374, 399}, cbTurnsRightX, cbCostRightX)
}

// refreshRates evaluates again which build rates the selected item allows and gates
// the X1/X2/X4 buttons accordingly: X1 is always live, X2 needs max>=2, X4 max>=4;
// the selected rate is clamped DOWN to what is reachable (never raised) and the
// radio "checked" state follows. Disabled buttons go inert (their turns/cost stay
// blank via drawRates). Called whenever the selection changes.
// C++: buildmenu.cpp - AbstractBuildMenu::Draw (max_build_rate gate + clamp).
func (m *constructMenu) refreshRates() {
	if m.sel >= len(m.roster) {
		return
	}
	m.maxRate = build.MaxBuildRate(m.builder, m.turnsBase(m.roster[m.sel].t), m.storage)
	// Clamp the selected rate down to the reachable maximum, moving the checked
	// radio button with it (vanilla only ever lowers build_rate, never raises it).
	if m.rate > m.maxRate {
		m.ctx.Sys.SetRestState(m.rateX2, false)
		m.ctx.Sys.SetRestState(m.rateX4, false)
		m.rate = m.maxRate
		switch m.rate {
		case 1:
			m.ctx.Sys.SetRestState(m.rateX1, true)
		case 2:
			m.ctx.Sys.SetRestState(m.rateX2, true)
		case 4:
			m.ctx.Sys.SetRestState(m.rateX4, true)
		}
	}
	// Enable only the reachable rates; X1 is always available.
	if m.maxRate >= 2 {
		m.ctx.Sys.EnableButton(m.rateX2)
	} else {
		m.ctx.Sys.DisableButton(m.rateX2)
	}
	if m.maxRate >= 4 {
		m.ctx.Sys.EnableButton(m.rateX4)
	} else {
		m.ctx.Sys.DisableButton(m.rateX4)
	}
	m.drawRates()
}

// clampScroll keeps the selected row inside the visible list window.
func (m *constructMenu) clampScroll() {
	if m.sel < m.scroll {
		m.scroll = m.sel
	}
	if m.sel >= m.scroll+m.listRows {
		m.scroll = m.sel - m.listRows + 1
	}
}
