// buildmenudraw.go : holds the drawing shared by the Construction and Production menus.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++: buildmenu.cpp - AbstractBuildMenu draw helpers
package menus

import (
	"fmt"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/sprites"
	"github.com/suglasp/max-go/internal/units"
)

// The CONBUILD and FACBUILD menus draw the same portrait well, stat block and
// X1/X2/X4 turns/cost numbers; they differ only in their panel geometry, which
// roster entry is selected, and the builder that funds the cost. These helpers
// carry that shared drawing so each menu's own method is a thin wrapper.

// unitDescriptionShown is the Description checkbox state, shared by the build,
// factory, cargo and upgrade menus for the whole run, and on at start.
// C++ ported : abstractmenu.hpp - button_description_rest_state (one static, true)
var unitDescriptionShown = true

// unitDescription returns the description text of a unit type for the portrait
// overlay, or "" when the checkbox is off or the unit has none.
// C++ ported : buildmenu.cpp / abstractupgrademenu.cpp - Text_TextBox(base_unit.GetDescription())
func (ctx *Context) unitDescription(t assets.UnitType) string {
	if !unitDescriptionShown {
		return ""
	}

	return ctx.unitDescriptionText(t)
}

// unitDescriptionText returns the description text of a unit type from the
// language strings (key from LANG.RES UNITDESC), or "" when it has none.
// C++ ported : unit.cpp - Unit::GetDescription
func (ctx *Context) unitDescriptionText(t assets.UnitType) string {
	if ctx == nil || ctx.Assets == nil {
		return ""
	}

	key := ctx.Assets.DescriptionKey(t)
	if key == "" {
		return ""
	}

	return ctx.text(key, "")
}

// drawBuildPortrait paints the selected buildable's portrait well: the big
// portrait if the unit has one, else its team icon plus name, and the wrapped
// description text over it when desc is not empty.
func drawBuildPortrait(buf *gfx.Buffer, f5 *fonts.Font, sc *units.SpriteCache, t assets.UnitType, team int, name string, prevX, prevY, prevW, prevH int, desc string, loadPortrait func(assets.UnitType) *sprites.Big) {
	if big := loadPortrait(t); big != nil {
		bw, bh := minInt(big.W, prevW), minInt(big.H, prevH)
		buf.Sub(prevX, prevY, bw, bh).Copy(big.Buffer().Sub(0, 0, bw, bh))
	} else {
		sc.DrawIcon(buf, t, team, prevX+prevW/2, prevY+prevH/2)
		blitInBox(buf, f5, name, prevX, prevY+prevH-24, prevW, 20, colorGreen|fonts.FlagOutline, true)
	}
	if desc != "" {
		for i, ln := range wrapText(f5, desc, prevW-20) {
			blitInBox(buf, f5, ln, prevX+10, prevY+8+i*(f5.Height+2), prevW-20, f5.Height+2, cargoTan|fonts.FlagOutline, false)
		}
	}
}

// drawBuildStatRows draws the unit stat block (value, label, gauge run, hairline
// separator) into the stat panel at (statX, statY, statW, statH).
func drawBuildStatRows(buf *gfx.Buffer, f5 *fonts.Font, rows []statRow, statX, statY, statW, statH int, statIcon func(string) *gfx.Buffer) {
	for i, r := range rows {
		ry := statY + 2 + i*19
		if r.value > 0 {
			s := fmt.Sprintf("%d", r.value)
			blitInBox(buf, f5, s, statX+24-f5.Width(s), ry, f5.Width(s), 17, cargoGold, false)
		}
		blitInBox(buf, f5, r.label, statX+28, ry, 48, 17, cargoTan, false)
		drawGauge(buf, statIcon(r.icon), r.count, statX+76, ry, statW-76, statX, statX+statW, statY+statH)
		buf.Sub(statX+2, ry+17, statW-4, 1).Fill(colorRed)
	}
}

// drawBuildRates draws the turns + cost numbers beside the three Build X1/X2/X4
// buttons (the selected rate green, the rest tan; a rate above maxRate is left
// blank, matching vanilla's Draw). base is the selected unit's base turn count and
// rowY gives the three rows' y positions.
func drawBuildRates(buf *gfx.Buffer, f5 *fonts.Font, buildType, builder assets.UnitType, storage, maxRate, rate, base int, rowY [3]int, turnsRightX, costRightX int) {
	for i, sp := range []int{1, 2, 4} {
		if sp > maxRate {
			continue
		}
		ry := rowY[i]
		tn, cost := turnsAndCost(builder, buildType, sp, storage, base)
		col := cargoTan
		if sp == rate {
			col = colorGreen
		}
		ts := fmt.Sprintf("%d", tn)
		blitInBox(buf, f5, ts, turnsRightX-f5.Width(ts), ry, f5.Width(ts), 11, col, false)
		cs := fmt.Sprintf("%d", cost)
		blitInBox(buf, f5, cs, costRightX-f5.Width(cs), ry, f5.Width(cs), 11, col, false)
	}
}
