// vis.go : the M.A.X. per team visibility, and fog of war layer. Scan coverage heat maps and stealth gated unit visibility.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : access.cpp / heatmap.cpp - Access_UpdateMapStatus, Access_IsVisibleOnHeatMap, HeatMap
//
// Note: Package vis is the per team visibility/scan/FOG layer.
// The second half of access.cpp's map status machinery.
// Each active team owns a HeatMap of scan coverage reference counters.
// From those counters the layer answers "is this enemy unit visible to that team?"
// (with stealth gating for submarines and commandos),
// and "what is the fog state of this cell" for the minimap/tactical renderer?
//
// It imports units/game/world (it needs the live unit set),
// so it sits above access in the import DAG, access stays unit agnostic via its interfaces,
// while the unit aware visibility computation lives here.
package vis

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/world"
)

// tileSize is GFX_MAP_TILE_SIZE: scan range is measured in tiles but compared
// in pixels (a tile is 64 pixels), giving the circular reveal its shape.
const tileSize = 64

// Fog is a cell's exploration state from one team's point of view.
//
// C++: the minimap fov has only two states (lit / darkened via
// ResourceManager_DarkeningColorIndexTable); we add Unseen so a cell never
// scanned can be drawn fully black.
type Fog uint8

const (
	FogUnseen   Fog = iota // never within scan range, draw black
	FogExplored            // seen before, not currently scanned, draw darkened
	FogVisible             // currently within a team unit's scan range
)

// SurfaceSource yields the surface mask of a cell; vis needs it only to tell
// whether a colony transport (CLNTRANS) is currently over water and therefore
// submerged. It is satisfied by *world.Map and the synthetic test surfaces.
//
// C++: Access_GetModifiedSurfaceType (access.cpp).
type SurfaceSource interface {
	SurfaceAt(x, y int) uint8
}

// HeatMap is one team's scan coverage. Every cell holds three reference
// counts: complete (any selectable unit covering it), stealthSea (sonar,
// CORVETTE coverage) and stealthLand (COMMANDO/INFANTRY coverage). explored
// is the persistent fog memory: set the first time a cell is ever covered and
// never cleared, so a cell that leaves scan range stays "explored".
//
// C++: HeatMap with HeatMapCell{complete, stealth_sea, stealth_land}
// (heatmap.hpp); explored is our addition for fog memory.
type HeatMap struct {
	W, H        int
	complete    []int
	stealthSea  []int
	stealthLand []int
	explored    []bool
}

// NewHeatMap allocates a cleared heat map for a wxh map.
func NewHeatMap(w, h int) *HeatMap {
	n := w * h
	return &HeatMap{
		W:           w,
		H:           h,
		complete:    make([]int, n),
		stealthSea:  make([]int, n),
		stealthLand: make([]int, n),
		explored:    make([]bool, n),
	}
}

// inBounds reports whether x/y addresses a heat map cell.
func (m *HeatMap) inBounds(x, y int) bool {
	return x >= 0 && y >= 0 && x < m.W && y < m.H
}

// idx converts x/y coordinates to a flat heat map index.
func (m *HeatMap) idx(x, y int) int {
	return y*m.W + x
}

// Complete returns the scan coverage count at a cell (0 = not in range).
//
// C++: HeatMap::GetComplete (heatmap.cpp).
func (m *HeatMap) Complete(x, y int) int {
	if !m.inBounds(x, y) {
		return 0
	}
	return m.complete[m.idx(x, y)]
}

// StealthSea returns the sonar (submarine detection) coverage count at a cell.
//
// C++: HeatMap::GetStealthSea (heatmap.cpp).
func (m *HeatMap) StealthSea(x, y int) int {
	if !m.inBounds(x, y) {
		return 0
	}
	return m.stealthSea[m.idx(x, y)]
}

// StealthLand returns the land stealth detection coverage count at a cell.
//
// C++: HeatMap::GetStealthLand (heatmap.cpp).
func (m *HeatMap) StealthLand(x, y int) int {
	if !m.inBounds(x, y) {
		return 0
	}
	return m.stealthLand[m.idx(x, y)]
}

// Explored reports whether a cell has ever been within this team's scan range.
func (m *HeatMap) Explored(x, y int) bool {
	if !m.inBounds(x, y) {
		return false
	}
	return m.explored[m.idx(x, y)]
}

// FogAt classifies a cell: Visible if currently covered, else Explored if it
// was ever covered, else Unseen.
//
// C++: Access_UpdateMinimapFogOfWar: complete==0 cells get darkened
// (access.cpp); the Unseen vs Explored split is our fog memory.
func (m *HeatMap) FogAt(x, y int) Fog {
	if m.Complete(x, y) > 0 {
		return FogVisible
	}
	if m.Explored(x, y) {
		return FogExplored
	}
	return FogUnseen
}

// clear zeroes the coverage counters but keeps the explored memory; the
// recompute each turn starts from no current coverage.
func (m *HeatMap) clear() {
	for i := range m.complete {
		m.complete[i] = 0
		m.stealthSea[i] = 0
		m.stealthLand[i] = 0
	}
}

// addCoverage stamps one unit's circular scan zone into the counters. sea and
// land select which stealth detection layers the unit also contributes to.
//
// C++: the loop over the scan zone in Access_UpdateMapStatus gated by
// Access_IsWithinScanRange, plus HeatMap::Add (access.cpp / heatmap.cpp).
func (m *HeatMap) addCoverage(gx, gy, scan int, sea, land bool) {
	if scan < 0 {
		scan = 0
	}
	scanArea := (scan * tileSize) * (scan * tileSize)
	for y := gy - scan; y <= gy+scan; y++ {
		for x := gx - scan; x <= gx+scan; x++ {
			if !m.inBounds(x, y) {
				continue
			}
			dx := (x - gx) * tileSize
			dy := (y - gy) * tileSize
			if dx*dx+dy*dy > scanArea {
				continue
			}
			i := m.idx(x, y)
			m.complete[i]++
			m.explored[i] = true
			if sea {
				m.stealthSea[i]++
			}
			if land {
				m.stealthLand[i]++
			}
		}
	}
}

// View holds the per team heat maps and recomputes them from the live unit
// set. It is the unit aware facade the game screen and the headless harness
// talk to.
type View struct {
	W, H int
	Surf SurfaceSource // optional; only needed to submerge CLNTRANS over water
	maps [game.TeamCount]*HeatMap
}

// NewView builds a View with a fresh heat map per team.
func NewView(w, h int) *View {
	v := &View{W: w, H: h}
	for t := range v.maps {
		v.maps[t] = NewHeatMap(w, h)
	}
	return v
}

// Team returns a team's heat map (for fog queries by the renderer).
func (v *View) Team(team int) *HeatMap {
	if team < 0 || team >= len(v.maps) {
		return nil
	}
	return v.maps[team]
}

// contributes reports whether a unit projects scan coverage for its team this
// turn, and which stealth detection layers it feeds.
//
// C++: the (flags & SELECTABLE) && order != IDLE/DISABLE gate plus the
// CORVETTE / COMMANDO|INFANTRY type checks in Access_UpdateMapStatus &
// Access_UpdateMapStatusAddUnit (access.cpp).
func contributes(u *game.UnitInfo) (ok, sea, land bool) {
	if !u.Flags.Has(game.FlagSelectable) {
		return false, false, false
	}
	// A unit still parked in its factory, or a building not yet revealed behind its parked builder, is idle in the
	// original (ORDER_IDLE, ORDER_STATE_BUILDING_READY) and scans nothing until it leaves or unfolds.
	// C++ ported : unitinfo.cpp - the output set to ORDER_IDLE + ORDER_STATE_BUILDING_READY
	if u.HiddenUntilExpand {
		return false, false, false
	}
	if u.Order == game.OrderIdle || u.Order == game.OrderDisable {
		return false, false, false
	}
	sea = u.Type == assets.Corvette
	land = u.Type == assets.Commando || u.Type == assets.Infantry
	return true, sea, land
}

// scanRange is a unit's ATTRIB_SCAN in tiles (0 if it carries no values).
func scanRange(u *game.UnitInfo) int {
	if u.Values == nil {
		return 0
	}
	return u.Values.Get(game.AttribScan)
}

// Rebuild recomputes every team's coverage from the current unit set: it
// clears the counters (keeping fog memory) then stamps each contributing
// unit's scan zone into its own team's heat map.
//
// C++: the sweep each turn of Access_UpdateMapStatus over every unit
// (access.cpp), collapsed into a single recompute for the headless model.
func (v *View) Rebuild(allUnits []*game.UnitInfo) {
	for _, m := range v.maps {
		m.clear()
	}
	for _, u := range allUnits {
		if u.Team < 0 || u.Team >= len(v.maps) {
			continue
		}
		ok, sea, land := contributes(u)
		if !ok {
			continue
		}
		v.maps[u.Team].addCoverage(u.GridX, u.GridY, scanRange(u), sea, land)
	}
}

// exportExplored packs the fog memory into one bit per cell, row major.
func (m *HeatMap) exportExplored() []byte {
	bits := make([]byte, (len(m.explored)+7)/8)
	for i, seen := range m.explored {
		if seen {
			bits[i/8] |= 1 << uint(i%8)
		}
	}

	return bits
}

// importExplored ORs a packed bitmap back into the fog memory. A short or nil slice
// leaves the cells it does not reach untouched.
func (m *HeatMap) importExplored(bits []byte) {
	for i := range m.explored {
		if i/8 >= len(bits) {
			return
		}
		if bits[i/8]&(1<<uint(i%8)) != 0 {
			m.explored[i] = true
		}
	}
}

// ExportExplored packs every team's fog memory, one bitmap per team, for the save.
// The scan counters are re derived from the units on load, but the explored memory
// cannot be: it is the record of where a team has BEEN, so without it a reload blacks
// out every cell the team walked and is not standing in right now.
func (v *View) ExportExplored() [][]byte {
	out := make([][]byte, len(v.maps))
	for team, m := range v.maps {
		out[team] = m.exportExplored()
	}

	return out
}

// ImportExplored merges saved fog memory back into the team heat maps. It ORs rather
// than overwrites, so a cell a unit is covering right now stays explored whatever the
// save held for it.
func (v *View) ImportExplored(teams [][]byte) {
	for team, bits := range teams {
		if team >= len(v.maps) {
			return
		}
		v.maps[team].importExplored(bits)
	}
}

// isUnderWater reports whether a unit is currently submerged (and thus only
// detectable by sonar). A submarine is submerged unless it has surfaced (to
// attack or under recent combat); a surfaced submarine is a normal unit,
// visible to general scan coverage like an AWAC. Colony transports are
// submerged when sitting over water.
//
// C++: UnitsManager_IsUnitUnderWater (units_manager.cpp); the sonar coverage
// that reveals a submerged submarine is fed only by a Corvette
// (resource_manager.cpp sea_qualifier), but a surfaced sub is visible to
// general coverage the same as any other unit.
func (v *View) isUnderWater(u *game.UnitInfo) bool {
	switch u.Type {
	case assets.Submarne:
		return !u.Surfaced
	case assets.ClnTrans:
		return v.Surf != nil && v.Surf.SurfaceAt(u.GridX, u.GridY)&world.SurfaceWater != 0
	default:
		return false
	}
}

// VisibleToTeam reports whether unit u is visible to the given team. A team
// always sees its own units; an enemy unit is visible when the team's heat map
// covers its cell, with stealth gating: a submerged unit needs sonar
// (stealthSea) coverage, a commando needs land stealth (stealthLand) coverage,
// and a 2x2 building is visible if any of its four corners is covered.
//
// C++: Access_IsVisibleOnHeatMap + the SpotByTeam conditions in
// Access_DrawUnit (access.cpp), evaluated directly from coverage rather than
// via the persistent visible_to_team flag.
func (v *View) VisibleToTeam(u *game.UnitInfo, team int) bool {
	m := v.Team(team)
	if m == nil {
		return false
	}
	if u.Team == team {
		return true
	}
	if u.Order == game.OrderIdle {
		return false
	}
	switch {
	case v.isUnderWater(u):
		return m.StealthSea(u.GridX, u.GridY) > 0
	case u.Type == assets.Commando:
		// A commando is hidden by land stealth from the enemy unless a land
		// detector (commando/infantry coverage) reveals them.
		return m.StealthLand(u.GridX, u.GridY) > 0
	case isMine(u):
		// A laid mine is not on the heat map at all: only a mine detector that
		// walked up to it uncovers it, and then it stays uncovered.
		return u.SpottedByTeam(team)
	case u.Flags.Has(game.FlagBuilding):
		return m.Complete(u.GridX, u.GridY) > 0 ||
			m.Complete(u.GridX+1, u.GridY) > 0 ||
			m.Complete(u.GridX, u.GridY+1) > 0 ||
			m.Complete(u.GridX+1, u.GridY+1) > 0
	default:
		return m.Complete(u.GridX, u.GridY) > 0
	}
}

// isMine reports whether u is a laid mine (land or sea).
func isMine(u *game.UnitInfo) bool {
	return u.Type == assets.LandMine || u.Type == assets.SeaMine
}

// StealthHiddenFrom reports whether u is a STEALTH unit - a laid mine, a
// submerged submarine / colony transport, or a commando - that the given team
// cannot currently detect, and so must not be drawn or clicked by that team.
//
// Unlike VisibleToTeam this consults ONLY stealth detection (sonar, land stealth,
// or the sticky mine mark), never general scan/fog: a unit with no stealth returns
// false (it is hidden the usual way, by the fog overlay painting over unscanned
// cells), so this is safe to gate the world render on without also hiding idle
// enemy structures the way VisibleToTeam would. A team always sees its own units.
//
// A mine is the odd one out: it is not on the heat map, so no amount of scan
// coverage finds it. Only a mine detector (surveyor / mine layer / commando) that
// came within a cell of it uncovers it, and that mark sticks - sim.sweepMineDetectors
// sets UnitInfo.SpottedBy.
//
// C++: the stealth (SpotByTeam) branch of Access_DrawUnit (access.cpp) plus
// Survey_SurveyArea (survey.cpp) for the mines.
func (v *View) StealthHiddenFrom(u *game.UnitInfo, team int) bool {
	if u == nil || u.Team == team {
		return false
	}
	m := v.Team(team)
	if m == nil {
		return false
	}
	switch {
	case isMine(u):
		return !u.SpottedByTeam(team)
	case u.Type == assets.Commando:
		return m.StealthLand(u.GridX, u.GridY) == 0
	case v.isUnderWater(u):
		return m.StealthSea(u.GridX, u.GridY) == 0
	default:
		return false
	}
}
