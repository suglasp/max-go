// registry.go : unit registry, spatial cell hash, and ID hash that index live units.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : units_manager.cpp - unit bookkeeping, cell hash, and unit enumeration
package units

import "github.com/suglasp/max-go/internal/game"

// MapHash buckets units by grid cell for fast "what is at (x,y)" queries.
//
// Deviation: the original (hash.cpp MapHash) uses a fixed size open hash
// keyed by a packed grid coordinate. Because the port uses its own save
// format (MAXGOSV), this is an idiomatic Go map keyed by the cell Point,
// identical semantics without a fixed bucket count.
//
// C++: MapHash (hash.cpp)
type MapHash struct {
	cells map[game.Point][]*game.UnitInfo
}

// NewMapHash returns an empty cell hash.
func NewMapHash() *MapHash {
	return &MapHash{cells: map[game.Point][]*game.UnitInfo{}}
}

// Add inserts u into the bucket for every cell it occupies (a building fills
// its whole 2x2 footprint).
// C++: MapHash::Add.
func (h *MapHash) Add(u *game.UnitInfo) {
	for _, p := range footprintCells(u) {
		h.cells[p] = append(h.cells[p], u)
	}
}

// Remove drops u from every cell it occupies. Removing a unit not present
// does nothing.
// C++: MapHash::Remove.
func (h *MapHash) Remove(u *game.UnitInfo) {
	for _, p := range footprintCells(u) {
		list := h.cells[p]
		for i, v := range list {
			if v == u {
				h.cells[p] = append(list[:i], list[i+1:]...)
				if len(h.cells[p]) == 0 {
					delete(h.cells, p)
				}
				break
			}
		}
	}
}

// AddAt reserves cell (x, y) for u without touching its own tracked GridX/GridY -
// it claims a destination the moment a unit commits to entering it, before its
// slide animation finishes, so a second unit's occupancy check sees it as taken
// during the whole crossing, not only once the mover fully arrives.
// C++: MoveInTransitUnitInMapHash (unitinfo.cpp).
func (h *MapHash) AddAt(u *game.UnitInfo, x, y int) {
	p := game.Point{X: x, Y: y}
	h.cells[p] = append(h.cells[p], u)
}

// RemoveAt releases a single-cell reservation taken by AddAt. Removing one not
// present does nothing.
func (h *MapHash) RemoveAt(u *game.UnitInfo, x, y int) {
	p := game.Point{X: x, Y: y}
	list := h.cells[p]
	for i, v := range list {
		if v == u {
			h.cells[p] = append(list[:i], list[i+1:]...)
			if len(h.cells[p]) == 0 {
				delete(h.cells, p)
			}
			break
		}
	}
}

// footprintCells lists the cells a unit occupies in the hash: a building fills
// its whole 2x2 block, so it is indexed at all four cells; every other unit
// occupies just its single position. Without this only a building's top left
// cell blocked, letting another building overlap the other three (the
// collision gap when hovering a build site to the northeast, east, south, or
// southeast of an existing building).
//
// C++: the BUILDING branch of MapHash::Add / MapHash::Remove (hash.cpp).
func footprintCells(u *game.UnitInfo) []game.Point {
	// A builder reserves the entire construction site so passing units route
	// around every occupied cell, not only the corner holding the builder.
	// C++ holds ORDER_BUILD across the slab and tape footprint.
	if u.Building && u.BuildFootprint >= 1 {
		cells := make([]game.Point, 0, u.BuildFootprint*u.BuildFootprint)
		for dy := 0; dy < u.BuildFootprint; dy++ {
			for dx := 0; dx < u.BuildFootprint; dx++ {
				cells = append(cells, game.Point{X: u.BuildSiteX + dx, Y: u.BuildSiteY + dy})
			}
		}
		return cells
	}
	p := u.Position()
	if u.Flags.Has(game.FlagBuilding) {
		return []game.Point{
			p,
			{X: p.X + 1, Y: p.Y},
			{X: p.X, Y: p.Y + 1},
			{X: p.X + 1, Y: p.Y + 1},
		}
	}
	return []game.Point{p}
}

// At returns the units in cell (x, y). The slice is the live bucket; do
// not retain or mutate it across Add/Remove. C++: MapHash::operator[].
func (h *MapHash) At(x, y int) []*game.UnitInfo {
	return h.cells[game.Point{X: x, Y: y}]
}

// Clear empties the hash. C++: MapHash::Clear.
func (h *MapHash) Clear() {
	clear(h.cells)
}

// UnitHash maps a unit id to its instance.
//
// C++: UnitHash (hash.cpp)
type UnitHash struct {
	byID map[uint16]*game.UnitInfo
}

// NewUnitHash returns an empty id hash.
func NewUnitHash() *UnitHash {
	return &UnitHash{byID: map[uint16]*game.UnitInfo{}}
}

// Add registers u by its id. C++: UnitHash::PushBack.
func (h *UnitHash) Add(u *game.UnitInfo) {
	h.byID[u.ID] = u
}

// Remove unregisters u. C++: UnitHash::Remove.
func (h *UnitHash) Remove(u *game.UnitInfo) {
	delete(h.byID, u.ID)
}

// Get returns the unit with the given id, or nil. C++: UnitHash::operator[].
func (h *UnitHash) Get(id uint16) *game.UnitInfo {
	return h.byID[id]
}

// Clear empties the hash. C++: UnitHash::Clear.
func (h *UnitHash) Clear() {
	clear(h.byID)
}

// All returns every registered unit, in unspecified order. Callers that
// need determinism (e.g. the turn loop) sort by ID.
func (h *UnitHash) All() []*game.UnitInfo {
	out := make([]*game.UnitInfo, 0, len(h.byID))
	for _, u := range h.byID {
		out = append(out, u)
	}
	return out
}

// Manager is the unit registry: the spatial cell hash plus the id hash.
// Deploy/Remove keep both in sync. The four unit lists ordered for drawing,
// value tables per team, and the order Execute dispatch attach here in
// later chunks.
//
// C++: the UnitsManager_* globals (units_manager.cpp)
type Manager struct {
	Cells *MapHash
	ByID  *UnitHash
}

// Units returns every registered unit (registry order). C++: iterating the
// UnitsManager unit lists.
func (m *Manager) Units() []*game.UnitInfo {
	return m.ByID.All()
}

// NewManager returns an empty registry.
func NewManager() *Manager {
	return &Manager{Cells: NewMapHash(), ByID: NewUnitHash()}
}

// Deploy places u at a grid cell and indexes it in both hashes.
func (m *Manager) Deploy(u *game.UnitInfo, gridX, gridY int) {
	u.SetPosition(gridX, gridY)
	m.Cells.Add(u)
	m.ByID.Add(u)
}

// Remove takes u out of both hashes (it stays at its last cell otherwise).
func (m *Manager) Remove(u *game.UnitInfo) {
	m.Cells.Remove(u)
	m.ByID.Remove(u)
}

// Move relocates an indexed unit to a new cell, keeping the cell hash
// bucket correct.
func (m *Manager) Move(u *game.UnitInfo, gridX, gridY int) {
	m.Cells.Remove(u)
	u.SetPosition(gridX, gridY)
	m.Cells.Add(u)
}

// Arrive completes a reserved move (see AddAt): it releases the OLD cell and
// updates position, WITHOUT re-adding the destination - AddAt already holds it,
// so the reservation entry silently becomes the real one, matching the
// in transit hash handover (unitinfo.cpp:1773-1783).
func (m *Manager) Arrive(u *game.UnitInfo, gridX, gridY int) {
	m.Cells.Remove(u) // drops the OLD-cell entry (still keyed off the live GridX/GridY)
	u.SetPosition(gridX, gridY)
}
