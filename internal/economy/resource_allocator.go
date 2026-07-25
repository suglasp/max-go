// resource_allocator.go : scatters raw, fuel, and gold deposits across a new map.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package economy

import "github.com/suglasp/max-go/internal/game"

// resource_allocator.go scatters the raw/fuel/gold deposits across a new map at
// game start: a normal background level everywhere plus concentrated lodes,
// boosted near each team's landing spot, with below minimum zones cleared.
//
// C++ ported : resource_allocator.cpp - ResourceAllocator
// C++ ported : game_manager.cpp - GameManager_PopulateMapWithResources

// Cargo map cell byte: the material flag in the high bits, amount (0..31) in the
// low five. Each cell holds at most one material (the three materials populate
// interleaved cells via their m point offset).
//
// C++ ported : enums.hpp - CARGO_FUEL/GOLD/MATERIALS
const (
	caFuel   = 0x20
	caGold   = 0x40
	caRaw    = 0x80 // CARGO_MATERIALS
	caFlag   = 0xE0
	caAmount = 0x1F
)

// resourceRange is a [min,max] band; value averages two rolls for a central bias.
//
// C++ ported : resource_allocator.cpp - ResourceRange::GetValue
type resourceRange struct{ min, max int }

func (r resourceRange) value(rng *game.Randomizer) int {
	span := r.max - r.min + 1
	return ((rng.Generate(span) + r.min) + (rng.Generate(span) + r.min) + 1) / 2
}

// ResourceParams holds the allocator tunables (the settings.ini resource keys).
// DefaultResourceParams returns the original's shipped values.
type ResourceParams struct {
	MaxResources, MinResources, MixedSeparation          int
	RawNormalLow, RawNormalHigh                          int
	RawConcLow, RawConcHigh, RawConcSep, RawConcDiff     int
	FuelNormalLow, FuelNormalHigh                        int
	FuelConcLow, FuelConcHigh, FuelConcSep, FuelConcDiff int
	GoldNormalLow, GoldNormalHigh                        int
	GoldConcLow, GoldConcHigh, GoldConcSep, GoldConcDiff int
}

// DefaultResourceParams returns the original's shipped resource settings.
//
// C++ ported : settings.cpp - the *_normal/*_concentrate/min/max_resources defaults
func DefaultResourceParams() ResourceParams {
	return ResourceParams{
		MaxResources: 20, MinResources: 9, MixedSeparation: 40,
		RawNormalLow: 0, RawNormalHigh: 5, RawConcLow: 13, RawConcHigh: 16, RawConcSep: 23, RawConcDiff: 6,
		FuelNormalLow: 2, FuelNormalHigh: 3, FuelConcLow: 12, FuelConcHigh: 16, FuelConcSep: 24, FuelConcDiff: 5,
		GoldNormalLow: 0, GoldNormalHigh: 0, GoldConcLow: 10, GoldConcHigh: 16, GoldConcSep: 32, GoldConcDiff: 5,
	}
}

// clampLevel keeps a setup resource level in the valid 0..2 band (Poor / Average
// / Rich). The setup screen only ever emits those, so an out of range value just
// falls back to Average.
func clampLevel(level int) int {
	if level < 0 || level > 2 {
		return 1
	}
	return level
}

// ResourceParamsForLevels turns the three custom game resource levels (0 Poor, 1
// Average, 2 Rich) into the allocator tunables, the way the original recomputes
// them from the setup choices when a game starts. Raw sets the base min/max band;
// fuel and gold add to it; the mixed separation keys off raw+fuel. The alien
// derelict density is handled separately (alienspawn.go), so it is left out here.
//
// C++ ported : menu.cpp - menu_update_resource_levels
func ResourceParamsForLevels(raw, fuel, gold int) ResourceParams {
	raw, fuel, gold = clampLevel(raw), clampLevel(fuel), clampLevel(gold)

	var p ResourceParams
	resMin, resMax := 1, 15

	switch raw {
	case 0:
		p.RawNormalLow, p.RawNormalHigh = 0, 3
		p.RawConcLow, p.RawConcHigh = 8, 12
		p.RawConcSep, p.RawConcDiff = 28, 6
		resMin, resMax = 4, 16
	case 1:
		p.RawNormalLow, p.RawNormalHigh = 0, 5
		p.RawConcLow, p.RawConcHigh = 13, 16
		p.RawConcSep, p.RawConcDiff = 23, 5
		resMin, resMax = 6, 17
	case 2:
		p.RawNormalLow, p.RawNormalHigh = 1, 5
		p.RawConcLow, p.RawConcHigh = 16, 16
		p.RawConcSep, p.RawConcDiff = 19, 4
		resMin, resMax = 7, 18
	}

	switch fuel {
	case 0:
		p.FuelNormalLow, p.FuelNormalHigh = 1, 2
		p.FuelConcLow, p.FuelConcHigh = 8, 12
		p.FuelConcSep, p.FuelConcDiff = 29, 6
		resMin, resMax = resMin+2, resMax+1
	case 1:
		p.FuelNormalLow, p.FuelNormalHigh = 2, 3
		p.FuelConcLow, p.FuelConcHigh = 12, 16
		p.FuelConcSep, p.FuelConcDiff = 24, 5
		resMin, resMax = resMin+3, resMax+2
	case 2:
		p.FuelNormalLow, p.FuelNormalHigh = 2, 4
		p.FuelConcLow, p.FuelConcHigh = 16, 16
		p.FuelConcSep, p.FuelConcDiff = 20, 4
		resMin, resMax = resMin+4, resMax+3
	}

	switch raw + fuel {
	case 0:
		p.MixedSeparation = 49
	case 1:
		p.MixedSeparation = 44
	case 2:
		p.MixedSeparation = 40
	case 3:
		p.MixedSeparation = 36
	case 4:
		p.MixedSeparation = 33
	}

	switch gold {
	case 0:
		p.GoldNormalLow, p.GoldNormalHigh = 0, 0
		p.GoldConcLow, p.GoldConcHigh = 5, 9
		p.GoldConcSep, p.GoldConcDiff = 39, 6
	case 1:
		p.GoldNormalLow, p.GoldNormalHigh = 0, 0
		p.GoldConcLow, p.GoldConcHigh = 8, 12
		p.GoldConcSep, p.GoldConcDiff = 32, 5
		resMax++
	case 2:
		p.GoldNormalLow, p.GoldNormalHigh = 0, 1
		p.GoldConcLow, p.GoldConcHigh = 12, 16
		p.GoldConcSep, p.GoldConcDiff = 26, 4
		resMin, resMax = resMin+1, resMax+2
	}

	p.MinResources = resMin
	p.MaxResources = resMax
	return p
}

// resAlloc is the shared working state for all three materials: the single cargo
// map, its size, the RNG and the cross material tunables.
type resAlloc struct {
	cmap         []byte
	w, h         int
	rng          *game.Randomizer
	maxResources int
	mixedSep     int
	mixedVar     int
}

// matAlloc is one material's allocator over the shared cargo map.
type matAlloc struct {
	*resAlloc
	flag             byte
	mx, my           int // grid offset: raw (0,0), gold (1,0), fuel (1,1)
	normal           resourceRange
	concentrate      resourceRange
	concSep, concVar int
	concDiff         int
}

// AllocateResources fills rmap with a fresh game's deposits, boosting raw/fuel
// near each team start. rng is advanced deterministically, so a seeded run is
// reproducible.
func AllocateResources(rmap *ResourceMap, w, h int, starts []game.Point, rng *game.Randomizer) {
	AllocateResourcesWith(rmap, w, h, starts, rng, DefaultResourceParams())
}

// AllocateResourcesWith is AllocateResources with explicit tunables.
//
// C++ ported : game_manager.cpp - GameManager_PopulateMapWithResources
func AllocateResourcesWith(rmap *ResourceMap, w, h int, starts []game.Point, rng *game.Randomizer, p ResourceParams) {
	shared := &resAlloc{
		cmap: make([]byte, w*h), w: w, h: h, rng: rng,
		maxResources: p.MaxResources, mixedSep: p.MixedSeparation, mixedVar: p.MixedSeparation / 5,
	}
	mat := &matAlloc{resAlloc: shared, flag: caRaw, mx: 0, my: 0,
		normal: resourceRange{p.RawNormalLow, p.RawNormalHigh}, concentrate: resourceRange{p.RawConcLow, p.RawConcHigh},
		concSep: p.RawConcSep, concVar: p.RawConcSep / 5, concDiff: p.RawConcDiff}
	gold := &matAlloc{resAlloc: shared, flag: caGold, mx: 1, my: 0,
		normal: resourceRange{p.GoldNormalLow, p.GoldNormalHigh}, concentrate: resourceRange{p.GoldConcLow, p.GoldConcHigh},
		concSep: p.GoldConcSep, concVar: p.GoldConcSep / 5, concDiff: p.GoldConcDiff}
	fuel := &matAlloc{resAlloc: shared, flag: caFuel, mx: 1, my: 1,
		normal: resourceRange{p.FuelNormalLow, p.FuelNormalHigh}, concentrate: resourceRange{p.FuelConcLow, p.FuelConcHigh},
		concSep: p.FuelConcSep, concVar: p.FuelConcSep / 5, concDiff: p.FuelConcDiff}

	minFuel := (p.MaxResources*8 + 10) / 20
	minMat := (p.MaxResources*12 + 10) / 20

	mat.populateCargoMap()
	fuel.populateCargoMap()
	gold.populateCargoMap()

	for _, s := range starts {
		mat.optimize(s.X, s.Y, minMat, 100)
		fuel.optimize(s.X, s.Y, minFuel, 100)
	}

	mat.separateResources(fuel)
	mat.concentrateResources()
	fuel.concentrateResources()
	gold.concentrateResources()
	shared.settleMinimum(p.MinResources)

	for j := 0; j < h; j++ {
		for i := 0; i < w; i++ {
			b := shared.cmap[j*w+i]
			amt := int(b & caAmount)
			// Keep the region's resource type even where the amount tapered to 0,
			// so the survey can mark the cell with that type's empty circle (GOLDMK0
			// / FUELMK0 / RAWMSK0). A cell with no type flag stays truly empty.
			switch b & caFlag {
			case caRaw:
				rmap.SetDeposit(i, j, CargoRaw, amt)
			case caGold:
				rmap.SetDeposit(i, j, CargoGold, amt)
			case caFuel:
				rmap.SetDeposit(i, j, CargoFuel, amt)
			}
		}
	}
}

// approxDistance is the roughly octile metric the spread uses (2*long + short).
//
// C++ ported : access.cpp - Access_GetApproximateDistance
func approxDistance(dx, dy int) int {
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	if dx >= dy {
		return 2*dx + dy
	}
	return 2*dy + dx
}

// zoneLevel sums the amounts in the 2x2 block at (gx,gy), clamped to the map.
//
// C++ ported : resource_allocator.cpp - ResourceAllocator::GetZoneResoureLevel
func (a *resAlloc) zoneLevel(gx, gy int) int {
	ulx, uly, lrx, lry := gx, gy, gx+2, gy+2
	if ulx < 0 {
		ulx = 0
	}
	if uly < 0 {
		uly = 0
	}
	if lrx > a.w {
		lrx = a.w
	}
	if lry > a.h {
		lry = a.h
	}
	sum := 0
	for i := ulx; i < lrx; i++ {
		for j := uly; j < lry; j++ {
			sum += int(a.cmap[a.w*j+i] & caAmount)
		}
	}
	return sum
}

// optimizeResources caps levelMin so the zone at (gx,gy) does not exceed levelMax.
//
// C++ ported : resource_allocator.cpp - ResourceAllocator::OptimizeResources
func (a *resAlloc) optimizeResources(gx, gy, levelMin, levelMax int) int {
	if z := a.zoneLevel(gx, gy); z+levelMin > levelMax {
		levelMin = levelMax - z
	}
	if levelMin < 0 {
		levelMin = 0
	}
	return levelMin
}

// optimize raises a diffusing lode of this material around (px,py): it aligns
// the centre to the material's grid, caps the level against the four touching
// zones, then writes a distance falloff amount over the surrounding cells.
//
// C++ ported : resource_allocator.cpp - ResourceAllocator::Optimize
func (m *matAlloc) optimize(px, py, level, value int) {
	px = (((px+1)-m.mx)/2)*2 + m.mx
	py = (((py+1)-m.my)/2)*2 + m.my
	if px < 0 {
		px = 0
	}
	if py < 0 {
		py = 0
	}
	if px >= m.w {
		px = m.w - 1
	}
	if py >= m.h {
		py = m.h - 1
	}

	lry := py - 4
	if lry < 0 {
		lry = 0
	}
	lry = (lry/2)*2 + m.my
	lrx := px - 4
	if lrx < 0 {
		lrx = 0
	}
	lrx = (lrx/2)*2 + m.mx
	uly := py + 5
	if uly > m.h {
		uly = m.h
	}
	ulx := px + 5
	if ulx > m.w {
		ulx = m.w
	}

	mapAmt := int(m.cmap[py*m.w+px] & caAmount)
	if px > 0 {
		if py > 0 {
			level = m.optimizeResources(px-1, py-1, level, value+mapAmt)
		}
		if py < m.h-1 {
			level = m.optimizeResources(px-1, py, level, value+mapAmt)
		}
	}
	if px < m.w-1 {
		if py > 0 {
			level = m.optimizeResources(px, py-1, level, value+mapAmt)
		}
		if py < m.h-1 {
			level = m.optimizeResources(px, py, level, value+mapAmt)
		}
	}

	level *= 100
	for gx := lrx; gx < ulx; gx += 2 {
		for gy := lry; gy < uly; gy += 2 {
			df := (approxDistance(px-gx, py-gy)*10)/m.concDiff + 10
			val := level / (df * df)
			idx := gy*m.w + gx
			if val > int(m.cmap[idx]&caAmount) {
				m.cmap[idx] = m.flag + byte(val)
			}
		}
	}
}

// populateCargoMap lays the normal background level on every cell of this
// material's grid.
//
// C++ ported : resource_allocator.cpp - ResourceAllocator::PopulateCargoMap
func (m *matAlloc) populateCargoMap() {
	for i := m.mx; i < m.w; i += 2 {
		for j := m.my; j < m.h; j += 2 {
			m.cmap[m.w*j+i] = m.flag + byte(m.normal.value(m.rng))
		}
	}
}

// concentrateResources scatters concentrated lodes on a jittered grid.
//
// C++ ported : resource_allocator.cpp - ResourceAllocator::ConcentrateResources
func (m *matAlloc) concentrateResources() {
	stepX := max(1, (m.concSep*4)/5)
	x0 := m.rng.Generate(stepX + 1)
	y0 := m.rng.Generate(m.concSep/2 + 1)
	odd := false
	for i := x0; i < m.w; i += stepX {
		start := y0
		if odd {
			start = m.concSep/2 + y0
		}
		for j := start; j < m.h; j += m.concSep {
			px := m.rng.Generate(m.concVar*2+1) - m.concVar + i
			py := m.rng.Generate(m.concVar*2+1) - m.concVar + j
			m.optimize(px, py, m.concentrate.value(m.rng), m.maxResources)
		}
		odd = !odd
	}
}

// separateResources scatters mixed lodes of this material and other on a coarser
// jittered grid, pushing the two apart.
//
// C++ ported : resource_allocator.cpp - ResourceAllocator::SeparateResources
func (m *matAlloc) separateResources(other *matAlloc) {
	stepX := max(1, (m.mixedSep*4)/5)
	x0 := m.rng.Generate(stepX + 1)
	y0 := m.rng.Generate(m.mixedSep/2 + 1)
	odd := false
	for i := x0; i < m.w; i += stepX {
		start := y0
		if odd {
			start = m.mixedSep/2 + y0
		}
		for j := start; j < m.h; j += m.mixedSep {
			px := m.rng.Generate(m.mixedVar*2+1) - m.mixedVar + i
			py := m.rng.Generate(m.mixedVar*2+1) - m.mixedVar + j
			m.optimize(px, py, m.rng.Generate(5)+8, m.maxResources)
			other.optimize(px, py, m.rng.Generate(5)+8, m.maxResources)
		}
		odd = !odd
	}
}

// settleMinimum clears any cell whose four touching zones are all below minLevel.
//
// C++ ported : resource_allocator.cpp - ResourceAllocator::SettleMinimumResourceLevels
func (a *resAlloc) settleMinimum(minLevel int) {
	for i := 0; i < a.w; i++ {
		for j := 0; j < a.h; j++ {
			if a.zoneLevel(i-1, j-1) < minLevel && a.zoneLevel(i-1, j) < minLevel &&
				a.zoneLevel(i, j-1) < minLevel && a.zoneLevel(i, j) < minLevel {
				a.cmap[a.w*j+i] = caRaw // CARGO_MATERIALS, amount 0 (empty)
			}
		}
	}
}
