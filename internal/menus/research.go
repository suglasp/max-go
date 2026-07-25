// research.go : implements per-topic research-center allocation.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++: researchmenu.cpp - ResearchMenu
package menus

import (
	"fmt"

	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/gnw"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/sprites"
)

// RSRCHPIC allocates team research centres among eight topics with arrows and sliders. It shows bonuses and turns to the next level; Done commits allocation and Cancel discards it.

// researchTopicIcons names the per topic stat icon (ResearchMenu_TopicIcons).
var researchTopicIcons = [game.ResearchTopicCount]string{
	"I_HRDATK", "I_SHOTS", "I_RANGE", "I_ARMOR", "I_HITS", "I_SPEED", "I_SCAN", "I_GOLD",
}

// researchTopicLabel is a topic name: the lang_english.ini key and the English
// fallback used when the key is missing.
type researchTopicLabel struct{ key, fallback string }

// researchTopicLabelKeys are the lang_english.ini keys for the topic names.
var researchTopicLabelKeys = [game.ResearchTopicCount]researchTopicLabel{
	{"4cd5", "Attack"}, {"7382", "Shots"}, {"86bc", "Range"}, {"a592", "Armor"},
	{"07f9", "Hits"}, {"5857", "Speed"}, {"920c", "Scan"}, {"dbae", "Cost"},
}

// researchRow holds a topic row's slider geometry + background snapshots so the
// labs count, slider knob and turns can be erased and redrawn on change.
type researchRow struct {
	uly                       int
	countBg, sliderBg, turnBg *gfx.Buffer
	sliderX, sliderW          int
}

// researchMenu is the research screen state: the team's per topic state, the
// working lab allocation the arrows and slider edit, and the row geometry plus
// background snapshots the row painter needs. research is only written on Done.
// C++: researchmenu.cpp - ResearchMenu.
type researchMenu struct {
	ctx     *Context
	win     gnw.WinID
	ox      int // window left edge, to map slider clicks
	centers int

	research *[game.ResearchTopicCount]game.ResearchTopic
	alloc    [game.ResearchTopicCount]int // working copy, committed on Done
	rows     []researchRow

	pic    *sprites.Big
	knob   *sprites.Simple
	f1, f5 *fonts.Font
}

// RunResearch runs the research allocation screen. research is the team's
// per topic state (edited in place only on Done); centers is the count of the
// team's research buildings; center is the window region to centre in.
//
// C++: ResearchMenu::Run (researchmenu.cpp)
func RunResearch(ctx *Context, research *[game.ResearchTopicCount]game.ResearchTopic, centers int, center gfx.Rect) error {
	pic, err := ctx.loadBig("RSRCHPIC")
	if err != nil {
		return err
	}
	knob, err := ctx.loadSimple("PRFSLIDE")
	if err != nil {
		return err
	}

	ox := max(center.ULX+(center.Width()+1-pic.W)/2, 0)
	oy := max(center.ULY+(center.Height()+1-pic.H)/2, 0)
	win := ctx.Sys.AddWindow(ox, oy, pic.W, pic.H, 0, gnw.WindowMoveOnTop|gnw.WindowModal)
	if win < 0 {
		return fmt.Errorf("menus: research window alloc failed")
	}
	defer ctx.Sys.DeleteWindow(win)

	oldFont := ctx.Sys.Fonts().GetFont()
	defer ctx.Sys.Fonts().SetFont(oldFont)
	// Pin the menu font up front: plate button captions bake with the GNW global
	// current font at creation time, so this must precede any button creation
	// otherwise an in game caller's larger font leaks into the captions.
	ctx.Sys.Fonts().SetFont(fonts.Font5)

	m := &researchMenu{
		ctx:      ctx,
		win:      win,
		ox:       ox,
		centers:  centers,
		research: research,
		pic:      pic,
		knob:     knob,
		f5:       ctx.Sys.Fonts().Get(fonts.Font5),
		f1:       ctx.Sys.Fonts().Get(fonts.Font1),
	}
	// alloc is the working copy of each topic's lab count, committed on Done.
	for i := range m.alloc {
		m.alloc[i] = research[i].Allocation
	}

	m.drawStaticText()

	buttons := m.buildWidgets()
	defer func() {
		for _, b := range buttons {
			ctx.Sys.DeleteButton(b)
		}
	}()

	for i := 0; i < game.ResearchTopicCount; i++ {
		m.drawRow(i)
	}
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

// free reports how many of the team's research centers are still unassigned.
func (m *researchMenu) free() int {
	n := m.centers
	for _, a := range m.alloc {
		n -= a
	}
	return n
}

// drawStaticText paints the panel background and the screen's fixed captions.
func (m *researchMenu) drawStaticText() {
	ctx, pic, f5, f1 := m.ctx, m.pic, m.f5, m.f1
	winBuf := ctx.Sys.Buf(m.win)
	ctx.Sys.Lock()
	winBuf.Sub(0, 0, pic.W, pic.H).Copy(pic.Buffer().Sub(0, 0, pic.W, pic.H))
	blitInBox(winBuf, f5, ctx.text("3609", "Research Topics"), 95, 19, 170, 13, colorGreen|fonts.FlagOutline, true)
	blitInBox(winBuf, f1, ctx.text("d181", "Labs"), 23, 52, 125, 12, optGold, true)
	blitInBox(winBuf, f1, ctx.text("e228", "Topics"), 184, 52, 90, 12, optGold, true)
	blitInBox(winBuf, f1, ctx.text("9892", "Turns"), 282, 52, 60, 12, optGold, true)
	ctx.Sys.Unlock()
}

// buildWidgets paints each topic row's static half (icon, name, level bonus),
// snapshots its dynamic boxes and creates the row arrows, slider region and the
// bottom button row, returning the ids so the caller can delete them on exit.
func (m *researchMenu) buildWidgets() []gnw.ButtonID {
	ctx, win, f5, research := m.ctx, m.win, m.f5, m.research
	rows := make([]researchRow, game.ResearchTopicCount)
	m.rows = rows
	var buttons []gnw.ButtonID

	for i := 0; i < game.ResearchTopicCount; i++ {
		uly := i*28 + 70
		rows[i].uly = uly
		rows[i].sliderX, rows[i].sliderW = 91, 48

		// Static: icon + label + level bonus.
		if icon, e := ctx.loadSimple(researchTopicIcons[i]); e == nil {
			ctx.drawSimpleAt(win, icon, 170, uly+7, true) // hotspot centred
		}
		lbl := ctx.text(researchTopicLabelKeys[i].key, researchTopicLabelKeys[i].fallback)
		buf := ctx.Sys.Buf(win)
		ctx.Sys.Lock()
		blitInBox(buf, f5, lbl, 185, uly, 49, 15, optGold, false)
		blitRight(buf, f5, fmt.Sprintf("+%d%%", research[i].Level*10), 278, uly+3, optGold)
		ctx.Sys.Unlock()

		// Snapshot the dynamic boxes (count / slider track / turns).
		rows[i].countBg = snapBox(ctx, win, 28, uly+3, 32, 10)
		rows[i].sliderBg = snapBox(ctx, win, rows[i].sliderX, uly, rows[i].sliderW, 17)
		rows[i].turnBg = snapBox(ctx, win, 293, uly+3, 40, 10)

		// Arrows: left = 2000+i (decrement), right = 2100+i (increment). These
		// MUST avoid the bottom button values (Done 1000 / Cancel 1001); the
		// old 1000+i scheme made row 0/1 left arrows fire Done/Cancel and close
		// the menu (which exits the game in -menu debug mode).
		if l, e := ctx.imageButton(win, 72, uly, "UPGLFT_U", "UPGLFT_D", "", 0, 0, false, 2000+i); e == nil {
			ctx.registerButtonSfx(l, "MBUTT0")
			buttons = append(buttons, l)
		}
		if r, e := ctx.imageButton(win, 143, uly, "UPGRGT_U", "UPGRGT_D", "", 0, 0, false, 2100+i); e == nil {
			ctx.registerButtonSfx(r, "MBUTT0")
			buttons = append(buttons, r)
		}
		// Slider click region.
		if sb := ctx.Sys.AddButton(win, rows[i].sliderX-4, uly, rows[i].sliderW+8, 17,
			gnw.NoValue, gnw.NoValue, gnw.NoValue, 2200+i, nil, nil, nil, 0); sb >= 0 {
			buttons = append(buttons, sb)
		}
	}

	// Bottom buttons: Cancel (1001) / Help (1049) / Done (1000).
	mkCap := func(x int, label string, value int, sfx string) {
		if b, e := ctx.plateButton(win, x, 294, "DPTBAYUP", "DPTBAYDN", label, value); e == nil {
			ctx.registerButtonSfx(b, sfx)
			buttons = append(buttons, b)
		}
	}
	mkCap(92, ctx.text("74d9", "Cancel"), 1001, "NCANC0")
	if h, e := ctx.imageButton(win, 167, 294, "DPTHP_UP", "DPTHP_DN", "", 0, 0, false, 1049); e == nil {
		ctx.registerButtonSfx(h, "NHELP0")
		buttons = append(buttons, h)
	}
	mkCap(194, ctx.text("0bd5", "Done"), 1000, "NDONE0")
	return buttons
}

// drawRow redraws topic i's labs count, slider knob and turns.
func (m *researchMenu) drawRow(i int) {
	// alloc is read through the receiver: it is an array, so a local copy would
	// silently go stale after set changes it.
	ctx, f5, research := m.ctx, m.f5, m.research
	alloc := &m.alloc
	r := m.rows[i]
	buf := ctx.Sys.Buf(m.win)
	ctx.Sys.Lock()
	defer ctx.Sys.Unlock()
	if r.countBg != nil {
		buf.Sub(28, r.uly+3, r.countBg.W, r.countBg.H).Copy(r.countBg)
	}
	blitRight(buf, f5, fmt.Sprintf("%d", alloc[i]), 59, r.uly+3, optGold)
	if r.sliderBg != nil {
		buf.Sub(r.sliderX, r.uly, r.sliderBg.W, r.sliderBg.H).Copy(r.sliderBg)
	}
	if m.centers > 0 {
		kx := r.sliderX + (r.sliderW-m.knob.Buffer().W)*alloc[i]/m.centers
		m.knob.Draw(buf, kx, r.uly, true)
	}
	if r.turnBg != nil {
		buf.Sub(293, r.uly+3, r.turnBg.W, r.turnBg.H).Copy(r.turnBg)
	}
	if alloc[i] > 0 {
		// turns at this allocation = ceil(turns_to_complete / alloc), using
		// the turns to complete for the current level.
		ttc := game.ResearchTurns(research[i].Level, game.ResearchFactors[i])
		if research[i].Allocation > 0 && research[i].TurnsToComplete > 0 {
			ttc = research[i].TurnsToComplete
		}
		turns := (alloc[i] + ttc - 1) / alloc[i]
		blitRight(buf, f5, fmt.Sprintf("%d", turns), 332, r.uly+3, optGold)
	}
}

// set assigns topic i v labs, clamped to what is still free, and repaints the
// row when the count actually changed.
func (m *researchMenu) set(i, v int) {
	if v < 0 {
		v = 0
	}
	if v > m.alloc[i]+m.free() {
		v = m.alloc[i] + m.free()
	}
	if v != m.alloc[i] {
		m.alloc[i] = v
		m.drawRow(i)
	}
}

// handleKey runs one input event of the research loop, returning true when the
// screen is finished. Done writes the working allocation onto the team's topics,
// Cancel drops it.
func (m *researchMenu) handleKey(key int) bool {
	ctx, research := m.ctx, m.research
	alloc := &m.alloc
	switch {
	case key == 1000 || key == platform.KeyReturn:
		for i := range alloc {
			research[i].SetAllocation(game.ResearchFactors[i], alloc[i])
		}
		return true
	case key == 1001 || key == platform.KeyEscape:
		return true
	case key == 1049 || key == '?':
		Help(ctx, "RESEARCH_SETUP")
	case key >= 2000 && key < 2000+game.ResearchTopicCount:
		m.set(key-2000, alloc[key-2000]-1)
	case key >= 2100 && key < 2100+game.ResearchTopicCount:
		m.set(key-2100, alloc[key-2100]+1)
	case key >= 2200 && key < 2200+game.ResearchTopicCount:
		i := key - 2200
		if m.centers > 0 {
			mx := ctx.Plat.Mouse().X - m.ox
			m.set(i, (mx-m.rows[i].sliderX)*m.centers/m.rows[i].sliderW)
		}
	}
	return false
}

// blitRight draws s ending at rightX (right aligned) at y, under the gnw lock.
func blitRight(buf *gfx.Buffer, f *fonts.Font, s string, rightX, y, color int) {
	x := rightX - f.Width(s)
	if x < 0 {
		x = 0
	}
	w := buf.W - x
	h := buf.H - y
	if w <= 0 || h <= 0 || y < 0 {
		return
	}
	f.Blit(buf.Sub(x, y, w, h), s, w, color)
}
