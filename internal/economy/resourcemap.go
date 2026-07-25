// resourcemap.go : stores deposits and each team's survey state.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ ported : survey.cpp - resource deposit map and mining
package economy

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/world"
)

// Resource map cell bit layout, matching the original packed CargoMap word:
// the low 5 bits hold the deposit amount, one type bit flags the resource.
//
// C++: the CARGO_* enum (enums.hpp); per team "surveyed" bits are kept in a
// separate array here rather than packed into the high bits.
const (
	cargoMask = 0x1F // amount 0..31 (read capped at 16)
	cargoFuel = 0x20
	cargoGold = 0x40
	cargoRaw  = 0x80
	cargoCap  = 16 // a single cell never yields more than 16 of a resource
)

// SurfaceSource yields the surface mask of a cell; the resource map uses it to
// skip pure air cells. Satisfied by *world.Map and the test surfaces; may be
// nil (then no cell is treated as air).
type SurfaceSource interface {
	SurfaceAt(x, y int) uint8
}

// ResourceMap is the per cell raw/fuel/gold deposit map plus a per team
// "surveyed" overlay (which teams have revealed each cell's deposit). It is
// the Go home of ResourceManager_CargoMap and the survey.cpp queries.
type ResourceMap struct {
	W, H     int
	deposits []uint8
	surveyed []uint8 // bit per team (0..3)
	Surf     SurfaceSource
}

// NewResourceMap allocates an empty (no deposits, unsurveyed) map.
func NewResourceMap(w, h int) *ResourceMap {
	return &ResourceMap{W: w, H: h, deposits: make([]uint8, w*h), surveyed: make([]uint8, w*h)}
}

// Export returns copies of the raw deposit and per team surveyed cell arrays
// (row major, length W*H) for save games. The Surf source is not persisted - it
// is reconstructed from the map on import.
func (m *ResourceMap) Export() (deposits, surveyed []uint8) {
	return append([]uint8(nil), m.deposits...), append([]uint8(nil), m.surveyed...)
}

// ImportResourceMap rebuilds a ResourceMap from saved cell arrays (the output of
// Export). A short or nil slice leaves the remaining cells empty; set Surf after.
func ImportResourceMap(w, h int, deposits, surveyed []uint8) *ResourceMap {
	m := NewResourceMap(w, h)
	copy(m.deposits, deposits)
	copy(m.surveyed, surveyed)
	return m
}

// inBounds reports whether x/y addresses a resource map cell.
func (m *ResourceMap) inBounds(x, y int) bool {
	return x >= 0 && y >= 0 && x < m.W && y < m.H
}

// idx converts x/y coordinates to a flat resource map index.
func (m *ResourceMap) idx(x, y int) int {
	return y*m.W + x
}

// SetDeposit places a resource deposit of the given type and amount at a cell
// (used by tests and the start of game allocator). amount is clamped to 31.
func (m *ResourceMap) SetDeposit(x, y int, t CargoType, amount int) {
	if !m.inBounds(x, y) {
		return
	}
	if amount > cargoMask {
		amount = cargoMask
	}
	if amount < 0 {
		amount = 0
	}
	var flag uint8
	switch t {
	case CargoFuel:
		flag = cargoFuel
	case CargoGold:
		flag = cargoGold
	default:
		flag = cargoRaw
	}
	m.deposits[m.idx(x, y)] = flag | uint8(amount)
}

// DepositAt decodes the deposit at a cell into its type and amount (capped at
// 16, as the game reads it).
func (m *ResourceMap) DepositAt(x, y int) (CargoType, int) {
	if !m.inBounds(x, y) {
		return CargoNone, 0
	}
	v := m.deposits[m.idx(x, y)]
	amount := int(v & cargoMask)
	if amount > cargoCap {
		amount = cargoCap
	}
	// A cell carries its resource type even at amount 0 (a region's tapered
	// edge), so the survey can mark it with that type's empty circle. Only a cell
	// with no type flag at all is truly empty.
	if v&(cargoGold|cargoFuel|cargoRaw) == 0 {
		return CargoNone, 0
	}
	switch {
	case v&cargoGold != 0:
		return CargoGold, amount
	case v&cargoFuel != 0:
		return CargoFuel, amount
	default:
		return CargoRaw, amount
	}
}

// MarkSurveyed records that a team has surveyed a cell.
func (m *ResourceMap) MarkSurveyed(x, y, team int) {
	if m.inBounds(x, y) && team >= 0 && team < 8 {
		m.surveyed[m.idx(x, y)] |= 1 << uint(team)
	}
}

// IsSurveyed reports whether a team has surveyed a cell.
func (m *ResourceMap) IsSurveyed(x, y, team int) bool {
	if !m.inBounds(x, y) || team < 0 || team >= 8 {
		return false
	}
	return m.surveyed[m.idx(x, y)]&(1<<uint(team)) != 0
}

// isAir reports whether a cell is pure air (no deposit can be mined there).
func (m *ResourceMap) isAir(x, y int) bool {
	return m.Surf != nil && m.Surf.SurfaceAt(x, y) == world.SurfaceAir
}

// Survey marks every non air cell within radius of (gx, gy) as surveyed by
// team: the reveal a surveyor/mine layer performs.
//
// C++: Survey_SurveyArea (survey.cpp), reduced to the resource reveal effect.
func (m *ResourceMap) Survey(gx, gy, radius, team int) {
	for x := gx - radius; x <= gx+radius; x++ {
		for y := gy - radius; y <= gy+radius; y++ {
			if m.inBounds(x, y) && !m.isAir(x, y) {
				m.MarkSurveyed(x, y, team)
			}
		}
	}
}

// ResourcesInArea sums the raw/gold/fuel deposits in the radius x radius block
// whose top left corner is (gx, gy); the block a mining station or query
// covers. If all is false only cells surveyed by team count; each resource is
// capped at limit. It mirrors Survey_GetResourcesInArea (note the original
// scans [gx, gx+radius] inclusive, i.e. a (radius+1) square anchored at the
// corner).
//
// C++: Survey_GetResourcesInArea (survey.cpp).
func (m *ResourceMap) ResourcesInArea(gx, gy, radius, limit int, team int, all bool) (raw, gold, fuel int) {
	for x := gx; x <= gx+radius; x++ {
		for y := gy; y <= gy+radius; y++ {
			if !m.inBounds(x, y) || m.isAir(x, y) {
				continue
			}
			if !all && !m.IsSurveyed(x, y, team) {
				continue
			}
			t, amount := m.DepositAt(x, y)
			switch t {
			case CargoGold:
				gold += amount
			case CargoFuel:
				fuel += amount
			case CargoRaw:
				raw += amount
			}
		}
	}
	if raw > limit {
		raw = limit
	}
	if gold > limit {
		gold = limit
	}
	if fuel > limit {
		fuel = limit
	}
	return
}

// TotalResourcesInArea is ResourcesInArea with the 255 cap the game uses for
// a station's total deposit potential.
//
// C++: Survey_GetTotalResourcesInArea (survey.cpp).
func (m *ResourceMap) TotalResourcesInArea(gx, gy, radius, team int, all bool) (raw, gold, fuel int) {
	return m.ResourcesInArea(gx, gy, radius, 255, team, all)
}

// SetInitialMining computes a mining station's per resource extraction from
// the deposits in its footprint. It prioritises fuel up to a power generator's
// burn (2), then raw, then more fuel, then gold, with the station's total
// throughput capped at 16; the *Max fields record the full deposit available.
//
// C++: UnitsManager_SetInitialMining (units_manager.cpp).
func (m *ResourceMap) SetInitialMining(u *game.UnitInfo) {
	raw, gold, fuel := m.TotalResourcesInArea(u.GridX, u.GridY, 1, u.Team, true)

	u.RawMiningMax = raw
	u.FuelMiningMax = fuel
	u.GoldMiningMax = gold

	u.FuelMining = min(FuelConsumptionRate(assets.PowGen), fuel)
	free := 16 - u.FuelMining

	u.RawMining = min(raw, free)
	free -= u.RawMining

	extraFuel := min(fuel-u.FuelMining, free)
	u.FuelMining += extraFuel
	free -= extraFuel

	u.GoldMining = min(gold, free)

	u.TotalMining = u.RawMining + u.FuelMining + u.GoldMining
}
