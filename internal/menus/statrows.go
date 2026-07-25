// statrows.go : builds stat-panel rows shared by build and purchase menus.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++: unitstats.cpp - UnitStats_DrawStats
package menus

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
)

// The statRow type these builders return is declared in unitstats.go beside
// unitStatRows, the in game stats popup's builder over the same row shape.

// statCargoIcon picks the cargo gauge icon matching the unit's cargo type.
func statCargoIcon(t assets.UnitType) string {
	switch economy.CargoTypeOf(t) {
	case economy.CargoFuel:
		return "I_FUEL"
	case economy.CargoGold:
		return "I_GOLD"
	}
	return "I_RAW"
}

// buildStatRows lists the stat panel rows the Construct and Production menus
// show for buildable type t: cargo/armor/hits/scan plus the build cost in raw
// material drawn by the given builder. C++: UnitStats_DrawStats.
func buildStatRows(v *game.UnitValues, t, builder assets.UnitType, baseTurns int) []statRow {
	var rows []statRow
	if s := v.Get(game.AttribStorage); s > 0 {
		rows = append(rows, statRow{"Cargo", statCargoIcon(t), s, s / 10})
	}
	if a := v.Get(game.AttribArmor); a > 0 {
		rows = append(rows, statRow{"Armor", "I_ARMOR", a, a})
	}
	if h := v.Get(game.AttribHits); h > 0 {
		rows = append(rows, statRow{"Hits", "I_HITS", h, h})
	}
	if s := v.Get(game.AttribScan); s > 0 {
		rows = append(rows, statRow{"Scan", "I_SCAN", s, s})
	}
	cost := baseTurns * economy.RawConsumptionRate(builder, 1)
	rows = append(rows, statRow{"Cost", "I_GOLD", cost, cost})
	return rows
}

// cargoStatRow is one stat panel line on the purchase screen: the label, the
// backing attribute, the printed value, the gauge count, the gauge icon, and
// whether the row takes a purchase time upgrade stepper.
type cargoStatRow struct {
	label        string
	attr         game.Attrib
	value, count int
	icon         string
	upgradeable  bool
}

// cargoStepRow marks a purchase screen stat row with a live stepper: the
// attribute it edits and the row's y position (for click hit testing).
type cargoStepRow struct {
	attr game.Attrib
	y    int
}

// cargoStatRows builds the purchase screen's stat rows for unit type t in the
// original's order (UnitStats_DrawStats), valued from cv (the upgraded values)
// against base bv. Military rows lead (only when the unit has an attack value)
// and each upgradeable row takes a stepper; the "Uses" row appears only for
// units that consume raw material (builders/factories).
// C++: UnitStats_DrawStats / UnitStats_DrawMilitary / UnitStats_DrawUsage.
func cargoStatRows(at []assets.UnitAttributes, t assets.UnitType, cv, bv *game.UnitValues) []cargoStatRow {
	var rows []cargoStatRow
	if bv.Get(game.AttribAttack) > 0 {
		v := cv.Get(game.AttribAttack)
		rows = append(rows, cargoStatRow{"Attack", game.AttribAttack, v, v, "I_HRDATK", true})
		if bv.Get(game.AttribRounds) > 0 {
			sv := min(cv.Get(game.AttribRounds), cv.Get(game.AttribAmmo))
			rows = append(rows, cargoStatRow{"Shots", game.AttribRounds, sv, sv, "I_SHOTS", true})
		}
		if bv.Get(game.AttribRange) > 0 {
			v := cv.Get(game.AttribRange)
			rows = append(rows, cargoStatRow{"Range", game.AttribRange, v, v, "I_RANGE", true})
		}
		if bv.Get(game.AttribAmmo) > 0 {
			v := cv.Get(game.AttribAmmo)
			rows = append(rows, cargoStatRow{"Ammo", game.AttribAmmo, v, v, "I_AMMO", true})
		}
	}
	if bv.Get(game.AttribStorage) > 0 {
		v := cv.Get(game.AttribStorage)
		rows = append(rows, cargoStatRow{"Cargo", game.AttribStorage, v, v / 10, statCargoIcon(t), false})
	}
	if raw := economy.RawConsumptionRate(t, 1); raw > 0 {
		rows = append(rows, cargoStatRow{"Uses", 0, 0, raw, "I_RAW", false})
	}
	if bv.Get(game.AttribArmor) > 0 {
		v := cv.Get(game.AttribArmor)
		rows = append(rows, cargoStatRow{"Armor", game.AttribArmor, v, v, "I_ARMOR", true})
	}
	if bv.Get(game.AttribHits) > 0 {
		v := cv.Get(game.AttribHits)
		rows = append(rows, cargoStatRow{"Hits", game.AttribHits, v, v, "I_HITS", true})
	}
	if bv.Get(game.AttribScan) > 0 {
		v := cv.Get(game.AttribScan)
		rows = append(rows, cargoStatRow{"Scan", game.AttribScan, v, v, "I_SCAN", true})
	}
	if bv.Get(game.AttribSpeed) > 0 {
		v := cv.Get(game.AttribSpeed)
		rows = append(rows, cargoStatRow{"Speed", game.AttribSpeed, v, v, "I_SPEED", true})
	}
	cost := cargoUnitCost(at, t)
	rows = append(rows, cargoStatRow{"Cost", 0, cost, cost, "I_GOLD", false})
	return rows
}
