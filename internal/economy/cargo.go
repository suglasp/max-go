// cargo.go : defines resource bundles and unit production or consumption rates.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : cargo.cpp, complex.cpp, survey.cpp, units_manager.cpp - resource and cargo handling
package economy

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// CargoType is the single resource a storage bearing unit holds.
//
// C++: Unit::CargoType (unit.hpp): the RAW/FUEL/GOLD subset (LAND/SEA/AIR
// transport classes are not resource storage and are out of scope here).
type CargoType uint8

const (
	CargoNone CargoType = iota
	CargoRaw
	CargoFuel
	CargoGold
)

// String names the cargo type ("raw"/"fuel"/"gold"/"none") for debug logs and UI.
func (c CargoType) String() string {
	switch c {
	case CargoRaw:
		return "raw"
	case CargoFuel:
		return "fuel"
	case CargoGold:
		return "gold"
	default:
		return "none"
	}
}

// Cargo is a bundle of the five economy resources plus a free capacity
// accumulator, mirroring the original's Cargo struct. All fields are signed:
// a net production Cargo carries negative values for consumption.
//
// C++: struct Cargo (cargo.hpp).
type Cargo struct {
	Gold         int
	Raw          int
	Fuel         int
	Power        int
	Life         int
	FreeCapacity int
}

// Add accumulates other into c (operator+=).
func (c *Cargo) Add(o Cargo) {
	c.Gold += o.Gold
	c.Raw += o.Raw
	c.Fuel += o.Fuel
	c.Power += o.Power
	c.Life += o.Life
	c.FreeCapacity += o.FreeCapacity
}

// Sub subtracts other from c (operator-=).
func (c *Cargo) Sub(o Cargo) {
	c.Gold -= o.Gold
	c.Raw -= o.Raw
	c.Fuel -= o.Fuel
	c.Power -= o.Power
	c.Life -= o.Life
	c.FreeCapacity -= o.FreeCapacity
}

// Get returns the amount of a mined resource type (raw/fuel/gold only).
//
// C++: Cargo::Get (cargo.hpp).
func (c Cargo) Get(t CargoType) int {
	switch t {
	case CargoRaw:
		return c.Raw
	case CargoFuel:
		return c.Fuel
	case CargoGold:
		return c.Gold
	default:
		return 0
	}
}

// CargoTypeOf reports which resource a unit type stores, or CargoNone.
//
// C++: Unit::GetCargoType is data driven in the original unit definitions. This
// table records the storage-bearing types used by the port.
func CargoTypeOf(t assets.UnitType) CargoType {
	switch t {
	case assets.ADump, assets.MiningSt, assets.Engineer, assets.Constrct,
		assets.Bulldozr, assets.Repair, assets.SplyTrck, assets.MineLayr,
		assets.SeaMnLyr, assets.Master:
		// A mine layer's mine store is raw material (placing a mine spends one unit
		// of storage, reloaded one for one with raw), so it transfers raw like a truck.
		// Factories (Shipyard/LandPlt/LightPlt/AirPlt/TrainHal) are NOT here: their
		// build material is drawn straight from the complex pool (sim/buildorders.go
		// BuildMaterial/buildPool), never through a CargoType bin, and X-Fer does not
		// offer them one in the original (user 2026-07-26 live check).
		return CargoRaw
	case assets.FDump, assets.FuelTrck:
		// PowerStn/PowGen consume fuel but do not X-Fer it: in vanilla only the dedicated Fuel Truck moves fuel to or from a Fuel Tank,
		// the same way only a Gold Truck moves gold to or from a Gold Vault.
		// A power building's fuel draw is the complex pool, not a CargoType bin, like a factory's raw draw above.
		return CargoFuel
	case assets.GoldSm, assets.GoldTrck:
		return CargoGold
	default:
		return CargoNone
	}
}

// StartConstructorCargo / StartEngineerCargo are the raw materials the opening
// base's first Constructor (storage cap 60) and first Engineer (cap 40) land
// with: a fixed partial load, never full and never scaled by the map's resource
// richness. The purchase screen shows them already loaded (free of charge, the
// team's credit is untouched) and the random benches, which have no purchase
// screen, deploy the same amounts.
//
// C++: the mission loadout seeded into UnitsManager_TeamMissionSupplies before
// CargoMenu opens (cargomenu.cpp - GetMissionLoadout fills units + cargos and
// leaves team_gold alone).
const (
	StartConstructorCargo = 40
	StartEngineerCargo    = 20
)

// StartBuilderCargo returns the raw cargo a founder builder of type t lands with.
// Any other unit type starts empty.
func StartBuilderCargo(t assets.UnitType) int {
	switch t {
	case assets.Constrct:
		return StartConstructorCargo
	case assets.Engineer:
		return StartEngineerCargo
	default:
		return 0
	}
}

// RawConsumptionRate is the raw material draw of a builder/factory at the
// given build speed multiplier (1/2/4 turns the multiplier into 1/4/12).
//
// C++: Cargo_GetRawConsumptionRate (cargo.cpp).
func RawConsumptionRate(t assets.UnitType, speedMultiplier int) int {
	var mult int
	switch speedMultiplier {
	case 2:
		mult = 4
	case 4:
		mult = 12
	default:
		mult = speedMultiplier
	}
	switch t {
	case assets.TrainHal:
		return mult
	case assets.Engineer, assets.Constrct:
		return mult * 2
	case assets.Shipyard, assets.LandPlt, assets.AirPlt, assets.LightPlt:
		return mult * 3
	default:
		return 0
	}
}

// FuelConsumptionRate is the fuel a power building burns per turn.
//
// C++: Cargo_GetFuelConsumptionRate (cargo.cpp).
func FuelConsumptionRate(t assets.UnitType) int {
	switch t {
	case assets.PowerStn:
		return 6
	case assets.PowGen:
		return 2
	default:
		return 0
	}
}

// PowerConsumptionRate is a unit's power draw (negative for power producers).
//
// C++: Cargo_GetPowerConsumptionRate (cargo.cpp).
func PowerConsumptionRate(t assets.UnitType) int {
	switch t {
	case assets.Shipyard, assets.LandPlt, assets.AirPlt, assets.LightPlt,
		assets.TrainHal, assets.Greenhse, assets.Research, assets.CommTwr,
		assets.MiningSt:
		return 1
	case assets.PowerStn:
		return -6
	case assets.PowGen:
		return -1
	default:
		return 0
	}
}

// LifeConsumptionRate is a unit's worker/life draw (negative for habitats,
// which house workers).
//
// C++: Cargo_GetLifeConsumptionRate (cargo.cpp).
func LifeConsumptionRate(t assets.UnitType) int {
	switch t {
	case assets.Habitat:
		return -3
	case assets.TrainHal, assets.Greenhse, assets.Research:
		return 1
	default:
		return 0
	}
}

// GoldConsumptionRate is a unit's gold upkeep per turn.
//
// C++: Cargo_GetGoldConsumptionRate (cargo.cpp).
func GoldConsumptionRate(t assets.UnitType) int {
	switch t {
	case assets.CommTwr:
		return 5
	default:
		return 0
	}
}

// applyConsumption subtracts a unit's running costs (at the given build speed
// multiplier) from cargo.
//
// C++: Cargo_ApplyUnitConsumption (cargo.cpp).
func applyConsumption(t assets.UnitType, buildMultiplier int, cargo *Cargo) {
	cargo.Raw -= RawConsumptionRate(t, buildMultiplier)
	cargo.Fuel -= FuelConsumptionRate(t)
	cargo.Power -= PowerConsumptionRate(t)
	cargo.Life -= LifeConsumptionRate(t)
	cargo.Gold -= GoldConsumptionRate(t)
}

// NetProduction returns a unit's per turn contribution to its complex: a
// mining station's extracted resources (scaled by a difficulty factor, 4 for
// human/easy AI; pass 5/6 for master/god AI) minus its upkeep, a power/upkeep
// building's consumption when powered on, and a factory's build draw. Factory
// construction contributes nothing when no build rate is available.
//
// C++: Cargo_GetNetProduction (cargo.cpp).
func NetProduction(u *game.UnitInfo, difficultyFactor int) Cargo {
	var cargo Cargo

	orders := u.Order
	if u.State == game.OrderStateInit {
		orders = u.PriorOrder
	}

	switch u.Type {
	case assets.Shipyard, assets.LightPlt, assets.LandPlt, assets.TrainHal, assets.AirPlt:
		// A factory draws raw + power while building a unit, at its build rate; an
		// idle (or finished) factory draws nothing.
		if orders == game.OrderBuild && u.State != game.OrderStateUnitReady {
			rate := u.BuildRate
			if rate < 1 {
				rate = 1
			}
			applyConsumption(u.Type, rate, &cargo)
		}

	case assets.CommTwr, assets.PowerStn, assets.PowGen, assets.Habitat, assets.Research, assets.Greenhse:
		if orders == game.OrderPowerOn {
			applyConsumption(u.Type, 1, &cargo)
		}

	case assets.MiningSt:
		if orders == game.OrderPowerOn || orders == game.OrderNewAllocate {
			if difficultyFactor <= 0 {
				difficultyFactor = 4
			}
			cargo.Gold += (u.GoldMining * difficultyFactor) / 4
			cargo.Raw += (u.RawMining * difficultyFactor) / 4
			cargo.Fuel += (u.FuelMining * difficultyFactor) / 4
			cargo.FreeCapacity += 16 - u.TotalMining
			applyConsumption(u.Type, 1, &cargo)
		}
	}

	return cargo
}

// Inventory returns the stored cargo of a unit, bucketed by its cargo type.
//
// C++: Cargo_GetInventory (cargo.cpp).
func Inventory(u *game.UnitInfo) Cargo {
	var c Cargo
	switch CargoTypeOf(u.Type) {
	case CargoRaw:
		c.Raw = u.Storage
	case CargoFuel:
		c.Fuel = u.Storage
	case CargoGold:
		c.Gold = u.Storage
	}
	return c
}

// Capacity returns the cargo capacity of a unit (ATTRIB_STORAGE), bucketed by
// its cargo type.
//
// C++: Cargo_GetCargoCapacity (cargo.cpp).
func Capacity(u *game.UnitInfo) Cargo {
	var c Cargo
	if u.Values == nil {
		return c
	}
	storage := u.Values.Get(game.AttribStorage)
	switch CargoTypeOf(u.Type) {
	case CargoRaw:
		c.Raw = storage
	case CargoFuel:
		c.Fuel = storage
	case CargoGold:
		c.Gold = storage
	}
	return c
}
