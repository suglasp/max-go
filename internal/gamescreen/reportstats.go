// reportstats.go : builds the stat block shown beside each unit in the Units report:
// hits, ammo or cargo, speed, shots, and for a building its power and work draw
// against the complex it belongs to.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : reportstats.cpp - ReportStats_Draw / DrawCommonUnit / DrawStorageUnit / DrawPointsUnit
package gamescreen

import (
	"strconv"

	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/screendata"
)

// A stat block has four rows. Later helpers replace rows 1 through 3, so a power station shows POWER, TOTAL, and USAGE where a tank shows ammo, speed, and shots.
const reportStatRowCount = 4

// The gauge caps: a stat never draws more than this many icons, so a big number stays
// a readable bank of pips rather than a smear. Fuel and gold use the smaller cap
// because their icons are wider. C++: ReportStats_DrawRow's size block.
const (
	reportIconCap     = 25
	reportIconCapWide = 20
)

// reportStats builds one unit's stat block. Empty rows (Base 0) leave their quarter of the block blank.
func (s *Screen) reportStats(u *game.UnitInfo) []screendata.ReportStatRow {
	if u.Values == nil {
		return nil
	}

	rows := make([]screendata.ReportStatRow, reportStatRowCount)

	// Row 0, always: hits. Its icon carries the damage colour, blue through yellow to
	// red, which is why the host picks the sprite rather than the menu.
	maxHits := u.Values.Get(game.AttribHits)
	rows[0] = s.reportRow(s.lang.Get("891f", "HITS"), hitsGaugeKey, u.Hits, maxHits, 4, true)

	// Row 1: a unit that shoots shows its ammo, and anything else shows what it can
	// carry. An empty cargo hold still draws its icons, so the capacity reads at a glance.
	if u.Values.Get(game.AttribAttack) > 0 {
		rows[1] = s.reportRow(s.lang.Get("3347", "AMMO"), "AMMO", u.Ammo, u.Values.Get(game.AttribAmmo), 1, true)
	} else if storage := u.Values.Get(game.AttribStorage); storage > 0 {
		rows[1] = s.reportRow(s.lang.Get("199c", "CARGO"), cargoGaugeKey(u.Type), u.Storage, storage, cargoFactor(u.Type, storage), true)
	}

	rows[2] = s.reportRow(s.lang.Get("50c9", "SPEED"), "SPEED", u.Speed, u.Values.Get(game.AttribSpeed), 1, true)

	// Shots are capped by the ammo actually loaded: a full clip of rounds is no use
	// with an empty magazine.
	shots := min(u.Values.Get(game.AttribRounds), u.Values.Get(game.AttribAmmo))
	rows[3] = s.reportRow(s.lang.Get("27e1", "SHOTS"), "SHOTS", u.Shots, shots, 1, false)

	// A building draws power or work from its complex, and those rows replace the
	// mobile ones above.
	if u.Values.Get(game.AttribStorage) == 0 {
		s.reportPowerRows(u, rows)
	}

	s.reportWorkRows(u, rows)
	s.reportComplexCargoRow(u, rows)

	return rows
}

// reportRow assembles one line, capping the factor so the gauge fits its column.
// C++: the factor/size rescale at the top of ReportStats_DrawRow.
func (s *Screen) reportRow(label, gaugeKey string, cur, base, factor int, line bool) screendata.ReportStatRow {
	if base <= 0 || factor <= 0 {
		return screendata.ReportStatRow{}
	}

	if cur > base {
		cur = base
	}

	if cur < 0 {
		cur = 0
	}

	iconCap := reportIconCap

	if gaugeKey == "FUEL" || gaugeKey == "GOLD" {
		iconCap = reportIconCapWide
	}

	if (base+factor-1)/factor > iconCap {
		factor = (base + iconCap - 1) / iconCap
	}

	g := s.hudArt.statGauges[gaugeKey]

	if gaugeKey == hitsGaugeKey {
		g = s.hitsGaugeFor(cur, base)
	}

	return screendata.ReportStatRow{
		Label:  label,
		Cur:    cur,
		Base:   base,
		Factor: factor,
		Full:   g.full,
		Empty:  g.empty,
		Line:   line,
	}
}

// hitsGaugeKey is not a real gauge map entry: it tells reportRow to pick the damage
// coloured sprite instead. C++: SI_HITSB -> SI_HITSY -> SI_HITSR.
const hitsGaugeKey = "HITS"

// hitsGaugeFor picks the hits sprite by how battered the unit is.
func (s *Screen) hitsGaugeFor(cur, base int) gaugePair {
	return s.hudArt.hitsGauge[hitsGaugeTier(cur, base)]
}

// hitsGaugeTier selects blue above half health, yellow above a quarter, and red at a quarter or below.
// C++ ported : reportstats.cpp - the SI_HITSB -> SI_HITSY -> SI_HITSR swap
func hitsGaugeTier(cur, base int) int {
	switch {
	case cur*4 <= base:
		return 2 // red
	case cur*2 <= base:
		return 1 // yellow
	default:
		return 0 // blue
	}
}

// cargoGaugeKey maps a unit's cargo type onto its gauge sprite.
func cargoGaugeKey(t assets.UnitType) string {
	switch economy.CargoTypeOf(t) {
	case economy.CargoFuel:
		return "FUEL"
	case economy.CargoGold:
		return "GOLD"
	default:
		return "CARGO"
	}
}

// cargoFactor is how many cargo points one icon stands for. A small hold shows every
// point, a big one groups them, and anything that is not a material (a transport's
// passenger slots) groups hard. C++: the cargo_value ladder in ReportStats_Draw.
func cargoFactor(t assets.UnitType, storage int) int {
	if economy.CargoTypeOf(t) == economy.CargoNone {
		return 10
	}

	switch {
	case storage <= 4:
		return 1
	case storage <= 8:
		return 2
	default:
		return 3
	}
}

// reportPowerRows replaces rows 1 to 3 for a building that makes or burns power. A
// producer shows what it puts out and what the whole complex puts out; a consumer
// shows its own draw, the complex's draw, and the complex's supply.
//
// C++ ported : reportstats.cpp - ReportStats_DrawCommonUnit
func (s *Screen) reportPowerRows(u *game.UnitInfo, rows []screendata.ReportStatRow) {
	rate := economy.PowerConsumptionRate(u.Type)

	if rate == 0 || economy.LifeConsumptionRate(u.Type) != 0 {
		return
	}

	own := economy.NetProduction(u, 0).Power
	mine, complexMine, complexTheirs := 0, 0, 0

	if rate >= 0 {
		rows[1] = s.reportRow(s.lang.Get("deb1", "POWER"), "POWER", -own, rate, 1, false)
	} else {
		rows[1] = s.reportRow(s.lang.Get("76a7", "POWER"), "POWER", own, -rate, 1, false)
	}

	// Sum the whole complex: what it can supply against what it burns.
	supply, demand := 0, 0

	for _, o := range s.complexPeers(u) {
		if r := economy.PowerConsumptionRate(o.Type); r >= 0 {
			demand += r
		} else {
			supply -= r
		}

		if p := economy.NetProduction(o, 0).Power; p >= 0 {
			complexTheirs += p
		} else {
			complexMine -= p
		}
	}

	mine = complexMine

	if rate > 0 {
		rows[2] = s.reportRow(s.lang.Get("bda3", "TOTAL"), "POWER", mine, demand, 1, false)
	} else {
		rows[2] = s.reportRow(s.lang.Get("f047", "USAGE"), "POWER", complexTheirs, supply, 1, false)
		rows[3] = s.reportRow(s.lang.Get("b719", "TOTAL"), "POWER", mine, demand, 1, false)
	}
}

// reportWorkRows is the same idea for the work (life support) a habitat supplies and
// the rest of the complex consumes.
//
// C++ ported : reportstats.cpp - ReportStats_DrawStorageUnit
func (s *Screen) reportWorkRows(u *game.UnitInfo, rows []screendata.ReportStatRow) {
	rate := economy.LifeConsumptionRate(u.Type)

	if rate == 0 {
		return
	}

	own := economy.NetProduction(u, 0).Life

	if rate >= 0 {
		rows[1] = s.reportRow(s.lang.Get("1439", "USAGE"), "WORK", -own, rate, 1, false)
	} else {
		rows[1] = s.reportRow(s.lang.Get("3b4a", "WORK"), "WORK", own, -rate, 1, false)
	}

	supply, demand, mine, theirs := 0, 0, 0, 0

	for _, o := range s.complexPeers(u) {
		if r := economy.LifeConsumptionRate(o.Type); r >= 0 {
			demand += r
		} else {
			supply -= r
		}

		if l := economy.NetProduction(o, 0).Life; l >= 0 {
			theirs += l
		} else {
			mine -= l
		}
	}

	if rate > 0 {
		rows[2] = s.reportRow(s.lang.Get("7697", "TOTAL"), "WORK", mine, demand, 1, false)
	} else {
		rows[2] = s.reportRow(s.lang.Get("efc4", "USAGE"), "WORK", theirs, supply, 1, false)
		rows[3] = s.reportRow(s.lang.Get("d94e", "TOTAL"), "WORK", mine, demand, 1, false)
	}
}

// reportComplexCargoRow shows a storage building's whole complex pool rather than only the material in that building.
//
// C++ ported : reportstats.cpp - the trailing Cargo/GetCargoInfo block of ReportStats_Draw
func (s *Screen) reportComplexCargoRow(u *game.UnitInfo, rows []screendata.ReportStatRow) {
	if u.Values.Get(game.AttribStorage) == 0 || !u.Flags.Has(game.FlagStationary) {
		return
	}

	kind := economy.CargoTypeOf(u.Type)

	if kind != economy.CargoRaw && kind != economy.CargoFuel && kind != economy.CargoGold {
		return
	}

	c := s.game.ComplexOf(u)

	if c == nil {
		return
	}

	materials, capacity := c.CargoInfo()
	var cur, base int

	switch kind {
	case economy.CargoRaw:
		cur, base = materials.Raw, capacity.Raw
	case economy.CargoFuel:
		cur, base = materials.Fuel, capacity.Fuel
	case economy.CargoGold:
		cur, base = materials.Gold, capacity.Gold
	}

	// A pool of hundreds would draw hundreds of icons, so group them.
	factor := 10

	if base/factor > 25 {
		factor = base / 25
	}

	rows[2] = s.reportRow(s.lang.Get("4bde", "TOTAL"), cargoGaugeKey(u.Type), cur, base, factor, false)
}

// complexPeers lists the stationary units sharing this unit's complex, itself
// included. A unit with no complex stands alone.
func (s *Screen) complexPeers(u *game.UnitInfo) []*game.UnitInfo {
	c := s.game.ComplexOf(u)

	if c == nil {
		return []*game.UnitInfo{u}
	}

	var peers []*game.UnitInfo

	for _, o := range s.game.Units.Units() {
		if o.Flags.Has(game.FlagStationary) && s.game.ComplexOf(o) == c {
			peers = append(peers, o)
		}
	}

	return peers
}

// reportDisplayName is the name shown in the report: the mark the unit was built at,
// its type, and which one of that type it is. C++: UnitInfo::GetDisplayName ->
// "Mk <roman version> <singular name> <unit id>".
func reportDisplayName(u *game.UnitInfo, index int) string {
	mark := 1

	if u.Values != nil {
		mark = u.Values.Version()
	}

	return "Mk " + romanNumeral(mark) + " " + displayName(u.Type) + " " + strconv.Itoa(index)
}

// romanNumeral writes a mark level the way the original does.
// C++ ported : unitinfo.cpp - UnitInfo::GetVersion / CalcRomanDigit
func romanNumeral(n int) string {
	if n <= 0 {
		return ""
	}

	digits := []struct {
		value int
		sign  string
	}{
		{1000, "M"}, {900, "CM"}, {500, "D"}, {400, "CD"},
		{100, "C"}, {90, "XC"}, {50, "L"}, {40, "XL"},
		{10, "X"}, {9, "IX"}, {5, "V"}, {4, "IV"}, {1, "I"},
	}

	out := ""

	for _, d := range digits {
		for n >= d.value {
			out += d.sign
			n -= d.value
		}
	}

	return out
}
