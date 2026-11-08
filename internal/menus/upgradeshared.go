// upgradeshared.go : provides stat-display and cost helpers for upgrade menus.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package menus

import "github.com/suglasp/max-go/internal/gfx"

// Shared rendering for the two AbstractUpgradeMenu screens (cargo + upgrade):
// the selection marker, the gauge icon row, and the stat row table. C++:
// GameManager_DrawUnitSelector, UnitStats_DrawIcons/DrawIcon, UnitStats_DrawStats.

// drawSelMarker draws the white corner bracket selection marker (black outer
// edge) around a size x size icon cell at (x, y); the in game unit selector.
// C++: GameManager_DrawUnitSelector (single/mobile marker).
func drawSelMarker(buf *gfx.Buffer, x, y, size int) {
	const L = 9
	set := func(px, py int, c uint8) {
		if px >= 0 && px < buf.W && py >= 0 && py < buf.H {
			buf.Set(px, py, c)
		}
	}
	corner := func(cornerX, cornerY, dirX, dirY int) {
		for i := 0; i < L; i++ {
			set(cornerX+dirX*i, cornerY, 0)
			set(cornerX+dirX*i, cornerY+dirY, 0xFF)
			set(cornerX+dirX*i, cornerY+2*dirY, 0xFF)
			set(cornerX, cornerY+dirY*i, 0)
			set(cornerX+dirX, cornerY+dirY*i, 0xFF)
			set(cornerX+2*dirX, cornerY+dirY*i, 0xFF)
		}
	}
	corner(x, y, 1, 1)
	corner(x+size-1, y, -1, 1)
	corner(x, y+size-1, 1, -1)
	corner(x+size-1, y+size-1, -1, -1)
}

// drawGauge draws count gauge icons from (baseX, baseY) within availW pixels, in
// diagonal groups of 5 (each item steps right by the scaled item step and down
// by vstep; vstep is 0 for <6 icons so they draw in a straight line). Icons are
// clipped to [clipL, clipR) horizontally and clipB vertically.
//
// C++: UnitStats_DrawIcons + UnitStats_DrawIcon (full icon case, unitstats.cpp).
func drawGauge(buf, ic *gfx.Buffer, count, baseX, baseY, availW, clipL, clipR, clipB int) {
	drawGaugePair(buf, ic, nil, count, count, baseX, baseY, availW, gaugeRowH, clipL, clipR, clipB)
}

// gaugeRowH is the box height the unit stats panel lays its gauges out in. The Units
// report is shorter and passes its own, so the diagonal stagger stays inside the row.
const gaugeRowH = 18

// gaugeStagger is how far each icon in a group of five sits below the one before it.
// Only a run past five staggers, and only when the box has height to spare: an icon as
// tall as the box it sits in leaves none, which is what keeps a report row straight.
//
// C++ ported : unitstats.cpp - UnitStats_DrawIcons, (18 - image->height) / 4
func gaugeStagger(boxH, iconH, count int) int {
	if count < 6 {
		return 0
	}

	if step := (boxH - iconH) / 4; step > 0 {
		return step
	}

	return 0
}

// drawGaugePair is drawGauge with a depleted half: it lays out base icons but fills
// only the first cur of them, drawing the rest with the empty sprite. That is what a
// Units report row shows, where "13/21 TOTAL" is 21 icons of which 13 are full. A nil
// empty sprite simply stops after the filled run, which is what drawGauge wants.
//
// boxH is the height the icons are centred and staggered in, which is not always the
// whole row: the report hands over one pixel less because its bottom scanline carries
// the separator hairline.
//
// C++ ported : reportstats.cpp - ReportStats_DrawIcons (it swaps to the empty image
// the moment the filled count runs out, part way through the layout)
func drawGaugePair(buf, full, empty *gfx.Buffer, cur, base, baseX, baseY, availW, boxH, clipL, clipR, clipB int) {
	// The layout is measured off the filled sprite: the two are the same size, and a
	// row with nothing filled still has to reserve the same space.
	ic := full

	if ic == nil {
		ic = empty
	}

	if ic == nil || base <= 0 {
		return
	}

	count := base
	iconW, iconH := ic.W, ic.H
	vstep := gaugeStagger(boxH, iconH, count)
	groups := (count + 4) / 5
	curOffset := count - (groups-1)*5
	aw := availW - iconW*groups
	var2c := groups*4 - (5 - curOffset)
	itemStep, gap := iconW, 0
	if var2c > 0 {
		if aw < var2c {
			itemStep = 1
			if groups > 1 {
				gap = (groups + (var2c - aw) - 2) / (groups - 1)
			}
		} else if itemStep = aw / var2c; itemStep > iconW {
			itemStep = iconW
		}
	}
	groupAdv := itemStep*4 + iconW - gap
	topY := baseY + (((boxH - vstep*4) - iconH) / 2)
	if topY < baseY {
		topY = baseY
	}
	for i := 0; i < count; i++ {
		// The filled run comes first, then the sprite switches to the depleted one.
		sprite := full
		if i >= cur {
			sprite = empty
		}

		if sprite == nil {
			continue
		}

		r := i % 5
		x := baseX + (i/5)*groupAdv + itemStep*r
		y := topY + r*vstep
		w := min(sprite.W, clipR-x)
		h := min(sprite.H, clipB-y)
		if x >= clipL && w > 0 && h > 0 {
			buf.Sub(x, y, w, h).TransCopy(sprite.Sub(0, 0, w, h))
		}
	}
}
