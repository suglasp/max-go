// alloc.go : implements the resource-allocation screen for a mining complex.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package menus

import (
	"fmt"

	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/gnw"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/sprites"
)

// The ALLOCFRM screen shows available deposits, current production and use, and stored raw material, fuel, and gold.
// Arrows and bar clicks shift mining within the stations' capacity and local deposits. Production bars use a proportional fill,
// and economy.AddMining assigns spare capacity instead of reproducing AllocMenu_Optimize's full rebalance.

// allocRow describes one resource row.
type allocRow struct {
	cargo economy.CargoType
	label string
	uly   int
	bar   string
}

// allocMiningState snapshots one station's mining at menu open so Cancel can
// restore it.
type allocMiningState struct{ raw, fuel, gold, total int }

// allocDonor is one candidate resource to take a mining unit from when another
// resource needs capacity and the station is already at its total.
type allocDonor struct {
	cargo economy.CargoType
	mined int
}

// Production bar geometry: the bars start at barX and are barW wide, which the
// row widgets, the bar painters and the bar click handler all share.
const barX, barW = 174, 240

// allocMenu is the allocation screen state: the complex being edited, the three
// resource rows, and the panel and bar art the draw methods share. c is edited
// live, so snapshot holds each station's mining for the Cancel path.
// C++: allocmenu.cpp - AllocMenu.
type allocMenu struct {
	ctx        *Context
	win        gnw.WinID
	ox         int // window left edge, to map bar clicks
	c          *economy.Complex
	rmap       *economy.ResourceMap
	difficulty int

	rows     []allocRow
	snapshot map[*game.UnitInfo]allocMiningState

	pic  *sprites.Big
	bars []*gfx.Buffer
	tape *gfx.Buffer

	f1, f5 *fonts.Font
}

// tapeSpan returns the x offset and width of the hazard tape over a production
// bar's locked tail. The tape covers [lock..maxv] scaled to barW. When maxv <=
// 0, the resource has no deposit, so the tape covers the full bar. The width is
// capped to the tape sprite's own width.
//
// C++: CargoBarData::Draw (allocmenu.cpp) - the cargo_3 == 0 -> offset_x = rect_width branch.
func tapeSpan(maxv, lock, barW, tapeW int) (x, width int) {
	lockX := 0
	if maxv > 0 {
		lockX = barW * lock / maxv
		if lockX < 0 {
			lockX = 0
		}
		if lockX > barW {
			lockX = barW
		}
	}
	width = barW - lockX
	if width > tapeW {
		width = tapeW
	}
	if width < 0 {
		width = 0
	}
	return lockX, width
}

// RunAllocate runs the allocation screen for complex c. rmap and difficulty drive
// the deposit limits and usage; center is the window region to centre in.
//
// C++: AllocMenu::Run (allocmenu.cpp)
func RunAllocate(ctx *Context, c *economy.Complex, rmap *economy.ResourceMap, difficulty int, center gfx.Rect) error {
	if c == nil {
		Message(ctx, "No complex selected.")
		return nil
	}
	pic, err := ctx.loadBig("ALLOCFRM")
	if err != nil {
		return err
	}

	ox := max(center.ULX+(center.Width()+1-pic.W)/2, 0)
	oy := max(center.ULY+(center.Height()+1-pic.H)/2, 0)
	win := ctx.Sys.AddWindow(ox, oy, pic.W, pic.H, 0, gnw.WindowMoveOnTop|gnw.WindowModal)
	if win < 0 {
		return fmt.Errorf("menus: alloc window alloc failed")
	}
	defer ctx.Sys.DeleteWindow(win)

	oldFont := ctx.Sys.Fonts().GetFont()
	defer ctx.Sys.Fonts().SetFont(oldFont)

	m := &allocMenu{
		ctx:        ctx,
		win:        win,
		ox:         ox,
		c:          c,
		rmap:       rmap,
		difficulty: difficulty,
		pic:        pic,
		f5:         ctx.Sys.Fonts().Get(fonts.Font5),
		f1:         ctx.Sys.Fonts().Get(fonts.Font1),
		snapshot:   map[*game.UnitInfo]allocMiningState{},
	}
	m.rows = []allocRow{
		{economy.CargoRaw, ctx.text("8481", "Raw"), 70, "BARRAW"},
		{economy.CargoFuel, ctx.text("a80f", "Fuel"), 190, "BARFUEL"},
		{economy.CargoGold, ctx.text("dff1", "Gold"), 310, "BARGOLD"},
	}

	// Snapshot every station's mining so Cancel can restore it; the arrows edit
	// the complex directly.
	for _, u := range c.Members {
		m.snapshot[u] = allocMiningState{u.RawMining, u.FuelMining, u.GoldMining, u.TotalMining}
	}

	m.drawStaticText()

	buttons := m.buildWidgets()
	defer func() {
		for _, b := range buttons {
			ctx.Sys.DeleteButton(b)
		}
	}()

	m.drawBars()
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

// drawStaticText paints the panel background, the screen title and the three
// rows' Raw/Fuel/Gold, Usage and Reserve labels.
func (m *allocMenu) drawStaticText() {
	ctx, pic, f5 := m.ctx, m.pic, m.f5
	winBuf := ctx.Sys.Buf(m.win)
	ctx.Sys.Lock()
	winBuf.Sub(0, 0, pic.W, pic.H).Copy(pic.Buffer().Sub(0, 0, pic.W, pic.H))
	blitInBox(winBuf, f5, ctx.text("c771", "Allocation Menu"), 230, 6, 180, 20, colorGreen|fonts.FlagOutline, true)
	// Draw row labels in the x=42..120 box with FONT_5 and COLOR_GREEN, matching allocmenu.cpp's MENU_TITLE_ITEM_DEF table.
	for _, r := range m.rows {
		blitInBox(winBuf, f5, r.label, 42, r.uly+4, 78, 19, colorGreen|fonts.FlagOutline, true)
		blitInBox(winBuf, f5, ctx.text("1e19", "Usage"), 42, r.uly+41, 78, 19, colorGreen|fonts.FlagOutline, true)
		blitInBox(winBuf, f5, ctx.text("5ae6", "Reserve"), 42, r.uly+78, 78, 19, colorGreen|fonts.FlagOutline, true)
	}
	ctx.Sys.Unlock()
}

// buildWidgets loads the bar art and creates the per row arrows and bar click
// regions plus the bottom Help/Done row, returning the ids so the caller can
// delete them on exit.
func (m *allocMenu) buildWidgets() []gnw.ButtonID {
	ctx, win := m.ctx, m.win
	var buttons []gnw.ButtonID

	// Arrows on each row (left -1 / right +1) and a bar click region.
	bars := make([]*gfx.Buffer, len(m.rows))
	for i, r := range m.rows {
		if img, e := ctx.loadSimple(r.bar); e == nil {
			// Key out the bar art's transparent backdrop (its corner colour =
			// transparent_color) so the panel recess shows through instead of a
			// stray blue palette slot colour. C++: Image transparent_color.
			b := img.Buffer().Clone()
			b.KeyToZero(b.At(0, 0))
			bars[i] = b
		}
		if l, e := ctx.imageButton(win, 139, r.uly, "LFARO_OF", "LFARO_ON", "", 0, 0, false, 1000+i*10+1); e == nil {
			ctx.registerButtonSfx(l, "KCARG0")
			buttons = append(buttons, l)
		}
		if rt, e := ctx.imageButton(win, 421, r.uly, "RTARO_OF", "RTARO_ON", "", 0, 0, false, 1000+i*10+2); e == nil {
			ctx.registerButtonSfx(rt, "KCARG0")
			buttons = append(buttons, rt)
		}
		if bc := ctx.Sys.AddButton(win, barX, r.uly, barW, 30,
			gnw.NoValue, gnw.NoValue, gnw.NoValue, 1000+i*10+3, nil, nil, nil, 0); bc >= 0 {
			buttons = append(buttons, bc)
		}
	}
	m.bars = bars

	// BARTAPE is the yellow/black hazard stripe drawn over a production bar's
	// deposit headroom that cannot be mined (its key is 0, already transparent).
	if t, e := ctx.loadSimple("BARTAPE"); e == nil {
		m.tape = t.Buffer()
	}

	// Bottom buttons: Help (1000) / Done (1099). Captions use Font1 to match the
	// other large button (MNUBTN) screens like Save/Load; the in game Font5 is too
	// large here. The row labels above use an explicit f5, so they're unaffected.
	ctx.Sys.Fonts().SetFont(fonts.Font1)
	// MNUBTN5U's recessed face is ~2px left of the image centre, so nudge the
	// "?" caption left to sit centred in the visible recess (this one button).
	if h, e := ctx.plateButtonOff(win, 465, 438, "MNUBTN5U", "MNUBTN5D", ctx.text("eb74", "?"), 1000, -2); e == nil {
		ctx.registerButtonSfx(h, "NHELP0")
		buttons = append(buttons, h)
	}
	if d, e := ctx.plateButton(win, 514, 438, "MNUBTN6U", "MNUBTN6D", ctx.text("40c7", "Done"), 1099); e == nil {
		ctx.registerButtonSfx(d, "NDONE0")
		buttons = append(buttons, d)
	}
	return buttons
}

// scale maps a value out of maxv onto the bar's pixel width.
func (m *allocMenu) scale(v, maxv int) int {
	if maxv <= 0 {
		return 0
	}
	x := barW * v / maxv
	if x < 0 {
		return 0
	}
	if x > barW {
		return barW
	}
	return x
}

// drawBar paints one bar: restore the recess, fill the material art to
// value/capacity, optionally lay the hazard tape over the remainder, and
// centre the value caption on the bar in white with a black outline (FONT_1).
// C++: CargoBarData::Draw (allocmenu.cpp).
// drawBar fills value/max with the material sprite (anchored to the right so
// the worm's head end cap sits at the fill boundary, like LoadHorizontalBar),
// then lays the hazard tape over the locked tail [lock..max] (aligned to the
// left so its capped edge starts at the lock, like LoadHorizontalTape). value
// can't exceed lock, so the metal head never enters the tape.
func (m *allocMenu) drawBar(buf *gfx.Buffer, i, by, value, maxv, lock int, tapeBar bool, caption string) {
	buf.Sub(barX, by, barW, 30).Copy(m.pic.Buffer().Sub(barX, by, barW, 30))
	fw := m.scale(value, maxv)
	if m.bars[i] != nil && fw > 0 {
		bw := m.bars[i]
		w := min(fw, bw.W)
		h := min(bw.H, 30)
		buf.Sub(barX, by, w, h).TransCopy(bw.Sub(bw.W-w, 0, w, h)) // anchored to the right
	}
	// The hazard tape covers the locked tail [lock..maxv] of a production bar.
	// With no deposit (maxv == 0) the WHOLE bar is locked - the resource can
	// never be mined here - so the tape spans it (C++ CargoBarData::Draw: the
	// cargo_3 == 0 branch sets offset_x = rect_width). Usage/reserve bars pass
	// tapeBar == false and never tape.
	if m.tape != nil && tapeBar {
		lockX, tw := tapeSpan(maxv, lock, barW, m.tape.W)
		h := min(m.tape.H, 30)
		if tw > 0 {
			buf.Sub(barX+lockX, by, tw, h).TransCopy(m.tape.Sub(0, 0, tw, h)) // aligned to the left
		}
	}
	blitInBox(buf, m.f1, caption, barX, by, barW, 30, colorWhite|fonts.FlagOutline, true)
}

// drawBars repaints all three bars per resource: production (with the hazard
// tape over deposit headroom that cannot be mined), usage ("N (+net / turn)") and reserve.
func (m *allocMenu) drawBars() {
	ctx, c := m.ctx, m.c
	prod, capac := c.CargoMining(m.rmap) // current mining + deposits
	net := c.NetTurn(m.difficulty)       // production - consumption per turn
	stored, storeCap := c.CargoInfo()    // reserve + storage capacity
	buf := ctx.Sys.Buf(m.win)
	ctx.Sys.Lock()
	defer ctx.Sys.Unlock()
	for i, r := range m.rows {
		p := prod.Get(r.cargo)
		dep := capac.Get(r.cargo)
		n := net.Get(r.cargo)
		usage := p - n
		// MiningHeadroom respects each station's spare slots and local deposit.
		// A pooled FreeCapacity total overstates the result for complexes with
		// several mining stations. The tape covers [lock..dep].
		lock := dep
		if headroom := c.MiningHeadroom(m.rmap, r.cargo); p+headroom < lock {
			lock = p + headroom
		}
		m.drawBar(buf, i, r.uly, p, dep, lock, true, fmt.Sprintf("%d", p))
		// Usage / Reserve bars carry no lock tape (tapeBar == false).
		m.drawBar(buf, i, r.uly+37, usage, dep, dep, false, fmt.Sprintf("%d (%+d / turn)", usage, n))
		rsv, rcap := stored.Get(r.cargo), storeCap.Get(r.cargo)
		m.drawBar(buf, i, r.uly+74, rsv, rcap, rcap, false, fmt.Sprintf("%d", rsv))
	}
}

// addMining raises one resource by one unit. The station's total capacity is
// fixed (16), so when it is already full the unit is taken from the other
// resource with the most mining; a minimal stand in for the rebalance across
// resources that AllocMenu_Optimize does (so the bar/arrow can increase, not only lower).
func (m *allocMenu) addMining(t economy.CargoType) {
	c, rmap := m.c, m.rmap
	// A resource can never exceed its deposit (its lock), so don't drain the
	// others when it is already maxed.
	prod, capac := c.CargoMining(rmap)
	if prod.Get(t) >= capac.Get(t) {
		return
	}
	if c.AddMining(rmap, t, 1) > 0 {
		return // had spare capacity
	}
	raw, fuel, gold := c.MiningTotals()
	donors := []allocDonor{{economy.CargoRaw, raw}, {economy.CargoFuel, fuel}, {economy.CargoGold, gold}}
	best := -1
	for i, d := range donors {
		if d.cargo != t && d.mined > 0 && (best < 0 || d.mined > donors[best].mined) {
			best = i
		}
	}
	if best >= 0 && c.ReduceMining(donors[best].cargo, 1) > 0 {
		if c.AddMining(rmap, t, 1) == 0 {
			c.AddMining(rmap, donors[best].cargo, 1) // restore: t couldn't take it
		}
	}
}

// handleKey runs one input event of the allocation loop, returning true when the
// screen is finished. Done commits (the arrows edit the complex live), ESC puts
// every station's mining back to the entry snapshot.
func (m *allocMenu) handleKey(key int) bool {
	ctx, c, rows := m.ctx, m.c, m.rows
	switch {
	case key == 1099 || key == platform.KeyReturn:
		return true // commit (changes are already live)
	case key == platform.KeyEscape:
		for u, s := range m.snapshot { // discard
			u.RawMining, u.FuelMining, u.GoldMining, u.TotalMining = s.raw, s.fuel, s.gold, s.total
		}
		return true
	case key == 1000 || key == '?':
		Help(ctx, "ALLOCATE_SETUP")
	case key >= 1001 && key <= 1093:
		i, op := (key-1000)/10, (key-1000)%10
		if i < 0 || i >= len(rows) {
			break
		}
		switch op {
		case 1: // left arrow: -1
			c.ReduceMining(rows[i].cargo, 1)
		case 2: // right arrow: +1
			m.addMining(rows[i].cargo)
		case 3: // bar click: nudge by one toward the clicked half (clicking
			// left of the bar's midpoint lowers, right raises); a steady step
			// instead of jumping to the absolute clicked fraction.
			if ctx.Plat.Mouse().X-m.ox < barX+barW/2 {
				c.ReduceMining(rows[i].cargo, 1)
			} else {
				m.addMining(rows[i].cargo)
			}
		}
		m.drawBars()
	}
	return false
}
