// upgrade.go : calculates and stores permanent unit attribute upgrades.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : unitvalues.cpp - upgrade application when research completes
package game

import (
	"math"

	"github.com/suglasp/max-go/internal/assets"
)

// Upgrade is a permanent, gold-bought stat improvement for one unit type. Research raises team-wide levels; both stack on base attributes.
//
// C++ ported : abstractupgrademenu.cpp / upgradecontrol.cpp - AbstractUpgradeMenu / UpgradeControl

// UpgradeUnit is one selectable unit type for the Upgrade menu: spec base values (the cost
// denominator), a working copy the menu edits, the research discount, the upgradable attributes,
// and the class flags driving category filters. Plain data shared by gamescreen and menus.
//
// C++ ported : abstractupgrademenu.cpp - per unit type state (unitvalues_base / unitvalues_actual)
type UpgradeUnit struct {
	Type           assets.UnitType
	Name           string
	Base           *UnitValues
	Cur            *UnitValues
	ResearchFactor [AttribCount]int
	Attribs        []Attrib
	Ground         bool
	Air            bool
	Sea            bool
	Building       bool
	Combat         bool
}

// UpgradeCostLimit is the per step gold cost above which an attribute can no longer be upgraded.
//
// C++ ported : upgradecontrol.cpp - UPGRADECONTROL_UPGRADE_COST_LIMIT
const UpgradeCostLimit = 800

// upgradeFactors is the per attribute cost multiplier (attack, rounds, range, armor, hits, speed,
// scan, turns, ammo). Speed 4, turns 16 and ammo 2; vanilla DOS upgrade
// prices confirm the speed and ammo values.
//
// C++ ported : upgradecontrol.cpp - UpgradeControl_Factors
var upgradeFactors = [9]int{4, 4, 8, 2, 2, 4, 8, 16, 2}

// upgradeFactor returns the cost multiplier for attribute a (0 if it cannot be upgraded).
func upgradeFactor(a Attrib) int {
	if a >= 0 && int(a) < len(upgradeFactors) {
		return upgradeFactors[a]
	}
	return 0
}

// UpgradeStep is the amount one upgrade click adds to attribute a, scaled by the unit type's spec
// base so high stat units step in bigger increments.
//
// C++ ported : upgradecontrol.cpp - UpgradeControl::Init (upgrade_amount)
func UpgradeStep(a Attrib, baseValue int) int {
	if a == AttribTurns {
		return 1
	}
	switch {
	case baseValue >= 50:
		return 10
	case baseValue >= 25:
		return 5
	case baseValue >= 10:
		return 2
	default:
		return 1
	}
}

// UpgradeResearchFactor is the research offset subtracted from the current value before cost is
// computed, so research makes upgrades cheaper. It scales the spec base by the research level.
//
// C++ ported : researchmenu.cpp - ResearchMenu_CalculateFactor
func UpgradeResearchFactor(level, specBase int) int {
	if level <= 0 {
		return 0
	}
	return specBase * level / 10
}

// levelCost is the cumulative gold cost of having attribute a at value, given the research factor
// and the unit type's spec base.
//
// C++ ported : upgradecontrol.cpp - UpgradeControl_CalculateCost
func levelCost(a Attrib, value, researchFactor, baseValue int) int {
	f := upgradeFactor(a)
	if a == AttribTurns { // build turns reduction: cheaper as value drops
		denom := value - researchFactor
		if denom <= 0 {
			return 0
		}
		return int(math.Pow(float64(baseValue)/float64(denom), 7.5) * float64(f))
	}
	if baseValue <= 0 {
		return 0
	}
	return int(math.Pow(float64(value-researchFactor)/float64(baseValue), 7.5) * float64(f))
}

// UpgradeCost is the gold cost of one upgrade click raising attribute a by UpgradeStep, given the
// research factor and spec base. Refunded on stepping back down. At least 1.
//
// C++ ported : upgradecontrol.cpp - UpgradeControl::CalculateCost
func UpgradeCost(a Attrib, currentValue, researchFactor, baseValue int) int {
	step := UpgradeStep(a, baseValue)
	cost := levelCost(a, currentValue+step, researchFactor, baseValue) -
		levelCost(a, currentValue, researchFactor, baseValue)
	if cost < 1 {
		cost = 1
	}
	return cost
}

// UnitUpgrade is one gold-bought attribute step above a unit type's base. Purchase screens return these before game creation; the in-game Upgrade menu writes the offsets to TeamInfo.
//
// C++ ported : cargomenu.cpp - CargoMenu::CommitUpgradeChanges (the bought values go
// into UnitsManager_TeamInfo[team].team_units, which every deployed and produced unit reads)
type UnitUpgrade struct {
	Type   assets.UnitType
	Attrib Attrib
	Offset int
}

// teamUpgrades returns the per attribute upgrade offsets for unit type t, allocating lazily.
func (t *TeamInfo) teamUpgrades(unit assets.UnitType) *[AttribCount]int {
	if t.Upgrades == nil {
		t.Upgrades = map[assets.UnitType]*[AttribCount]int{}
	}
	off := t.Upgrades[unit]
	if off == nil {
		off = &[AttribCount]int{}
		t.Upgrades[unit] = off
	}
	return off
}

// UpgradeOffset returns the gold bought offset for one attribute of a unit type (0 if never upgraded).
//
// C++ ported : units_manager.cpp - upgrade delta in current_unit_values
func (t *TeamInfo) UpgradeOffset(unit assets.UnitType, a Attrib) int {
	if t.Upgrades == nil || a < 0 || a >= AttribCount {
		return 0
	}
	if off := t.Upgrades[unit]; off != nil {
		return off[a]
	}
	return 0
}

// SetUpgradeOffset records the gold bought offset for one attribute of a unit type.
func (t *TeamInfo) SetUpgradeOffset(unit assets.UnitType, a Attrib, value int) {
	if a < 0 || a >= AttribCount {
		return
	}
	off := t.teamUpgrades(unit)
	off[a] = value
}
