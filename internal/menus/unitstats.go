// unitstats.go : implements the attribute panel shown when inspecting a map unit.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++: unitstatsmenu.cpp - UnitStatsMenu
package menus

import (
	"fmt"

	"github.com/suglasp/max-go/internal/build"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/gnw"
	"github.com/suglasp/max-go/internal/platform"
	"github.com/suglasp/max-go/internal/sprites"
	"github.com/suglasp/max-go/internal/units"
)

// STATS is the full-screen unit information panel opened by right-click. It shows the portrait, name,
// description, and attribute gauges.
//
// DEVIATIONS (carried over, shared with the cargo/construct screens): the per unit
// description text is a placeholder; the real strings need the unit data to
// lang key map; Help is a stub; the task debugger overlay is omitted.

// unitStatusMenu is the Unit Status screen state: the unit being inspected and
// the art its single paint pass draws with.
//
// C++: UnitStatsMenu (unitstats.cpp)
type unitStatusMenu struct {
	ctx  *Context
	u    *game.UnitInfo
	name string

	pic      *sprites.Big
	portrait *sprites.Big
	sc       *units.SpriteCache
	f5       *fonts.Font
	buf      *gfx.Buffer
}

// RunUnitStatus shows the status screen for unit u (display name name) and
// returns on Done/ESC. C++: UnitStats_Menu -> UnitStatsMenu::Run (unitstats.cpp).
func RunUnitStatus(ctx *Context, u *game.UnitInfo, name string) error {
	pic, err := ctx.loadBig("STATS")
	if err != nil {
		return err
	}
	oldFont := ctx.Sys.Fonts().GetFont()
	defer ctx.Sys.Fonts().SetFont(oldFont)

	m := &unitStatusMenu{
		ctx:  ctx,
		u:    u,
		name: name,
		pic:  pic,
		sc:   units.NewSpriteCache(ctx.Lib, &pic.Palette),
		f5:   ctx.Sys.Fonts().Get(fonts.Font5),
		buf:  ctx.Sys.Buf(mainWindow),
	}

	// Load the portrait up front (a resource load, kept out of the locked draw).
	if tag := unitPortrait[u.Type]; tag != "" {
		if big, e := ctx.loadBig(tag); e == nil {
			m.portrait = big
		}
	}

	m.paint()

	buttons := m.buildWidgets()
	defer func() {
		for _, b := range buttons {
			ctx.Sys.DeleteButton(b)
		}
	}()

	ctx.Sys.MouseShow()
	for {
		key := ctx.Sys.GetInput()
		switch {
		case key == 1000 || key == platform.KeyReturn || key == platform.KeyEscape:
			return nil
		case key == 1001 || key == '?':
			Help(ctx, "STATS_SETUP")
		}
	}
}

// paint draws the whole screen: the background, the portrait, the name and
// description, and the attribute gauges.
//
// C++: UnitStatsMenu::Init + UnitStats_DrawStats (unitstats.cpp)
func (m *unitStatusMenu) paint() {
	ctx, u, pic, f5, buf, sc := m.ctx, m.u, m.pic, m.f5, m.buf, m.sc
	portrait, name := m.portrait, m.name

	const (
		prevX, prevY, prevW, prevH = 11, 13, 300, 240  // portrait (11,13)..(311,253)
		statX, statY, statW, statH = 11, 293, 245, 174 // stats panel (image_width 245)
		rowH                       = 19                // C++: each stat row advances screen_width*19
	)

	// statIcon caches a keyed gauge icon (I_HITS/I_ARMOR/...), with its fuchsia
	// top left keyed to transparent.
	statIcons := map[string]*gfx.Buffer{}
	statIcon := func(tag string) *gfx.Buffer {
		if b, ok := statIcons[tag]; ok {
			return b
		}
		var b *gfx.Buffer
		if tag != "" {
			if img, e := ctx.loadSimple(tag); e == nil {
				c := img.Buffer().Clone()
				c.KeyToZero(c.At(0, 0))
				b = c
			}
		}
		statIcons[tag] = b
		return b
	}

	rows := unitStatRows(u)
	// Pack the rows into the panel: shrink the pitch when there are more than fit,
	// so no row is blitted past the panel (and the buffer). That blit going out of bounds
	// panicked while the gnw lock was held, which deadlocked the renderer.
	pitch := rowH
	if n := len(rows); n > 0 && n*pitch > statH {
		pitch = statH / n
	}

	// Draw the whole screen under one lock, released even on a panic (defer in a
	// closure), so the compositor never shows a half drawn frame (the brief fuchsia
	// flash) and a draw panic can never strand the lock.
	func() {
		ctx.Sys.Lock()
		defer ctx.Sys.Unlock()
		// Blit the STATS background here (drawBigAt inlined: calling it would lock again
		// the gnw mutex we already hold). Doing the bg, palette and overlays under one
		// lock also keeps the compositor from ever showing a half drawn frame or a
		// palette/buffer mismatch (the brief fuchsia flash).
		if w, h := min(pic.W, buf.W-pic.ULX), min(pic.H, buf.H-pic.ULY); w > 0 && h > 0 {
			buf.Sub(pic.ULX, pic.ULY, w, h).Copy(pic.Buffer().Sub(0, 0, w, h))
		}
		ctx.Plat.SetPalette(&pic.Palette)

		// Title.
		blitInBox(buf, f5, ctx.text("94b7", "Unit Status Screen"), 327, 7, 158, 18, colorGreen|fonts.FlagOutline, true)

		// Portrait, top left. The unit's P_* big image, else its team coloured icon.
		if portrait != nil {
			drawPortraitFit(buf, portrait, prevX, prevY, prevW, prevH)
		} else {
			sc.DrawIcon(buf, u.Type, u.Team, prevX+prevW/2, prevY+prevH/2)
		}

		// Name (centred) + description, top right.
		blitInBox(buf, f5, name, 336, 60, 287, 11, cargoTan, true)
		// C++ ported : unitstats.cpp - Text_TextBox(base_unit.GetDescription(), 345, 90, 270, 117)
		for i, ln := range wrapText(f5, ctx.unitDescriptionText(u.Type), 270) {
			blitInBox(buf, f5, ln, 345, 90+i*(f5.Height+2), 270, f5.Height+2, cargoTan, false)
		}

		// Stat gauges, bottom left.
		for i, r := range rows {
			ry := statY + i*pitch
			if r.value > 0 {
				s := fmt.Sprintf("%d", r.value)
				blitInBox(buf, f5, s, statX+24-f5.Width(s), ry, f5.Width(s), 17, cargoGold, false)
			}
			blitInBox(buf, f5, r.label, statX+28, ry, 48, 17, cargoTan, false)
			drawGauge(buf, statIcon(r.icon), r.count, statX+76, ry, statW-76, statX, statX+statW, statY+statH)
			if uy := ry + pitch - 2; uy < statY+statH {
				buf.Sub(statX+2, uy, statW-4, 1).Fill(colorRed)
			}
		}
	}()
}

// buildWidgets adds the help and Done buttons and returns their ids.
func (m *unitStatusMenu) buildWidgets() []gnw.ButtonID {
	ctx := m.ctx

	var buttons []gnw.ButtonID

	ctx.Sys.Fonts().SetFont(fonts.Font5)
	if b, e := ctx.imageButton(mainWindow, 457, 452, "BLDHLP_U", "BLDHLP_D", "", 0, 0, false, 1001); e == nil {
		ctx.registerButtonSfx(b, "NHELP0")
		buttons = append(buttons, b)
	}
	if b, e := ctx.plateButton(mainWindow, 484, 452, "BLDONE_U", "BLDONE_D", ctx.text("cf39", "Done"), 1000); e == nil {
		ctx.registerButtonSfx(b, "NDONE0")
		buttons = append(buttons, b)
	}

	return buttons
}

// statRow is one attribute gauge line: a label, its gauge icon, the value shown
// (0 = no number drawn), and the icon count for the gauge.
type statRow struct {
	label, icon  string
	value, count int
}

// unitStatRows builds the gauge rows for a unit from its current values, in the
// original's order (combat first, then cargo / usage / armor, hits, scan / speed /
// cost), each row present only when it applies. C++: UnitStats_DrawStats.
func unitStatRows(u *game.UnitInfo) []statRow {
	val := func(a game.Attrib) int {
		if u.Values != nil {
			return u.Values.Get(a)
		}
		return 0
	}
	var rows []statRow
	if v := val(game.AttribAttack); v > 0 {
		rows = append(rows, statRow{"Attack", "I_HRDATK", v, v})
		if r := val(game.AttribRounds); r > 0 {
			rows = append(rows, statRow{"Shots", "I_SHOTS", r, r})
		}
		if r := val(game.AttribRange); r > 0 {
			rows = append(rows, statRow{"Range", "I_RANGE", r, r})
		}
		if r := val(game.AttribAmmo); r > 0 {
			rows = append(rows, statRow{"Ammo", "I_AMMO", r, r})
		}
	}
	if v := val(game.AttribStorage); v > 0 {
		rows = append(rows, statRow{"Cargo", statCargoIcon(u.Type), v, v / 10})
	}
	if raw := economy.RawConsumptionRate(u.Type, 1); raw > 0 {
		rows = append(rows, statRow{"Uses", "I_RAW", 0, raw}) // consumption, no number
	}
	if v := val(game.AttribArmor); v > 0 {
		rows = append(rows, statRow{"Armor", "I_ARMOR", v, v})
	}
	if v := val(game.AttribHits); v > 0 {
		rows = append(rows, statRow{"Hits", "I_HITS", v, v})
	}
	if v := val(game.AttribScan); v > 0 {
		rows = append(rows, statRow{"Scan", "I_SCAN", v, v})
	}
	if v := val(game.AttribSpeed); v > 0 {
		rows = append(rows, statRow{"Speed", "I_SPEED", v, v})
	}
	if cost := build.Cost(u); cost > 0 {
		rows = append(rows, statRow{"Cost", "I_RAW", cost, cost})
	}
	return rows
}

// drawPortraitFit blits a big portrait into the (x,y,w,h) box, clipped to fit.
func drawPortraitFit(buf *gfx.Buffer, big *sprites.Big, x, y, w, h int) {
	bw, bh := minInt(big.W, w), minInt(big.H, h)
	buf.Sub(x, y, bw, bh).Copy(big.Buffer().Sub(0, 0, bw, bh))
}
