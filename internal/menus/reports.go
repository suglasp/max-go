// reports.go : implements the in-game resource, unit, message, and score reports.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++: reportmenu.cpp - ReportMenu
package menus

import (
	"fmt"

	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/gnw"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/screendata"
	"github.com/suglasp/max-go/internal/sprites"
)

// Reports screen: a first slice of reportmenu.cpp (REP_FRME). A full screen
// panel (drawn into the main window with its own palette, like Save/Load) with
// four report tabs down the right (Units / Casualties / Score / Messages) eight
// category filter checkboxes, a scrollable list on the left, and Done / Help /
// scroll arrows. This slice implements the Units report (the player's units,
// filtered + paginated) and the Score tab (team standings); Casualties and
// Messages are placeholders for a later pass.
//
// The data is resolved once by the host and filtered locally as the checkboxes
// toggle (the same data in split the Transfer screen uses), so this package stays
// free of the sim. C++: ReportMenu (reportmenu.cpp).

// The Reports data transfer objects now live in internal/screendata, shared with
// the gamescreen host, so the cmd bridge no longer copies a parallel set.

const (
	reportUnits = iota
	reportCasualties
	reportScore
	reportMessages
)

// Report button value scheme.
const (
	rpType0   = 1600 // + tab index (0..3)
	rpFilter0 = 1610 // + filter index (0..7)
	rpUp      = 1620
	rpDown    = 1621
	rpDone    = 1622
	rpHelp    = 1623
	rpList    = 1624 // a click inside the list area (a Units row selects that unit)
)

// reportFilter is one category checkbox: its label, the active flag and its row.
type reportFilter struct {
	label string
	on    bool
	y     int
	/* // note: unused code in Go port
	// location was set in the filter table and never read; reportRowMatches tells the location filters by index.
	location bool
	*/
}

// Filter checkbox indices into reportMenu.filters.
const (
	rpFilterAir = iota
	rpFilterLand
	rpFilterSea
	rpFilterStationary
	rpFilterProduction
	rpFilterCombat
	rpFilterDamaged
	rpFilterStealthy
)

// List geometry (left panel of REP_FRME), from the original's list image.
//
// C++ ported : reportmenu.cpp - Image(20, 17, 459, 448), then row_counts[0] =
// height / 50 (8 rows) and the row pitch = height / row_counts[0] (56).
const (
	rpListX, rpListY = 20, 17
	rpListW, rpListH = 459, 448

	// A Units row is 50 tall and sits centred in its 56px pitch. The columns below
	// are offsets from the list's left edge, and are the original's to the pixel.
	rpRowH        = rpListH / rpUnitRows // 56
	rpUnitRows    = rpListH / rpRowBodyH // 8
	rpRowBodyH    = 50
	rpIconCenterX = 16 // the icon is centred on this, 25 down the row
	rpIconCenterY = 25
	rpNameX       = 35 // name box, 80 wide, wrapped over the row's full height
	rpNameW       = 80
	rpStatsX      = 117 // the stat block runs from here to rpStatsEndX
	rpStatsEndX   = 260
	rpCoordsX     = 265 // grid position, centred in 40
	rpCoordsW     = 40
	rpStatusX     = 310 // order/status text takes the rest of the row

	// One stat block holds four rows, and the readout, label and gauge columns
	// inside it. C++: ReportStats_DrawRow's 0 / 45 / 75 offsets.
	rpStatRows   = 4
	rpStatValueW = 45
	rpStatLabelW = 30
	rpStatIconX  = 75

	rpMsgRowH = 48

	// The Casualties tab: a 140 wide type column (icon + name) on the left, then one
	// column per team from rpCasCountX on, each rpCasColW wide.
	//
	// C++ ported : reportmenu.cpp - DrawCasualties (window_ulx += 145; width = (w - 145) / 4)
	rpCasNameW  = 140
	rpCasCountX = 145
	rpCasColW   = (rpListW - rpCasCountX) / 4
	rpCasHeadH  = 20
	rpCasRowH   = 36
	rpCasRows   = (rpListH - rpCasHeadH) / rpCasRowH
)

// reportMenu is the Reports screen state: the active tab, the scroll position,
// the filter set, and the art the draw methods share.
// C++: reportmenu.cpp - ReportMenu.
type reportMenu struct {
	ctx  *Context
	data screendata.ReportData

	activeType int
	scroll     int
	filters    []reportFilter

	pic            *sprites.Big
	f5             *fonts.Font // names, coords, status
	f2             *fonts.Font // the small stat block readouts
	upBox, downBox *gfx.Buffer

	typeBtns []gnw.ButtonID

	rowsPerPage    int
	msgRowsPerPage int
}

// RunReports runs the Reports screen until Done/ESC. It returns the id of a unit the
// player clicked in the Units list, or 0 when they just closed the screen - the caller
// selects that unit and centres the map on it, so the report is a way to FIND a unit and
// not only to read about one.
//
// C++: ReportMenu::ReportMenu + the get_input loop, ReportMenu::SelectUnit (reportmenu.cpp).
func RunReports(ctx *Context, data screendata.ReportData) (uint16, error) {
	pic, err := ctx.loadBig("REP_FRME")
	if err != nil {
		return 0, err
	}
	unchecked, _ := ctx.loadSimple("UNCHKED")
	checked, _ := ctx.loadSimple("CHECKED")
	var upBox, downBox *gfx.Buffer
	if unchecked != nil {
		upBox = unchecked.Buffer().Clone()
		upBox.KeyToZero(upBox.At(0, 0))
	}
	if checked != nil {
		downBox = checked.Buffer().Clone()
		downBox.KeyToZero(downBox.At(0, 0))
	}

	ctx.Plat.SetPalette(&pic.Palette)
	oldFont := ctx.Sys.Fonts().GetFont()
	ctx.Sys.Fonts().SetFont(fonts.Font5)
	defer ctx.Sys.Fonts().SetFont(oldFont)

	m := &reportMenu{
		ctx:        ctx,
		data:       data,
		activeType: reportUnits,
		// Filter checkboxes: the four location filters default on, the four attribute
		// filters off (reportmenu.cpp ButtonState defaults).
		filters: []reportFilter{
			{ctx.text("2ecc", "Air Units"), true /* , true: note: unused code in Go port, see reportFilter */, 218},
			{ctx.text("4a3b", "Land Units"), true /* , true: note: unused code in Go port, see reportFilter */, 236},
			{ctx.text("1c43", "Sea Units"), true /* , true: note: unused code in Go port, see reportFilter */, 254},
			{ctx.text("4393", "Stationary Units"), true /* , true: note: unused code in Go port, see reportFilter */, 272},
			{ctx.text("929b", "Production Units"), false /* , false: note: unused code in Go port, see reportFilter */, 312},
			{ctx.text("1f97", "Combat Units"), false /* , false: note: unused code in Go port, see reportFilter */, 330},
			{ctx.text("045b", "Damaged Units"), false /* , false: note: unused code in Go port, see reportFilter */, 348},
			{ctx.text("e5ae", "Stealthy Units"), false /* , false: note: unused code in Go port, see reportFilter */, 366},
		},
		pic:            pic,
		f5:             ctx.Sys.Fonts().Get(fonts.Font5),
		f2:             ctx.Sys.Fonts().Get(fonts.Font2),
		upBox:          upBox,
		downBox:        downBox,
		rowsPerPage:    rpListH / rpRowH,
		msgRowsPerPage: rpListH / rpMsgRowH,
	}

	ctx.drawBigAt(pic, 0, 0)

	buttons := m.buildWidgets()
	defer func() {
		for _, b := range buttons {
			ctx.Sys.DeleteButton(b)
		}
	}()

	m.setType(reportUnits)
	m.refresh()
	ctx.Sys.MouseShow()

	for {
		key := ctx.Sys.GetInput()
		if key < 0 {
			continue
		}
		if id, done := m.handleKey(key); done {
			return id, nil
		}
	}
}

// buildWidgets creates the tab column, the filter checkboxes, the list click
// region and the bottom controls, and returns their ids for the caller to delete.
//
// C++: ReportMenu::ReportMenu - the button table (reportmenu.cpp)
func (m *reportMenu) buildWidgets() []gnw.ButtonID {
	ctx := m.ctx

	var buttons []gnw.ButtonID

	// Four report type radio buttons.
	typeLabels := []string{
		ctx.text("4f24", "Units"), ctx.text("e5da", "Casualties"),
		ctx.text("1f25", "Score"), ctx.text("c8d2", "Messages"),
	}
	m.typeBtns = make([]gnw.ButtonID, 4)
	for i, lbl := range typeLabels {
		y := 71 + i*29
		if b, e := ctx.captionButton(509, y, "DPTMNUUP", "DPTMNUDN", "", lbl, rpType0+i); e == nil {
			ctx.registerButtonSfx(b, "MBUTT0")
			m.typeBtns[i] = b
			buttons = append(buttons, b)
		}
	}
	// Filter checkboxes: invisible click regions; the box face is drawn manually
	// so the checked state persists (UNCHKED vs CHECKED). Labels sit to the right.
	for i := range m.filters {
		if b := ctx.Sys.AddButton(mainWindow, 496, m.filters[i].y, 14, 14,
			gnw.NoValue, gnw.NoValue, gnw.NoValue, rpFilter0+i, nil, nil, nil, 0); b >= 0 {
			buttons = append(buttons, b)
		}
	}
	if b, e := ctx.captionButton(509, 398, "DPTMNUUP", "DPTMNUDN", "", ctx.text("27af", "Done"), rpDone); e == nil {
		ctx.registerButtonSfx(b, "NDONE0")
		buttons = append(buttons, b)
	}
	if b, e := ctx.captionButton(487, 426, "DPTUP_UP", "DPTUP_DN", "DPTUP_X", "", rpUp); e == nil {
		ctx.registerButtonSfx(b, "MBUTT0")
		buttons = append(buttons, b)
	}
	if b, e := ctx.captionButton(516, 426, "DPTDN_UP", "DPTDN_DN", "DPTDN_X", "", rpDown); e == nil {
		ctx.registerButtonSfx(b, "MBUTT0")
		buttons = append(buttons, b)
	}
	if b, e := ctx.captionButton(543, 427, "DPTHP_UP", "DPTHP_DN", "", "", rpHelp); e == nil {
		ctx.registerButtonSfx(b, "NHELP0")
		buttons = append(buttons, b)
	}
	// An invisible click region over the list, so a Units row can be clicked to select
	// that unit on the map. C++: the list is a clickable widget (ReportMenu::SelectUnit).
	if b := ctx.Sys.AddButton(mainWindow, rpListX, rpListY, rpListW, rpListH,
		gnw.NoValue, gnw.NoValue, gnw.NoValue, rpList, nil, nil, nil, 0); b >= 0 {
		buttons = append(buttons, b)
	}

	return buttons
}

// handleKey answers one input code. It reports the screen closing, along with
// the unit the player picked out of the Units list (0 when they just closed it).
//
// C++: ReportMenu::Run - the get_input switch (reportmenu.cpp)
func (m *reportMenu) handleKey(key int) (uint16, bool) {
	ctx := m.ctx

	switch {
	case key == rpDone || key == platform.KeyEscape || key == platform.KeyReturn:
		return 0, true
	case key == rpList:
		// A click on a Units row closes the report and hands the unit back, so the
		// caller can select it and centre the map on it.
		if id, ok := m.rowAt(ctx.Plat.Mouse().Y); ok {
			return id, true
		}
	case key == rpHelp || key == '?':
		Help(ctx, "REPORTS_SETUP")
	case key >= rpType0 && key < rpType0+4:
		m.setType(key - rpType0)
		m.drawList()
	case key >= rpFilter0 && key < rpFilter0+len(m.filters):
		m.filters[key-rpFilter0].on = !m.filters[key-rpFilter0].on
		m.scroll = 0
		m.refresh()
	case key == rpUp:
		if _, perPage := m.pageInfo(); m.scroll > 0 {
			m.scroll -= perPage
			if m.scroll < 0 {
				m.scroll = 0
			}
			m.drawList()
		}
	case key == rpDown:
		if count, perPage := m.pageInfo(); m.scroll+perPage < count {
			m.scroll += perPage
			m.drawList()
		}
	}

	return 0, false
}

// rowAt resolves a click at screen y into the unit whose Units row was hit. It answers
// only on the Units tab (the other tabs are readouts, not unit lists) and only when the
// row actually holds a unit.
func (m *reportMenu) rowAt(mouseY int) (uint16, bool) {
	if m.activeType != reportUnits {
		return 0, false
	}

	row := (mouseY - rpListY) / rpRowH

	if row < 0 || row >= m.rowsPerPage {
		return 0, false
	}

	rows := m.filterRows()
	idx := m.scroll + row

	if idx < 0 || idx >= len(rows) || rows[idx].UnitID == 0 {
		return 0, false
	}

	return rows[idx].UnitID, true
}

// filterRows applies the active checkboxes to the unit list.
func (m *reportMenu) filterRows() []screendata.ReportRow {
	var out []screendata.ReportRow
	for _, r := range m.data.Units {
		if reportRowMatches(r, m.filters[rpFilterAir].on, m.filters[rpFilterLand].on, m.filters[rpFilterSea].on,
			m.filters[rpFilterStationary].on, m.filters[rpFilterProduction].on, m.filters[rpFilterCombat].on,
			m.filters[rpFilterDamaged].on, m.filters[rpFilterStealthy].on) {
			out = append(out, r)
		}
	}
	return out
}

// drawCheckboxes paints the "Include:" / "Limit To:" group headers and each
// filter box (checked/unchecked) + its label, all in the gold shadowed caption
// style. C++: the Text_TextLine calls in the ReportMenu constructor.
func (m *reportMenu) drawCheckboxes() {
	buf := m.ctx.Sys.Buf(mainWindow)
	m.ctx.Sys.Lock()
	defer m.ctx.Sys.Unlock()
	blitTextLine(buf, m.f5, m.ctx.text("bbb4", "Include:"), 497, 206, 140, fonts.ColorGold, false)
	blitTextLine(buf, m.f5, m.ctx.text("28d5", "Limit To:"), 497, 298, 140, fonts.ColorGold, false)
	for _, fl := range m.filters {
		box := m.upBox
		if fl.on {
			box = m.downBox
		}
		if box != nil {
			w := min(box.W, buf.W-496)
			h := min(box.H, buf.H-fl.y)
			if w > 0 && h > 0 {
				buf.Sub(496, fl.y, w, h).TransCopy(box.Sub(0, 0, w, h))
			}
		}
		blitTextLine(buf, m.f5, fl.label, 517, fl.y+2, 120, fonts.ColorGold, false)
	}
}

// drawList repaints the left list area for the active report tab.
func (m *reportMenu) drawList() {
	buf := m.ctx.Sys.Buf(mainWindow)
	m.ctx.Sys.Lock()
	defer m.ctx.Sys.Unlock()
	buf.Sub(rpListX, rpListY, rpListW, rpListH).Copy(m.pic.Buffer().Sub(rpListX, rpListY, rpListW, rpListH))

	switch m.activeType {
	case reportUnits:
		rows := m.filterRows()
		if m.scroll > len(rows)-1 {
			m.scroll = 0
		}

		// The 50px row body is centred in its pitch, so the first one starts a few
		// pixels into the list. C++: window_uly = image ULY + (row_limit - 50) / 2.
		top := rpListY + (rpRowH-rpRowBodyH)/2

		for i := 0; i < m.rowsPerPage; i++ {
			idx := m.scroll + i
			if idx >= len(rows) {
				break
			}

			m.drawUnitRow(buf, rows[idx], top+i*rpRowH)
		}
	case reportScore:
		drawScoreGraph(buf, m.f5, m.data.Teams, rpListX, rpListY, rpListW, rpListH)
	case reportCasualties:
		m.drawCasualties(buf)
	case reportMessages:
		if len(m.data.Messages) == 0 {
			blitInBox(buf, m.f5, m.ctx.text("c8d3", "No messages."), rpListX, rpListY+rpListH/2-8, rpListW, 16, colorWhite, true)
			break
		}
		if m.scroll > len(m.data.Messages)-1 {
			m.scroll = 0
		}
		// Columns: [icon] [red coords] [text, wrapped to two lines].
		const icoX, coordX, textX = 2, 44, 92
		for i := 0; i < m.msgRowsPerPage; i++ {
			idx := m.scroll + i
			if idx >= len(m.data.Messages) {
				break
			}
			msg := m.data.Messages[idx]
			y := rpListY + i*rpMsgRowH
			if msg.Icon != nil {
				blitIcon(buf, msg.Icon, rpListX+icoX, y+(rpMsgRowH-msg.Icon.H)/2)
			}
			if msg.Coords != "" {
				blitInBox(buf, m.f5, msg.Coords, rpListX+coordX, y, textX-coordX-2, rpMsgRowH, colorRed, false)
			}
			lines := wrapLines(m.f5, msg.Text, rpListW-textX-4, 2)
			ty := y + (rpMsgRowH-len(lines)*(m.f5.Height+1))/2
			for _, ln := range lines {
				blitInBox(buf, m.f5, ln, rpListX+textX, ty, rpListW-textX-4, m.f5.Height+1, colorWhite, false)
				ty += m.f5.Height + 1
			}
		}
	}
}

// drawCasualties paints the Casualties tab: a team coloured header per column, then one
// row per unit type anybody has lost - its icon and name on the left, and what each team
// lost of it in that team's colour. A type nobody has lost never appears, so the tab is
// empty at the start of a game and fills up as the war does.
//
// C++ ported : reportmenu.cpp - ReportMenu::DrawCasualties
func (m *reportMenu) drawCasualties(buf *gfx.Buffer) {
	for i, t := range m.data.Teams {
		x := rpListX + rpCasCountX + i*rpCasColW
		blitInBox(buf, m.f5, t.Name, x, rpListY, rpCasColW, rpCasHeadH, t.Color, true)
	}

	if len(m.data.Casualties) == 0 {
		blitInBox(buf, m.f5, m.ctx.text("bb8f", "No casualties."),
			rpListX, rpListY+rpListH/2-8, rpListW, 16, colorWhite, true)

		return
	}

	if m.scroll > len(m.data.Casualties)-1 {
		m.scroll = 0
	}

	for i := 0; i < rpCasRows; i++ {
		idx := m.scroll + i

		if idx >= len(m.data.Casualties) {
			break
		}

		row := m.data.Casualties[idx]
		y := rpListY + rpCasHeadH + i*rpCasRowH

		if row.Icon != nil {
			blitIcon(buf, row.Icon, rpListX+rpIconCenterX-row.Icon.W/2, y+rpCasRowH/2-row.Icon.H/2)
		}

		blitInBox(buf, m.f5, row.Name, rpListX+rpNameX, y, rpCasNameW-rpNameX, rpCasRowH, 0xA2, false)

		for team, n := range row.Counts {
			if team >= len(m.data.Teams) {
				break
			}

			x := rpListX + rpCasCountX + team*rpCasColW
			blitInBox(buf, m.f5, fmt.Sprintf("%d", n), x, y, rpCasColW, rpCasRowH, m.data.Teams[team].Color, true)
		}
	}
}

// drawUnitRow draws one Units row at the given top: the icon, the wrapped display
// name, the stat block, the grid position, and the order status. The row body is 50
// tall and every column offset is the original's.
//
// C++ ported : reportmenu.cpp - ReportMenu::DrawUnits (one loop iteration)
func (m *reportMenu) drawUnitRow(buf *gfx.Buffer, r screendata.ReportRow, y int) {
	if r.Icon != nil {
		// The original centres the icon on a point rather than boxing it.
		blitIcon(buf, r.Icon, rpListX+rpIconCenterX-r.Icon.W/2, y+rpIconCenterY-r.Icon.H/2)
	}

	// The name wraps inside a narrow column ("Mk I Gold Refinery 1" takes two lines)
	// and the block of lines is centred against the row.
	lineH := m.f5.Height + 1
	lines := wrapLines(m.f5, r.Name, rpNameW, rpRowBodyH/lineH)
	nameY := y + (rpRowBodyH-len(lines)*lineH)/2

	for i, ln := range lines {
		blitInBox(buf, m.f5, ln, rpListX+rpNameX, nameY+i*lineH, rpNameW, lineH, 0xA2, false)
	}

	m.drawStatBlock(buf, r.Stats, rpListX+rpStatsX, y, rpStatsEndX-rpStatsX, rpRowBodyH)

	blitInBox(buf, m.f5, r.Coords, rpListX+rpCoordsX, y, rpCoordsW, rpRowBodyH, 0xA2, true)
	blitInBox(buf, m.f5, r.Status, rpListX+rpStatusX, y, rpListW-rpStatusX, rpRowBodyH, 0xA2, false)
}

// drawStatBlock stacks a row's stat lines in the block: each is a "cur/base" readout
// coloured by how full it is, its label, and a run of gauge icons showing the same
// thing. A line with no base is blank, which is how a unit with no ammo or no cargo
// leaves its slot empty.
//
// C++ ported : reportstats.cpp - ReportStats_DrawRowEx + ReportStats_DrawRow
func (m *reportMenu) drawStatBlock(buf *gfx.Buffer, stats []screendata.ReportStatRow, x, y, w, h int) {
	for i, st := range stats {
		if i >= rpStatRows || st.Base <= 0 || st.Factor <= 0 {
			continue
		}

		top, bottom := statLineSpan(y, h, i)
		rowH := bottom - top

		if rowH <= 0 {
			continue
		}

		if st.Line {
			// A hairline under the line, as wide as the block.
			buf.Sub(x, bottom, w-1, 1).Fill(0x3C)
		}

		cur := st.Cur

		if cur > st.Base {
			cur = st.Base
		}

		if cur < 0 {
			cur = 0
		}

		blitInBox(buf, m.f2, fmt.Sprintf("%d/%d", cur, st.Base), x, top, rpStatValueW, rowH, statColor(cur, st.Base), true)
		blitInBox(buf, m.f2, st.Label, x+rpStatValueW, top, rpStatLabelW, rowH, 0xA2, false)

		// The gauge counts icons, not raw points: the host already capped the factor
		// so a big stat still fits the column.
		iconX := x + rpStatIconX
		gaugeCur := (cur + st.Factor - 1) / st.Factor
		gaugeBase := (st.Base + st.Factor - 1) / st.Factor

		// The icons get the row less its bottom scanline, which is the hairline's. That
		// one pixel decides the whole look: a 7px icon in the 11px rows would otherwise
		// clear the stagger threshold and walk down the row instead of standing level.
		drawGaugePair(buf, st.Full, st.Empty, gaugeCur, gaugeBase, iconX, top, w-rpStatIconX, statGaugeBoxH(rowH), iconX, x+w, top+rowH)
	}
}

// statLineSpan returns the top and bottom scanline of stat line i in a block h pixels tall starting at y: the block is
// cut into rpStatRows lines, and each line is inset a pixel top and bottom.
func statLineSpan(y, h, i int) (top, bottom int) {
	return y + (h*i)/rpStatRows + 1, y + (h*(i+1))/rpStatRows - 1
}

// statGaugeBoxH is the icon box height of a stat line rowH tall: the line less its bottom scanline, the hairline's.
func statGaugeBoxH(rowH int) int {
	return rowH - 1
}

// pageInfo returns the scrollable item count and rows per page for the active
// tab (Units and Messages scroll; the others do not).
func (m *reportMenu) pageInfo() (count, perPage int) {
	switch m.activeType {
	case reportUnits:
		return len(m.filterRows()), m.rowsPerPage
	case reportCasualties:
		return len(m.data.Casualties), rpCasRows
	case reportMessages:
		return len(m.data.Messages), m.msgRowsPerPage
	}
	return 0, 1
}

// setType switches the active report tab and moves the pressed radio button.
func (m *reportMenu) setType(t int) {
	m.activeType = t
	m.scroll = 0
	for i, b := range m.typeBtns {
		m.ctx.Sys.SetRestState(b, i == t)
	}
}

// refresh repaints both the checkbox column and the list for the active tab.
func (m *reportMenu) refresh() {
	m.drawCheckboxes()
	m.drawList()
}

// reportRowMatches reports whether a unit row passes the Units filters: the four
// location checkboxes are inclusive (the unit must match at least one enabled
// one), the four attribute checkboxes are restrictive (each, when on, excludes
// units lacking it); Stealthy keeps only the stealth units (Submarine, Commando). C++:
// ReportMenu::AddUnits + the active_units category set.
func reportRowMatches(r screendata.ReportRow, air, land, sea, stationary, production, combat, damaged, stealthy bool) bool {
	loc := (air && r.Air) || (land && r.Land) || (sea && r.Sea) || (stationary && r.Stationary)
	if !loc {
		return false
	}
	if production && !r.Producer {
		return false
	}
	if combat && !r.Combat {
		return false
	}
	if damaged && r.Hits >= r.MaxHits {
		return false
	}
	if stealthy && !r.Stealthy {
		return false
	}
	return true
}

// drawScoreGraph draws the Score tab: a team header line per team (points +
// Eco Spheres, in the team colour), axes, and a per team score history polyline.
// C++: reportmenu.cpp - ReportMenu score graph
// axisFraction is a y axis gridline position as a fraction (num/den) of the
// graph's maximum value.
type axisFraction struct{ num, den int }

func drawScoreGraph(buf *gfx.Buffer, f *fonts.Font, teams []screendata.ReportTeam, x, y, w, h int) {
	for i, t := range teams {
		line := fmt.Sprintf("%s: %d points, %d Eco-spheres", t.Name, t.Points, t.Ecospheres)
		blitInBox(buf, f, line, x+4, y+i*14, w-8, 14, t.Color, false)
	}

	// Graph area below the header lines.
	headerH := len(teams) * 14
	gx0 := x + 30
	gy0 := y + headerH + 6
	gy1 := y + h - 16
	gx1 := x + w - 10
	if gy0 >= gy1 {
		return // no space to draw
	}

	// Y axis scale: max value across all teams and slots (minimum 10).
	maxVal := int16(10)
	for _, t := range teams {
		for _, v := range t.ScoreHistory {
			if v > maxVal {
				maxVal = v
			}
		}
	}

	buf.DrawLine(gx0, gy0, gx0, gy1, 0xA2) // Y axis
	buf.DrawLine(gx0, gy1, gx1, gy1, 0xA2) // X axis

	// Y gridline labels at 0%, 50%, 100% of max.
	for _, frac := range []axisFraction{{0, 1}, {1, 2}, {1, 1}} {
		label := int(maxVal) * frac.num / frac.den
		yy := gy1 - (gy1-gy0)*frac.num/frac.den
		blitInBox(buf, f, fmt.Sprintf("%d", label), x+4, yy-7, 22, 14, 0xA2, false)
	}

	// Per team history polyline (50 samples, index 0=oldest, 49=newest).
	const slots = 50
	gw := gx1 - gx0
	gh := gy1 - gy0
	for _, t := range teams {
		// Skip teams that have no score history yet.
		hasData := false
		for _, v := range t.ScoreHistory {
			if v > 0 {
				hasData = true
				break
			}
		}
		if !hasData {
			continue
		}
		px, py := 0, 0
		for i, v := range t.ScoreHistory {
			nx := gx0 + i*gw/(slots-1)
			ny := gy1 - int(v)*gh/int(maxVal)
			if i > 0 {
				buf.DrawLine(px, py, nx, ny, uint8(t.Color))
			}
			px, py = nx, ny
		}
	}
}

// statColor returns the value colour for a cur/max readout (green over half,
// gold over a quarter, red below). C++: ReportStats colour tiers.
func statColor(cur, max int) int {
	if max <= 0 {
		return colorWhite
	}
	switch {
	case cur*4 <= max:
		return 0x01
	case cur*2 <= max:
		return 0xA2
	default:
		return 0x02
	}
}
