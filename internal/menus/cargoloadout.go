// cargoloadout.go : holds the starting loadout model and the buyable roster table.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++: cargomenu.cpp - UnitsManager_TeamMissionSupplies + CargoMenu unit_types1
package menus

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
)

// CargoLoadout is a team's starting roster and the cargo carried by each unit.
// C++ ported : units_manager.cpp - UnitsManager_TeamMissionSupplies
type CargoLoadout struct {
	Team      int
	StartGold int
	TeamGold  int
	Units     []assets.UnitType
	Cargos    []int
	Initial   int // count of protected mission supply units at the front of Units (C++: unit_count); 0 in a standard game

	// Upgrades records purchased steps above base values, and UpgradeGold records their cost. The caller applies them before landing.
	Upgrades    []game.UnitUpgrade
	UpgradeGold int
}

// cargoMinimumTypes are mandatory Constructor, Engineer, and Surveyor units. A fresh loadout seeds them free and protects the last of each from deletion.
// C++: the protected mission supply units (CargoMenu unit_count minimums).
var cargoMinimumTypes = []assets.UnitType{assets.Constrct, assets.Engineer, assets.Surveyor}

// isCargoMinimumType reports whether t is one of the mandatory loadout types.
func isCargoMinimumType(t assets.UnitType) bool {
	for _, m := range cargoMinimumTypes {
		if m == t {
			return true
		}
	}
	return false
}

// NewCargoLoadout creates a free Constructor, Engineer, and Surveyor loadout. Builders receive their default free raw-material cargo, while later purchases spend gold.
func NewCargoLoadout(team, startGold int) *CargoLoadout {
	lo := &CargoLoadout{
		Team:      team,
		StartGold: startGold,
		TeamGold:  startGold,
		Units:     append([]assets.UnitType(nil), cargoMinimumTypes...),
		Initial:   len(cargoMinimumTypes),
	}

	lo.Cargos = make([]int, len(lo.Units))
	for i, t := range lo.Units {
		lo.Cargos[i] = economy.StartBuilderCargo(t)
	}

	return lo
}

// CanDelete reports whether the unit at index may be removed from the loadout: any
// bought duplicate is removable, but the last Constructor, Engineer, or
// Surveyor of its type is protected.
func (lo *CargoLoadout) CanDelete(index int) bool {
	if index < 0 || index >= len(lo.Units) {
		return false
	}
	t := lo.Units[index]
	if !isCargoMinimumType(t) {
		return true // a not minimum type is always removable
	}
	count := 0
	for _, u := range lo.Units {
		if u == t {
			count++
		}
	}
	return count > 1 // a minimum type is removable only if a duplicate exists
}

// cargoRosterEntry is one buyable type and its display name (the roster the
// original seeds into unit_types1, in the same order).
type cargoRosterEntry struct {
	t    assets.UnitType
	name string
}

// cargoRoster is the buyable unit list (C++: CargoMenu's unit_types1).
var cargoRoster = []cargoRosterEntry{
	{assets.Constrct, "Constructor"},
	{assets.Engineer, "Engineer"},
	{assets.Surveyor, "Surveyor"},
	{assets.Scout, "Scout"},
	{assets.Scanner, "Scanner"},
	{assets.Tank, "Tank"},
	{assets.Artillry, "Assault Gun"},
	{assets.RocktLch, "Rocket Launcher"},
	{assets.MisslLch, "Missile Crawler"},
	{assets.MineLayr, "Mine Layer"},
	{assets.Repair, "Repair Unit"},
	{assets.SplyTrck, "Supply Truck"},
	{assets.FuelTrck, "Fuel Truck"},
	{assets.SpFlak, "Mobile A-A Gun"},
	{assets.GoldTrck, "Gold Truck"},
}

// cargoUnitCost is a roster unit's gold cost: the land factory's raw draw at
// build speed 1 times the unit's build turns.
//
// C++: Cargo_GetRawConsumptionRate(LANDPLT, 1) * ATTRIB_TURNS (cargomenu.cpp).
func cargoUnitCost(at []assets.UnitAttributes, t assets.UnitType) int {
	rate := economy.RawConsumptionRate(assets.LandPlt, 1)
	if int(t) >= 0 && int(t) < len(at) {
		return rate * at[t].TurnsToBuild
	}
	return 0
}

// rosterName returns the display name for a roster unit type, or its tag.
func rosterName(t assets.UnitType) string {
	for _, e := range cargoRoster {
		if e.t == t {
			return e.name
		}
	}
	return t.Tag()
}

// cargoCanBuy reports whether the team can still afford one more unit of type t.
func cargoCanBuy(at []assets.UnitAttributes, gold int, t assets.UnitType) bool {
	return gold >= cargoUnitCost(at, t)
}
