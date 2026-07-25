// statpower.go : implements the building info panel power and worker gauges.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// Note: Power / worker icon gauge rows for the selected building info panel. A power
// generator reads its own output plus the whole complex's available (Total) and
// consumed (Usage) power; a consumer reads its draw plus the complex Total. The
// same pattern drives the worker (life) rows for habitats and research/training.
package gamescreen

import (
	"fmt"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/gfx"
	"github.com/suglasp/max-go/internal/sim"
)

// cargoIconKey selects the stat gauge icon for a material storage unit's cargo
// type: raw boxes, fuel drums or gold bars. C++: ReportStats_CargoIcons indexed
// by the unit's cargo type (reportstats.cpp).
func (s *Screen) cargoIconKey(t assets.UnitType) string {
	switch economy.CargoTypeOf(t) {
	case economy.CargoFuel:
		return "FUEL"
	case economy.CargoGold:
		return "GOLD"
	default:
		return "CARGO" // raw (and the safe fallback)
	}
}

// complexCargo returns the pooled stock and capacity of a building's resource
// across its whole connected complex (the "Total" row beneath its own store).
// Zero capacity means the building is not in a complex (e.g. a bench unit), so
// the caller then draws only the own store row. C++: UnitInfo::GetComplex()->
// GetCargoInfo, switched on the cargo type (game_manager.cpp).
func (s *Screen) complexCargo(u *game.UnitInfo) (current, capacity int) {
	if s.game == nil {
		return 0, 0
	}

	c := s.game.ComplexOf(u)

	if c == nil {
		return 0, 0
	}

	materials, caps := c.CargoInfo()

	switch economy.CargoTypeOf(u.Type) {
	case economy.CargoFuel:
		return materials.Fuel, caps.Fuel
	case economy.CargoGold:
		return materials.Gold, caps.Gold
	case economy.CargoRaw:
		return materials.Raw, caps.Raw
	}

	return 0, 0
}

// statDifficultyFactor is the mining scale factor passed to NetProduction. The
// power and worker (life) balances it returns are flat (not difficulty scaled),
// so any value works for these rows; 4 is the human/easy factor.
const statDifficultyFactor = 4

// complexResourceSums accumulates a complex's power or worker balance the way
// the original walks the stationary unit list once: current vs capacity, split
// into producers and consumers. C++: the loop in GameManager_DrawInfoDisplay-
// Type2 / Type1 (game_manager.cpp).
type complexResourceSums struct {
	curProd int // current production output (members making the resource now)
	capProd int // production capacity (sum of producer rates)
	curUse  int // current consumption draw (members using it now)
	capUse  int // consumption capacity (sum of consumer rates)
}

// resourceLabels holds the per branch label string IDs the original uses for a
// resource's rows, so the gauge headings match vanilla exactly.
type resourceLabels struct {
	icon            string
	ownProduce      langID // row 2 when this building produces
	ownConsume      langID // row 2 when this building consumes
	totalProduce    langID // row 3 producer: complex Total (available)
	totalConsume    langID // row 3 consumer: complex Total (consumed)
	usageProduceRow langID // row 4 producer: complex Usage (consumed)
}

// langID pairs a lang.ini key with an English fallback.
type langID struct {
	key, fallback string
}

// powerLabels / workerLabels are the vanilla string IDs for the two resources.
var (
	powerLabels = resourceLabels{
		icon:            "POWER",
		ownProduce:      langID{"848e", "Power"},
		ownConsume:      langID{"8d59", "Usage"},
		totalProduce:    langID{"d4ca", "Total"},
		totalConsume:    langID{"c9b3", "Total"},
		usageProduceRow: langID{"81c5", "Usage"},
	}

	workerLabels = resourceLabels{
		icon:            "WORK",
		ownProduce:      langID{"1e1a", "Workers"},
		ownConsume:      langID{"3aea", "Usage"},
		totalProduce:    langID{"7be8", "Total"},
		totalConsume:    langID{"c6fe", "Total"},
		usageProduceRow: langID{"160c", "Usage"},
	}
)

// livePower is u's actual power contribution this turn for the stat readout: positive =
// output, negative = draw. It matches the real power balance (the flat PowerConsumptionRate
// of every ACTIVE member; a power STARVED consumer cut on a deficit draws nothing). Unlike
// NetProduction(u).Power it counts a powered on FACTORY's draw - NetProduction returns 0
// for a factory whose Order is PowerOn (not OrderBuild), which made a running factory read
// "Usage 0/1" while it was in fact drawing power.
func livePower(u *game.UnitInfo) int {
	if !economy.IsActive(u) {
		return 0
	}

	rate := economy.PowerConsumptionRate(u.Type)

	if rate > 0 && u.PowerStarved {
		return 0 // a starved consumer is cut this turn, drawing nothing
	}

	return -rate
}

// sumComplexResource walks the selected building's complex once and tallies the
// power (life=false) or worker (life=true) production and consumption, current
// and capacity. Returns zero sums when the building is not in a complex (e.g. an
// emulate bench), so only the own contribution row then draws.
func (s *Screen) sumComplexResource(u *game.UnitInfo, life bool) complexResourceSums {
	var cs complexResourceSums

	if s.game == nil {
		return cs
	}

	c := s.game.ComplexOf(u)

	if c == nil {
		return cs
	}

	for _, m := range c.Members {
		rate := economy.PowerConsumptionRate(m.Type)
		net := livePower(m)

		if life {
			rate = economy.LifeConsumptionRate(m.Type)
			net = economy.NetProduction(m, statDifficultyFactor).Life
		}

		// A positive rate consumes the resource, a negative rate produces it.
		if rate >= 0 {
			cs.capUse += rate
		} else {
			cs.capProd += -rate
		}

		// A positive net is current production, a negative net current draw.
		if net >= 0 {
			cs.curProd += net
		} else {
			cs.curUse += -net
		}
	}

	return cs
}

// drawResourceRows draws the own contribution row plus the complex wide Total /
// Usage gauge rows for power (life=false) or workers (life=true), exactly as the
// original's Type2 / Type1 readouts. C++: GameManager_DrawInfoDisplayType2 /
// GameManager_DrawInfoDisplayType1 (game_manager.cpp).
func (s *Screen) drawResourceRows(buf *gfx.Buffer, u *game.UnitInfo, life bool) {
	lbl := powerLabels
	rate := economy.PowerConsumptionRate(u.Type)
	ownNet := livePower(u)

	if life {
		lbl = workerLabels
		rate = economy.LifeConsumptionRate(u.Type)
		ownNet = economy.NetProduction(u, statDifficultyFactor).Life
	}

	// Row 2: this building's own contribution this turn over its capacity.
	if rate >= 0 {
		// Consumer: how much it is drawing now (positive) over its rated draw.
		s.drawStatRow(buf, StatRow2, s.label(lbl.ownConsume), lbl.icon, -ownNet, rate, 1)
	} else {
		// Producer: how much it is making now over its rated output.
		s.drawStatRow(buf, StatRow2, s.label(lbl.ownProduce), lbl.icon, ownNet, -rate, 1)
	}

	cs := s.sumComplexResource(u, life)

	if rate > 0 {
		// Consumer: row 3 is the complex's total consumption.
		s.drawStatRow(buf, StatRow3, s.label(lbl.totalConsume), lbl.icon, cs.curUse, cs.capUse, 1)
	} else {
		// Producer: row 3 is the complex's available power/workers (Total),
		// row 4 is how much of it the complex is using (Usage).
		s.drawStatRow(buf, StatRow3, s.label(lbl.totalProduce), lbl.icon, cs.curProd, cs.capProd, 1)
		s.drawStatRow(buf, StatRow4, s.label(lbl.usageProduceRow), lbl.icon, cs.curUse, cs.capUse, 1)
	}
}

// label resolves a langID through the string table with its English fallback.
func (s *Screen) label(id langID) string {
	return s.lang.Get(id.key, id.fallback)
}

// drawStatWindow fills the selected unit stat panel with hits/ammo/speed/shots,
// each a coloured value over a proportional bar; restores the FRAMEPIC backing
// when nothing is selected. C++: WINDOW_STAT_WINDOW's GameManager_DrawInfoDisplay(Row).
func (s *Screen) drawStatWindow(buf *gfx.Buffer) {
	u := s.selected

	if u == nil || u.Values == nil {
		s.restoreFrame(buf, StatWindow)
		return
	}

	w, h := viewSize(StatWindow)
	buf.Sub(StatWindow.ULX, StatWindow.ULY, w, h).Fill(0) // COLOR_BLACK backing

	for _, ry := range []int{StatRow1.LRY, StatRow2.LRY, StatRow3.LRY} {
		buf.DrawLine(StatWindow.ULX, ry, StatWindow.LRX, ry, 0x3C)
	}

	s.drawStatRow(buf, StatRow1, s.lang.Get("c02c", "Hits"), "HITS", u.Hits, u.Values.Get(game.AttribHits), 4)

	if u.Flags.Has(game.FlagStationary) {
		// A building has no movement/ammo: row 1 is hits; rows 2..4 carry its store
		// and the complex wide POWER/WORKER gauges. C++: GameManager_DrawInfoDisplayType2/Type1.
		storage := u.Values.Get(game.AttribStorage)

		if storage > 0 {
			// Row 2: this building's own stored material. Row 3: the complex TOTAL of
			// that resource. C++: own storage row + GetCargoInfo complex total row.
			icon := s.cargoIconKey(u.Type)
			s.drawStatRow(buf, StatRow2, s.cargoLabel(u.Type), icon, u.Storage, storage, 10)

			if cur, cap := s.complexCargo(u); cap > 0 {
				s.drawStatRow(buf, StatRow3, s.lang.Get("d28b", "Total"), icon, cur, cap, 10)
			}
		}

		if storage == 0 && economy.PowerConsumptionRate(u.Type) != 0 && economy.LifeConsumptionRate(u.Type) == 0 {
			s.drawResourceRows(buf, u, false) // power rows: pure power building only
		}

		// Worker rows apply to habitats, research centers, and training halls. Eco Spheres use the Type3 victory-score display.
		// C++: Type1, GREENHSE -> Type3 dispatch.
		if economy.LifeConsumptionRate(u.Type) != 0 && u.Type != assets.Greenhse {
			s.drawResourceRows(buf, u, true)
		}

		return
	}

	// A mobile unit: armed shows ammo+shots, a cargo carrier its load, recon neither,
	// plus speed. Role is read from the type's REAL ATTRIBS, not live values, so a
	// demo unit's placeholder stats do not mislabel a builder or surveyor as armed.
	base := s.baseValues(u.Type)

	switch {
	case sim.IsTransport(u.Type):
		// A transport stores UNITS, not material; slots scale with capacity
		// (<=4 -> 1, <=8 -> 2, else 3 units per icon).
		capacity := sim.TransportCapacity(u)
		slots := 3

		switch {
		case capacity <= 4:
			slots = 1
		case capacity <= 8:
			slots = 2
		}

		s.drawStatRow(buf, StatRow2, s.lang.Get("9f7b", "Units"), "UNITS", len(u.Carried), capacity, slots)
	case base.Get(game.AttribAttack) > 0:
		ammo := u.Values.Get(game.AttribAmmo)
		s.drawStatRow(buf, StatRow2, s.lang.Get("5027", "Ammo"), "AMMO", u.Ammo, ammo, 1)
		s.drawStatRow(buf, StatRow4, s.lang.Get("2bce", "Shots"), "SHOTS", u.Shots, min(u.Values.Get(game.AttribRounds), ammo), 1)
	case base.Get(game.AttribStorage) > 0:
		s.drawStatRow(buf, StatRow2, s.cargoLabel(u.Type), s.cargoIconKey(u.Type), u.Storage, u.Values.Get(game.AttribStorage), 10)
	}

	s.drawStatRow(buf, StatRow3, s.lang.Get("c372", "Speed"), "SPEED", u.Speed, u.Values.Get(game.AttribSpeed), 1)
}

// cargoLabel names a unit's stored resource (Raw / Fuel / Gold) for its stat
// row, so a fuel or gold carrier reads correctly; falls back to "Cargo".
func (s *Screen) cargoLabel(t assets.UnitType) string {
	switch economy.CargoTypeOf(t) {
	case economy.CargoFuel:
		return s.lang.Get("fuel", "Fuel")
	case economy.CargoGold:
		return s.lang.Get("gold", "Gold")
	case economy.CargoRaw:
		return s.lang.Get("raw", "Raw")
	}

	return s.lang.Get("e093", "Cargo")
}

// drawStatRow draws one stat readout: "cur/max" (coloured by ratio), its label,
// then an icon gauge. The icon count is the value scaled down by factor and
// capped at 25 (20 for fuel/gold), so a large stat reads as a bounded bank of
// pips. iconKey empty falls back to a solid bar. C++ ported : game_manager.cpp - GameManager_DrawInfoDisplayRow.
func (s *Screen) drawStatRow(buf *gfx.Buffer, r gfx.Rect, label, iconKey string, cur, max, factor int) {
	if max <= 0 || factor <= 0 {
		return
	}

	if cur > max {
		cur = max
	}

	if cur < 0 {
		cur = 0
	}

	color := int(colorStat(cur, max))
	h := r.Height() + 1
	f := s.sys.Fonts().Get(fonts.Font2)
	s.blitStatText(buf, r.ULX, r.ULY, 45, h, f, fmt.Sprintf("%d/%d", cur, max), color, true)
	s.blitStatText(buf, r.ULX+45, r.ULY, 30, h, f, label, colorTurnText, false)

	barX := r.ULX + 75
	barW := r.LRX - barX + 1

	if barW <= 0 || h <= 4 {
		return
	}

	g := s.hudArt.statGauges[iconKey]

	if iconKey == "HITS" {
		// HITS icon variant tracks the hit ratio (C++: SI_HITSB->Y->R).
		switch {
		case cur*4 <= max:
			g = s.hudArt.hitsGauge[2] // red
		case cur*2 <= max:
			g = s.hudArt.hitsGauge[1] // yellow
		default:
			g = s.hudArt.hitsGauge[0] // blue/green
		}
	}

	size := 25 // icon cap so a large stat reads as a bounded row. C++: DrawInfoDisplayRow's factor/size block

	if iconKey == "FUEL" || iconKey == "GOLD" {
		size = 20
	}

	if (max+factor-1)/factor > size {
		factor = (size + max - 1) / size
	}

	gaugeCur := (cur + factor - 1) / factor
	gaugeMax := (max + factor - 1) / factor

	if g.full == nil || g.full.W <= 0 {
		// No icon resource: fall back to the proportional solid bar.
		if fill := statBarFill(gaugeCur, gaugeMax, barW); fill > 0 {
			buf.Sub(barX, r.ULY+2, fill, h-4).Fill(uint8(color))
		}

		return
	}

	s.drawIconGauge(buf, g, barX, r.ULY, barW, h, gaugeCur, gaugeMax)
}

// gaugePair is a stat gauge's full (filled) and empty (depleted) icon sprites.
type gaugePair struct {
	full  *gfx.Buffer // filled gauge sprite
	empty *gfx.Buffer // depleted gauge sprite
}

// drawIconGauge draws ALL `max` icons (first `cur` "full", rest "empty"), grouped
// 5 to a column, columns compressed (factor/offset), the 5 in a column fanned
// downward by heightOffset, so density tracks the real stat value instead of
// every high stat looking identical. C++ ported : game_manager.cpp - GameManager_DrawInfoDisplayRowIcons.
func (s *Screen) drawIconGauge(buf *gfx.Buffer, g gaugePair, barX, rowY, barW, h, cur, max int) {
	iconW, iconH := g.full.W, g.full.H

	if iconW < 1 || max <= 0 {
		return
	}

	// Vertical fan within a 5-icon column, only when there are more than 5 icons.
	heightOffset := 0

	if max > 5 {
		if heightOffset = ((h - 1) - iconH) / 4; heightOffset < 0 {
			heightOffset = 0
		}
	}

	// Vertical centring of the whole (fanned) gauge within the row.
	topY := (((h - 1) - heightOffset*4) - iconH) / 2

	if topY < 0 {
		topY = 0
	}

	columns := (max + 4) / 5       // base_value_scaled: 5 icons per column
	lastCol := max - (columns-1)*5 // base_value_diff: icons in the final column (1..5)
	width := barW - iconW*columns  // width left after reserving one icon per column
	widthOffset := columns*4 - (5 - lastCol)
	factor, offset := iconW, 0

	switch {
	case widthOffset == 0:
		factor = iconW
	case width >= widthOffset:
		if factor = width / widthOffset; factor > iconW {
			factor = iconW
		}
	default: // not enough room: pack columns to one pixel and compress their stride
		factor = 1

		if columns > 1 {
			offset = (columns + (widthOffset - width) - 2) / (columns - 1)
		}
	}

	colStride := (iconW + factor*4) - offset // ulx: horizontal step between columns
	icon := g.full

	if cur <= 0 {
		icon = g.empty
	}

	for i := 0; i < max; i++ {
		iconIndex := i % 5
		ix := barX + colStride*(i/5) + factor*iconIndex
		iy := rowY + topY + iconIndex*heightOffset

		if icon != nil && ix >= 0 && iy >= 0 {
			w := min(icon.W, barX+barW-ix, buf.W-ix)
			ih := min(icon.H, buf.H-iy)

			if w > 0 && ih > 0 {
				buf.Sub(ix, iy, w, ih).TransCopy(icon.Sub(0, 0, w, ih))
			}
		}

		if i+1 == cur { // the remaining icons are the depleted/empty variant
			icon = g.empty
		}
	}

}

// blitStatText draws FONT2 text inside an (x,y,w,h) box: centred when center is
// set, else left aligned, vertically centred either way.
func (s *Screen) blitStatText(buf *gfx.Buffer, x, y, w, h int, f *fonts.Font, text string, color int, center bool) {
	if text == "" {
		return
	}

	tw := f.Width(text)
	tx := x

	if center && tw < w {
		tx = x + (w-tw)/2
	}

	ty := y + (h-f.Height)/2

	if ty < y {
		ty = y
	}

	f.Blit(buf.Sub(tx, ty, buf.W-tx, buf.H-ty), text, tw+1, color)
}

// colorStat is the stat colour ramp: red at or below a quarter, yellow at or
// below half, green above. C++: GameManager_DrawInfoDisplayRow's colour branch.
func colorStat(cur, max int) uint8 {
	switch {
	case cur <= max/4:
		return 0x01 // COLOR_RED
	case cur <= max/2:
		return 0x04 // COLOR_YELLOW
	default:
		return 0x02 // COLOR_GREEN
	}
}

// statBarFill is the filled pixel width of a cur/max bar that spans barW.
func statBarFill(cur, max, barW int) int {
	if max <= 0 || barW <= 0 {
		return 0
	}

	if cur > max {
		cur = max
	}

	if cur < 0 {
		cur = 0
	}

	return cur * barW / max
}
