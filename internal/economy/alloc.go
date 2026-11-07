// alloc.go : distributes raw, fuel, and gold extraction across a complex.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : units_manager.cpp - resource redistribution logic
package economy

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// Allocation helpers for the in game Allocate screen (allocmenu.cpp): they move
// a complex's mining station extraction between raw / fuel / gold within each
// station's 16 unit capacity and the deposit available in its area.

// stationMining returns a pointer to a mining station's extraction field for a
// raw/fuel/gold cargo type, or nil for other cargo types.
func stationMining(u *game.UnitInfo, t CargoType) *int {
	switch t {
	case CargoRaw:
		return &u.RawMining
	case CargoFuel:
		return &u.FuelMining
	case CargoGold:
		return &u.GoldMining
	}
	return nil
}

// stationDeposit returns the deposit of cargo type t available in a station's
// 16 cell area; without a resource map it assumes full capacity.
func stationDeposit(rmap *ResourceMap, u *game.UnitInfo, t CargoType) int {
	if rmap == nil {
		return 16
	}
	raw, gold, fuel := rmap.ResourcesInArea(u.GridX, u.GridY, 1, 16, u.Team, true)
	switch t {
	case CargoRaw:
		return raw
	case CargoFuel:
		return fuel
	case CargoGold:
		return gold
	}
	return 0
}

// MiningTotals sums the complex's current per resource mining over its active mining stations.
func (c *Complex) MiningTotals() (raw, fuel, gold int) {
	for _, u := range c.Members {
		if u.Type != assets.MiningSt || !active(u) {
			continue
		}
		raw += u.RawMining
		fuel += u.FuelMining
		gold += u.GoldMining
	}
	return
}

// AddMining adds up to n units of cargo type t mining across the complex's
// stations, honouring each station's 16 unit capacity and the deposit available
// in its area. Returns the amount actually added.
//
// C++: AllocMenu_Optimize using spare capacity. Cross-resource rebalancing is
// handled separately by AutoBalanceFuel.
func (c *Complex) AddMining(rmap *ResourceMap, t CargoType, n int) int {
	added := 0
	for _, u := range c.Members {
		if n <= 0 {
			break
		}
		if u.Type != assets.MiningSt || !active(u) {
			continue
		}
		cur := stationMining(u, t)
		if cur == nil {
			continue
		}
		room := 16 - (u.RawMining + u.FuelMining + u.GoldMining)
		if d := stationDeposit(rmap, u, t) - *cur; d < room {
			room = d
		}
		if room > n {
			room = n
		}
		if room > 0 {
			*cur += room
			u.TotalMining += room
			added += room
			n -= room
		}
	}
	return added
}

// MiningHeadroom reports how much more of cargo type t the complex's stations
// could mine right now by reallocating their spare capacity: a read only dry
// run of AddMining, honouring the same per station 16 unit cap and the
// deposit in that station's own area. The Allocate screen's hazard tape lock
// needs this, not the pooled MiningTotals free count - a station's spare
// slots are only usable for a resource its own area actually has, and a
// simple pooled total overstates headroom once a complex has more than one
// mining station.
//
// C++: AllocMenu_Optimize (allocmenu.cpp), the dry-run half of AddMining.
func (c *Complex) MiningHeadroom(rmap *ResourceMap, t CargoType) int {
	headroom := 0
	for _, u := range c.Members {
		if u.Type != assets.MiningSt || !active(u) {
			continue
		}
		cur := stationMining(u, t)
		if cur == nil {
			continue
		}
		room := 16 - (u.RawMining + u.FuelMining + u.GoldMining)
		if d := stationDeposit(rmap, u, t) - *cur; d < room {
			room = d
		}
		if room > 0 {
			headroom += room
		}
	}
	return headroom
}

// AutoBalanceFuel makes active mining stations cover the fuel used by active
// power buildings. It uses spare station capacity first, then moves extraction
// from raw to fuel where deposits allow it. The operation shifts only enough
// extraction to meet the fuel requirement.
//
// C++: the Allocate screen exposes the same adjustment manually. The port applies
// this minimum each turn.
func (c *Complex) AutoBalanceFuel(rmap *ResourceMap) {
	burn := 0
	for _, u := range c.Members {
		if active(u) && isGenerator(u.Type) {
			burn += FuelConsumptionRate(u.Type)
		}
	}
	if burn <= 0 {
		return // no fuel burning generators: nothing to cover
	}
	_, fuel, _ := c.MiningTotals()
	need := burn - fuel
	if need <= 0 {
		return // already extracting enough fuel
	}
	// First take fuel from spare (unallocated) station capacity.
	need -= c.AddMining(rmap, CargoFuel, need)
	// Still short: convert raw extraction to fuel one unit at a time, but only when the
	// fuel deposit can actually absorb it - free a unit of raw, try to move it to fuel,
	// and if fuel cannot take it (no deposit/capacity) put the raw back and stop. This
	// never reduces raw output for fuel that cannot be mined.
	for need > 0 {
		if c.ReduceMining(CargoRaw, 1) == 0 {
			break // no raw extraction left to convert
		}
		if c.AddMining(rmap, CargoFuel, 1) == 0 {
			c.AddMining(rmap, CargoRaw, 1) // fuel deposit is tapped out: restore the raw unit
			break
		}
		need--
	}
}

// ReduceMining removes up to n units of cargo type t mining across the
// complex's stations. Returns the amount removed.
//
// C++: AllocMenu_ReduceProduction.
func (c *Complex) ReduceMining(t CargoType, n int) int {
	removed := 0
	for _, u := range c.Members {
		if n <= 0 {
			break
		}
		if u.Type != assets.MiningSt || !active(u) {
			continue
		}
		cur := stationMining(u, t)
		if cur == nil || *cur == 0 {
			continue
		}
		d := *cur
		if d > n {
			d = n
		}
		*cur -= d
		u.TotalMining -= d
		removed += d
		n -= d
	}
	return removed
}
