// factorybuild.go : implements the FACBUILD production queue editor.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++: factorymenu.cpp - FactoryMenu / FACBUILD interaction
package menus

import (
	"fmt"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/build"
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/gnw"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/screendata"
	"github.com/suglasp/max-go/internal/sprites"
	"github.com/suglasp/max-go/internal/units"
)

// FACBUILD lets a stationary factory edit its mobile-unit build queue. It shares CONBUILD's layout and adds
// queue, Add, Delete, and Repeat controls.
//
// DEVIATIONS: description text is a placeholder and Help is a stub. Queue rows show
// each unit's BASE turns (remaining turns of the head unit live in the sim). AI
// difficulty turn scaling not applied (no AI difficulty tiers yet, AI lane C4).
//
// C++: buildmenu.cpp - FactoryBuildMenu.

// factoryBuild is the production roster per factory (type + display name), ordered
// per the official manual (p.28-30). Two manual->engine name mappings: the manual's
// "Gunboat" is the engine's Battleship, and the Shipyard builds both the Cargo Ship
// ("Supply Boat") and the Sea Transport.
//
// C++: Builder_GetBuildableUnits for a STATIONARY factory.
var factoryBuild = map[assets.UnitType][]buildEntry{
	assets.LightPlt: { // Light Vehicle Plant (manual p.28): scouts + support + light AA
		{assets.Scout, "Scout Car"}, {assets.Surveyor, "Surveyor"},
		{assets.Engineer, "Engineer"}, {assets.Repair, "Repair Unit"},
		{assets.SplyTrck, "Supply Truck"}, {assets.FuelTrck, "Fuel Truck"},
		{assets.GoldTrck, "Gold Truck"}, {assets.SpFlak, "SP Flak"},
		{assets.MineLayr, "Mine Layer"}, {assets.Bulldozr, "Bulldozer"},
		{assets.ClnTrans, "Personnel Carrier"},
	},
	assets.LandPlt: { // Heavy Vehicle Plant (manual p.28): builders + armour + artillery
		{assets.Constrct, "Constructor"}, {assets.Scanner, "Scanner"},
		{assets.Tank, "Tank"}, {assets.Artillry, "Assault Gun"},
		{assets.RocktLch, "Rocket Launcher"}, {assets.MisslLch, "Missile Launcher"},
	},
	assets.AirPlt: { // Air Units Plant (manual p.30)
		{assets.Fighter, "Fighter"}, {assets.Bomber, "Bomber"},
		{assets.AirTrans, "Air Transport"}, {assets.Awac, "AWAC"},
	},
	assets.Shipyard: { // Shipyard (manual p.30); Gunboat=Battleship, Supply Boat=Cargo Ship
		{assets.FastBoat, "Escort"}, {assets.Corvette, "Corvette"},
		{assets.Battlshp, "Battleship"}, {assets.MsslBoat, "Missile Cruiser"},
		{assets.Submarne, "Submarine"}, {assets.CargoShp, "Cargo Ship"},
		{assets.SeaTrans, "Sea Transport"}, {assets.SeaMnLyr, "Sea Mine Layer"},
	},
	assets.TrainHal: { // Training Hall: infantry (Barracks is a service building, not a factory)
		{assets.Infantry, "Infantry"}, {assets.Commando, "Commando"},
	},
}

// Note (PLANNER/QA): the AI's factoryProducts (internal/ai/buildunits.go) keys
// infantry under Barracks and misplaces Scanner (Light) / SP Flak (Heavy) vs the
// manual; the player roster above follows the manual as the authority. Reconcile the AI map
// to one source.

// factoryRoster returns the production roster for a factory type, or nil if the
// type is not a factory.
func factoryRoster(factory assets.UnitType) []buildEntry {
	return factoryBuild[factory]
}

/* // note: unused code in Go port
// IsFactoryType reports whether a unit type is a factory the player can open a
// build menu for (it has a production roster).
func IsFactoryType(t assets.UnitType) bool {
	return factoryBuild[t] != nil
}
*/

// The factory build menu's result type is screendata.FactorySelection, shared with
// the gamescreen host (it was the local screendata.FactorySelection before the fold).

// Geometry. The Available list + portrait + stats panel share CONBUILD's rects
// (buildmenu.cpp); the rate column + turns/cost boxes sit lower than CONBUILD to
// make room for the queue controls, and the middle QUEUE list is FACBUILD only.
const (
	// Available roster list (selector sub window, buildmenu.cpp:1010-1013).
	fbListX, fbListY, fbListLRY = 483, 60, 429
	fbListW                     = 598 - 483 // 115
	fbRowH                      = 32
	fbIconCX                    = fbListX + 16
	fbNameX                     = fbListX + 35
	fbNameW                     = fbListW - 35
	fbTurnsRX                   = fbListX + 135
	fbAvailPanelW               = fbTurnsRX - fbListX + 4
	// Build QUEUE list (cargo_selector sub window, buildmenu.cpp:1022-1025).
	fbQueueX, fbQueueY, fbQueueLRY     = 337, 60, 280
	fbQueueW                           = 453 - 337 // 116
	fbQIconCX                          = fbQueueX + 16
	fbQNameX                           = fbQueueX + 35
	fbQNameW                           = fbQueueW - 35
	fbQTurnsRX                         = fbQueueX + 113
	fbQueuePanelW                      = fbQTurnsRX - fbQueueX + 4
	fbPrevX, fbPrevY, fbPrevW, fbPrevH = 11, 13, 300, 240  // portrait
	fbStatX, fbStatY, fbStatW, fbStatH = 11, 293, 250, 174 // stats
	// Rate turns/cost number boxes (right edges); rows at y 376/401/426.
	fbTurnsRightX, fbCostRightX = 398, 444
)

// factoryBuildMenu is the FACBUILD screen state: the roster and the edited
// queue, the two list selections, the rate gating, and the shared art/caches.
// C++: buildmenu.cpp - FactoryBuildMenu.
type factoryBuildMenu struct {
	ctx     *Context
	factory assets.UnitType
	team    int
	storage int
	roster  []buildEntry

	queueEdit  []assets.UnitType // working copies (Cancel discards these)
	repeatEdit bool

	selAvail, scrollAvail int
	selQueue, scrollQueue int
	rate, maxRate         int

	pic                  *sprites.Big
	f5                   *fonts.Font
	buf                  *gfx.Buffer
	sc                   *units.SpriteCache
	at                   []assets.UnitAttributes
	availRows, queueRows int

	rateX1, rateX2, rateX4 gnw.ButtonID

	dbl menuDoubleClick
	/* // note: unused code in Go port
	// the per menu cache moved to Context (statIcon / loadPortrait), shared by every menu.
	statIcons map[string]*gfx.Buffer
	*/
	/* // note: unused code in Go port
	// the per menu cache moved to Context (statIcon / loadPortrait), shared by every menu.
	portraitCache map[assets.UnitType]*sprites.Big
	*/
}

// RunFactoryBuild shows the FACBUILD menu for a factory of the given team holding
// storage material, seeded with the factory's current queue + repeat flag, and
// returns the edited queue + rate + repeat. ok is false on Cancel/ESC (no commit).
//
// C++: BuildMenu_Menu -> FactoryBuildMenu::Run (buildmenu.cpp).
func RunFactoryBuild(ctx *Context, factory assets.UnitType, team, storage int, queue []assets.UnitType, repeat bool) (screendata.FactorySelection, bool, error) {
	roster := factoryRoster(factory)
	if roster == nil {
		return screendata.FactorySelection{}, false, fmt.Errorf("factory build: %s is not a factory", factory.Tag())
	}
	pic, err := ctx.loadBig("FACBUILD")
	if err != nil {
		return screendata.FactorySelection{}, false, err
	}
	ctx.drawBigAt(pic, 0, 0)
	ctx.Plat.SetPalette(&pic.Palette)

	oldFont := ctx.Sys.Fonts().GetFont()
	defer ctx.Sys.Fonts().SetFont(oldFont)

	m := &factoryBuildMenu{
		ctx:     ctx,
		factory: factory,
		team:    team,
		storage: storage,
		roster:  roster,
		// Working copies (Cancel discards these).
		queueEdit:  append([]assets.UnitType(nil), queue...),
		repeatEdit: repeat,
		rate:       1,
		maxRate:    1,
		pic:        pic,
		f5:         ctx.Sys.Fonts().Get(fonts.Font5),
		buf:        ctx.Sys.Buf(mainWindow),
		sc:         units.NewSpriteCache(ctx.Lib, &pic.Palette),
		at:         ctx.Assets.Attribs,
		availRows:  (fbListLRY - fbListY) / fbRowH,   // 11
		queueRows:  (fbQueueLRY - fbQueueY) / fbRowH, // 6
		/* statIcons: map[string]*gfx.Buffer{}: note: unused code in Go port, see the struct */
		/* portraitCache: map[assets.UnitType]*sprites.Big{}: note: unused code in Go port, see the struct */
	}
	buttons := m.buildWidgets()
	defer func() {
		for _, b := range buttons {
			ctx.Sys.DeleteButton(b)
		}
	}()

	m.drawAvail()
	m.drawQueue()
	m.drawDetail()
	m.refreshRates()
	ctx.Sys.FlushInput() // menu->menu handoff: don't let a leaked click/Return fire here
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

// buildWidgets lays out the FACBUILD title captions and buttons: the two list
// scroll pairs, help, Add/Delete, the Description and Repeat toggles, the
// X1/X2/X4 radio rate set, Cancel/Done, and the two list click regions. It
// returns the button ids the caller deletes on exit. C++: buildmenu.cpp
// FactoryBuildMenu ctor.
func (m *factoryBuildMenu) buildWidgets() []gnw.ButtonID {
	ctx := m.ctx
	f5 := m.f5
	buf := m.buf

	// Title (plain flat green) + gold shadowed captions, matching the C++
	// Text_TextBox styles (buildmenu.cpp FactoryBuildMenu ctor).
	ctx.Sys.Lock()
	blitInBox(buf, f5, ctx.text("faaf", "Production Menu"), 327, 7, 158, 18, colorGreen, true)
	blitTextBoxShadowed(buf, f5, ctx.text("207f", "Description"), 209, 264, 80, 17, fonts.ColorGold, true)
	blitTextBoxShadowed(buf, f5, ctx.text("b6b7", "Repeat"), 395, 322, 50, 17, fonts.ColorGold, true)
	blitTextBoxShadowed(buf, f5, ctx.text("2180", "Turns"), 368, 357, 41, 13, fonts.ColorGold, true)
	blitTextBoxShadowed(buf, f5, ctx.text("064b", "Cost"), 409, 357, 41, 13, fonts.ColorGold, true)
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

	addImg(471, 441, "BLDUP__U", "BLDUP__D", 1110, "KCARG0") // available up
	addImg(491, 441, "BLDDWN_U", "BLDDWN_D", 1111, "KCARG0") // available down
	addImg(327, 293, "BLDUP__U", "BLDUP__D", 1120, "KCARG0") // queue up
	addImg(347, 293, "BLDDWN_U", "BLDDWN_D", 1121, "KCARG0") // queue down
	addImg(370, 452, "BLDHLP_U", "BLDHLP_D", 1001, "NHELP0") // help
	addPlate(412, 293, "BLDDEL_U", "BLDDEL_D", ctx.text("e2d0", "Delete"), 1131, "MBUTT0")
	addPlate(548, 441, "BLDBLD_U", "BLDBLD_D", ctx.text("bdd2", "Add"), 1130, "MBUTT0")
	// Description checkbox (toggle).
	if b, e := ctx.imageButton(mainWindow, 292, 264, "BLDDES_U", "BLDDES_D", "", 0, 0, true, 1200); e == nil {
		ctx.Sys.SetButtonToggle(b)
		ctx.Sys.SetRestState(b, unitDescriptionShown) // shared by the build and upgrade menus, starts on
		ctx.registerButtonSfx(b, "KCARG0")
		buttons = append(buttons, b)
	}
	// Repeat checkbox (toggle), restored from the factory's current state.
	if b, e := ctx.imageButton(mainWindow, 447, 322, "BLDREP_U", "BLDREP_D", "", 0, 0, true, 1132); e == nil {
		ctx.Sys.SetButtonToggle(b)
		ctx.Sys.SetRestState(b, m.repeatEdit)
		ctx.registerButtonSfx(b, "KCARG0")
		buttons = append(buttons, b)
	}
	m.rateX1 = addPlate(292, 371, "BLDBLD_U", "BLDBLD_D", ctx.text("6b67", "Build X1"), 1201, "KCARG0")
	m.rateX2 = addPlate(292, 396, "BLD2X_U", "BLD2X_D", ctx.text("68dc", "Build X2"), 1202, "KCARG0")
	m.rateX4 = addPlate(292, 421, "BLD4X_U", "BLD4X_D", ctx.text("8433", "Build X4"), 1204, "KCARG0")
	var rateBtns []gnw.ButtonID
	for _, b := range []gnw.ButtonID{m.rateX1, m.rateX2, m.rateX4} {
		if b >= 0 {
			rateBtns = append(rateBtns, b)
		}
	}
	if len(rateBtns) > 0 {
		ctx.Sys.GroupRadioButtons(rateBtns, nil)
		ctx.Sys.SetRestState(m.rateX1, true)
	}
	addPlate(307, 452, "BLDCAN_U", "BLDCAN_D", ctx.text("1976", "Cancel"), 1002, "NCANC0")
	addPlate(397, 452, "BLDONE_U", "BLDONE_D", ctx.text("11a6", "Done"), 1000, "NDONE0")
	// Click regions over the two lists.
	if b := ctx.Sys.AddButton(mainWindow, fbListX, fbListY, fbListW, m.availRows*fbRowH,
		gnw.NoValue, gnw.NoValue, gnw.NoValue, 1500, nil, nil, nil, 0); b >= 0 {
		buttons = append(buttons, b)
	}
	if b := ctx.Sys.AddButton(mainWindow, fbQueueX, fbQueueY, fbQueueW, m.queueRows*fbRowH,
		gnw.NoValue, gnw.NoValue, gnw.NoValue, 1501, nil, nil, nil, 0); b >= 0 {
		buttons = append(buttons, b)
	}
	return buttons
}

// handleKey processes one input key for the FACBUILD menu. done is true when the
// menu should close, returning sel + ok to the caller; otherwise it edits the
// queue/rate/scroll and repaints, and the loop keeps running.
func (m *factoryBuildMenu) handleKey(key int) (sel screendata.FactorySelection, done, ok bool) {
	ctx := m.ctx
	switch {
	case key == 1000 || key == platform.KeyReturn: // Done: commit the queue
		return screendata.FactorySelection{Queue: m.queueEdit, Rate: m.rate, Repeat: m.repeatEdit}, true, true
	case key == 1002 || key == platform.KeyEscape: // Cancel
		return screendata.FactorySelection{}, true, false
	case key == 1001 || key == '?':
		Help(ctx, "FACTORY_BUILD_SETUP")
	case key == 1130: // Add the selected Available unit to the queue tail
		m.addSelected()
	case key == 1131: // Delete: remove the selected queued unit
		m.deleteSelected()
	case key == 1132: // Repeat toggle
		m.repeatEdit = !m.repeatEdit
	case key == 1200: // Description toggle
		unitDescriptionShown = !unitDescriptionShown // C++: button_description_rest_state is one static for all menus
		m.drawDetail()
	case key == 1201:
		m.rate = 1
		m.drawRates()
	case key == 1202:
		m.rate = 2
		m.drawRates()
	case key == 1204:
		m.rate = 4
		m.drawRates()
	case key == 1110: // available up
		if m.scrollAvail > 0 {
			m.scrollAvail--
			m.drawAvail()
		}
	case key == 1111: // available down
		if m.scrollAvail+m.availRows < len(m.roster) {
			m.scrollAvail++
			m.drawAvail()
		}
	case key == 1120: // queue up
		if m.scrollQueue > 0 {
			m.scrollQueue--
			m.drawQueue()
		}
	case key == 1121: // queue down
		if m.scrollQueue+m.queueRows < len(m.queueEdit) {
			m.scrollQueue++
			m.drawQueue()
		}
	case key == 1500: // click an Available row
		if r := (ctx.Plat.Mouse().Y - fbListY) / fbRowH; r >= 0 && m.scrollAvail+r < len(m.roster) {
			// Picking a row is audible, even when it was already the selected one.
			// C++: UnitTypeSelector::Select plays KCARG0 (unittypeselector.cpp).
			ctx.playSfx()
			m.selAvail = m.scrollAvail + r
			m.clampAvailScroll()
			m.drawAvail()
			m.drawDetail()
			m.refreshRates()
			if m.dbl.Click(key, m.selAvail, platform.Ticks()) {
				m.addSelected()
			}
		}
	case key == 1501: // click a queue row
		if r := (ctx.Plat.Mouse().Y - fbQueueY) / fbRowH; r >= 0 && m.scrollQueue+r < len(m.queueEdit) {
			// The queue is a selector too (C++ buildmenu.cpp cargo_selector), so it
			// answers a pick with the same KCARG0.
			ctx.playSfx()
			m.selQueue = m.scrollQueue + r
			m.drawQueue()
			if m.dbl.Click(key, m.selQueue, platform.Ticks()) {
				m.deleteSelected()
			}
		}
	}
	return screendata.FactorySelection{}, false, false
}

// turnsBase returns a unit's base turns to build from ATTRIBS.
func (m *factoryBuildMenu) turnsBase(t assets.UnitType) int {
	if int(t) >= 0 && int(t) < len(m.at) {
		return m.at[t].TurnsToBuild
	}
	return 0
}

/* // note: unused code in Go port
// the per menu cache moved to Context (statIcon / loadPortrait), shared by every menu.
// statIcon caches a keyed gauge icon (I_HITS/I_ARMOR/...).
func (m *factoryBuildMenu) statIcon(tag string) *gfx.Buffer {
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
*/

/* // note: unused code in Go port
// the per menu cache moved to Context (statIcon / loadPortrait), shared by every menu.
// loadPortrait caches a unit's portrait art; nil when the type has none.
func (m *factoryBuildMenu) loadPortrait(t assets.UnitType) *sprites.Big {
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
*/

// drawAvail repaints the Available roster list (icon + name + base turns).
// Factory units are never drawn red (only a mobile builder's unaffordable rows
// are). C++: BuildUnitTypeSelector::Draw.
func (m *factoryBuildMenu) drawAvail() {
	m.ctx.Sys.Lock()
	defer m.ctx.Sys.Unlock()
	m.buf.Sub(fbListX, fbListY, fbAvailPanelW, m.availRows*fbRowH).Copy(m.pic.Buffer().Sub(fbListX, fbListY, fbAvailPanelW, m.availRows*fbRowH))
	for r := 0; r < m.availRows && m.scrollAvail+r < len(m.roster); r++ {
		e := m.roster[m.scrollAvail+r]
		y := fbListY + r*fbRowH
		m.sc.DrawIcon(m.buf, e.t, m.team, fbIconCX, y+16)
		lines := wrapLines(m.f5, e.name, fbNameW, 0)
		lh := m.f5.Height + 1
		top := y + (fbRowH-len(lines)*lh)/2
		for li, ln := range lines {
			blitInBox(m.buf, m.f5, ln, fbNameX, top+li*lh, fbNameW, lh, cargoTan, false)
		}
		s := fmt.Sprintf("%d", m.turnsBase(e.t))
		blitInBox(m.buf, m.f5, s, fbTurnsRX-m.f5.Width(s), y, m.f5.Width(s), fbRowH, cargoTan, false)
		if m.scrollAvail+r == m.selAvail {
			drawSelMarker(m.buf, fbListX, y, fbRowH)
		}
	}
}

// drawQueue repaints the build QUEUE list (icon + name + base turns), the head
// (index 0) being the unit currently in production. C++: the cargo_selector Draw.
func (m *factoryBuildMenu) drawQueue() {
	m.ctx.Sys.Lock()
	defer m.ctx.Sys.Unlock()
	m.buf.Sub(fbQueueX, fbQueueY, fbQueuePanelW, m.queueRows*fbRowH).Copy(m.pic.Buffer().Sub(fbQueueX, fbQueueY, fbQueuePanelW, m.queueRows*fbRowH))
	for r := 0; r < m.queueRows && m.scrollQueue+r < len(m.queueEdit); r++ {
		t := m.queueEdit[m.scrollQueue+r]
		y := fbQueueY + r*fbRowH
		m.sc.DrawIcon(m.buf, t, m.team, fbQIconCX, y+16)
		name := unitName(m.roster, t)
		lines := wrapLines(m.f5, name, fbQNameW, 0)
		lh := m.f5.Height + 1
		top := y + (fbRowH-len(lines)*lh)/2
		for li, ln := range lines {
			blitInBox(m.buf, m.f5, ln, fbQNameX, top+li*lh, fbQNameW, lh, cargoTan, false)
		}
		s := fmt.Sprintf("%d", m.turnsBase(t))
		blitInBox(m.buf, m.f5, s, fbQTurnsRX-m.f5.Width(s), y, m.f5.Width(s), fbRowH, cargoTan, false)
		if m.scrollQueue+r == m.selQueue {
			drawSelMarker(m.buf, fbQueueX, y, fbRowH)
		}
	}
}

// drawDetail repaints the portrait + stat rows for the selected Available unit.
func (m *factoryBuildMenu) drawDetail() {
	m.ctx.Sys.Lock()
	defer m.ctx.Sys.Unlock()
	m.buf.Sub(fbPrevX, fbPrevY, fbPrevW, fbPrevH).Copy(m.pic.Buffer().Sub(fbPrevX, fbPrevY, fbPrevW, fbPrevH))
	m.buf.Sub(fbStatX, fbStatY, fbStatW, fbStatH).Copy(m.pic.Buffer().Sub(fbStatX, fbStatY, fbStatW, fbStatH))
	if m.selAvail >= len(m.roster) {
		return
	}
	t := m.roster[m.selAvail].t
	drawBuildPortrait(m.buf, m.f5, m.sc, t, m.team, m.roster[m.selAvail].name, fbPrevX, fbPrevY, fbPrevW, fbPrevH, m.ctx.unitDescription(t), m.ctx.loadPortrait)
	if int(t) >= len(m.at) {
		return
	}
	rows := buildStatRows(game.NewUnitValues(m.at[t]), t, m.factory, m.turnsBase(t))
	drawBuildStatRows(m.buf, m.f5, rows, fbStatX, fbStatY, fbStatW, fbStatH, m.ctx.statIcon)
}

// drawRates repaints the turns + cost numbers beside each Build X1/X2/X4 button
// for the selected Available unit (selected rate green, others tan; unreachable
// rates blank). C++: AbstractBuildMenu::Draw turns/cost block.
func (m *factoryBuildMenu) drawRates() {
	m.ctx.Sys.Lock()
	defer m.ctx.Sys.Unlock()
	m.buf.Sub(374, 374, 76, 66).Copy(m.pic.Buffer().Sub(374, 374, 76, 66))
	if m.selAvail >= len(m.roster) {
		return
	}
	base := m.turnsBase(m.roster[m.selAvail].t)
	drawBuildRates(m.buf, m.f5, m.roster[m.selAvail].t, m.factory, m.storage, m.maxRate, m.rate, base, [3]int{376, 401, 426}, fbTurnsRightX, fbCostRightX)
}

// refreshRates gates the X1/X2/X4 buttons by the selected unit's reachable rate
// (X1 always, X2 needs maxRate>=2, X4 >=4) and clamps the chosen rate down.
// C++: buildmenu.cpp - AbstractBuildMenu::Draw (max_build_rate gate).
func (m *factoryBuildMenu) refreshRates() {
	if m.selAvail >= len(m.roster) {
		return
	}
	m.maxRate = build.MaxBuildRate(m.factory, m.turnsBase(m.roster[m.selAvail].t), m.storage)
	if m.factory == assets.TrainHal {
		m.maxRate = 1 // the Training Hall trains at X1 only (C++ buildmenu.cpp:603)
	}
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

// clampAvailScroll keeps the Available selection inside the visible window.
func (m *factoryBuildMenu) clampAvailScroll() {
	if m.selAvail < m.scrollAvail {
		m.scrollAvail = m.selAvail
	}
	if m.selAvail >= m.scrollAvail+m.availRows {
		m.scrollAvail = m.selAvail - m.availRows + 1
	}
}

// clampQueueScroll keeps the queue selection valid and inside its window.
func (m *factoryBuildMenu) clampQueueScroll() {
	if len(m.queueEdit) == 0 {
		m.selQueue, m.scrollQueue = 0, 0
		return
	}
	if m.selQueue >= len(m.queueEdit) {
		m.selQueue = len(m.queueEdit) - 1
	}
	if m.selQueue < 0 {
		m.selQueue = 0
	}
	if m.selQueue < m.scrollQueue {
		m.scrollQueue = m.selQueue
	}
	if m.selQueue >= m.scrollQueue+m.queueRows {
		m.scrollQueue = m.selQueue - m.queueRows + 1
	}
}

// addSelected appends the selected Available unit to the queue tail.
func (m *factoryBuildMenu) addSelected() {
	if m.selAvail >= len(m.roster) {
		return
	}
	m.queueEdit = appendFactoryQueue(m.queueEdit, m.roster[m.selAvail].t)
	m.selQueue = len(m.queueEdit) - 1
	m.clampQueueScroll()
	m.drawQueue()
}

// deleteSelected removes the selected queued unit.
func (m *factoryBuildMenu) deleteSelected() {
	var ok bool
	m.queueEdit, ok = deleteFactoryQueueAt(m.queueEdit, m.selQueue)
	if !ok {
		return
	}
	m.clampQueueScroll()
	m.drawQueue()
}

func appendFactoryQueue(queue []assets.UnitType, t assets.UnitType) []assets.UnitType {
	return append(queue, t)
}

func deleteFactoryQueueAt(queue []assets.UnitType, index int) ([]assets.UnitType, bool) {
	if index < 0 || index >= len(queue) {
		return queue, false
	}
	return append(queue[:index], queue[index+1:]...), true
}

// unitName resolves a queued type's display name from the factory roster (falling
// back to the type tag for a type not in the roster).
func unitName(roster []buildEntry, t assets.UnitType) string {
	for _, e := range roster {
		if e.t == t {
			return e.name
		}
	}
	return t.Tag()
}
