// clearzone_ladder.go : the way out ladder of the zone clearing task for a trapped base.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ reference: none (port addition, user's way out ladder for a trapped base); see taskclearzone.cpp for the zone task itself.
package ai

import (
	"sort"

	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/sim"
	"github.com/suglasp/max-go/internal/world"
)

const (
	// waterSearchSteps is how many steps an amphibious blocker looks for open water.
	waterSearchSteps = 10

	// storeRestTurns is how many turns a stored unit rests before a routine unload may take it out.
	storeRestTurns = 4
)

// storageWantOrder is the order the clear task offers its storage wants to the building
// manager. A Dock is never raised only to park boats, so it is not in the list.
var storageWantOrder = [...]assets.UnitType{assets.Depot, assets.Barracks}

// moveToWater sends an amphibious blocker onto the nearest open water outside every zone, so
// the land it leaves is free for the rest. It reports whether the unit got the order.
func (t *taskClearZone) moveToWater(blocker *game.UnitInfo, desc access.UnitDesc) bool {
	g := t.ctx.Game

	// A unit that cannot swim has no water to go to.
	if desc.LandType&world.SurfaceWater == 0 {
		return false
	}

	waterX, waterY, ok := t.nearestWater(blocker, desc)

	// No open water in reach: the land search takes over.
	if !ok {
		return false
	}

	// The ledger decides: a unit held by a task that will not let it go stays.
	if !t.mayClaim(blocker) {
		return false
	}

	// Only a unit that really got the order is held: a failed order leaves it free.
	if !g.OrderMove(blocker, waterX, waterY) {
		return false
	}

	t.claim(blocker)
	dbg.Logf(dbg.AI, "team %d moves %s off the island from (%d,%d) to (%d,%d)", t.team, blocker.Type.Tag(), blocker.GridX, blocker.GridY, waterX, waterY)

	return true
}

// nearestWater walks outward from the blocker over cells it can enter and no ground unit
// stands on, up to waterSearchSteps steps, and returns the nearest open water cell (not
// coast) outside every zone.
func (t *taskClearZone) nearestWater(blocker *game.UnitInfo, desc access.UnitDesc) (int, int, bool) {
	g := t.ctx.Game
	inZone := t.zoneCells()
	start := [2]int{blocker.GridX, blocker.GridY}
	steps := map[[2]int]int{start: 0}
	queue := [][2]int{start}

	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]

		// The search stops at its step bound.
		if steps[c] >= waterSearchSteps {
			continue
		}

		for _, step := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			n := [2]int{c[0] + step[0], c[1] + step[1]}

			if _, seen := steps[n]; seen || n[0] < 0 || n[1] < 0 || n[0] >= g.W || n[1] >= g.H {
				continue
			}

			// Terrain and buildings only: a cell no mobile unit could stand on is a wall.
			if !g.Grid.IsAccessible(desc, t.team, n[0], n[1], access.IgnoreMobileUnits).Passable() {
				continue
			}

			// The blocker drives itself, so a ground unit on the cell is a wall too.
			if t.groundUnitOn(n) {
				continue
			}

			steps[n] = steps[c] + 1

			// Open water outside every zone: the first one found is the nearest.
			if g.Grid.Surf.SurfaceAt(n[0], n[1]) == world.SurfaceWater && !inZone[n] {
				return n[0], n[1], true
			}

			queue = append(queue, n)
		}
	}

	return 0, 0, false
}

// groundUnitOn reports whether any ground mobile unit stands on the cell.
func (t *taskClearZone) groundUnitOn(c [2]int) bool {
	for _, o := range t.ctx.Game.Units.Cells.At(c[0], c[1]) {
		if isGroundMobile(o) {
			return true
		}
	}

	return false
}

// turnNow returns the game turn, 0 when the context has no game state.
func (t *taskClearZone) turnNow() int {
	g := t.ctx.Game

	// A bare game without state has no turn counter.
	if g.State == nil {
		return 0
	}

	return g.State.TurnCounter
}

// storeBlocker parks a blocker that has no free cell in an own storage building of its
// class: it boards at once when it stands next to one, otherwise it drives to a cell beside
// the nearest one with room. With no room anywhere it records the want, so the building
// manager raises one.
func (t *taskClearZone) storeBlocker(blocker *game.UnitInfo) {
	g := t.ctx.Game

	// A requester waits on its own zone, and a held unit is not ours to move.
	if t.isRequester(blocker) || !t.mayClaim(blocker) {
		return
	}

	// A builder is never stored: the base needs it out, and a stored one would be built again.
	if blocker.Flags.Has(game.FlagConstructorUnit) {
		return
	}

	shopType, ok := repairShopType(blocker)

	// A unit with no class has no storage, and an aircraft never blocks a builder.
	if !ok || shopType == assets.Hangar {
		return
	}

	building := t.nearestStorage(blocker, shopType)

	// No Dock has room: a Dock is not raised only to park boats, so there is no want.
	if building == nil && shopType == assets.Dock {
		return
	}

	// No building of the class has room: ask for one, and log the first ask of the turn.
	if building == nil {
		if !t.wantStorage[shopType] {
			dbg.Logf(dbg.AI, "team %d wants a %s to store units", t.team, shopType.Tag())
		}

		t.wantStorage[shopType] = true

		return
	}

	// Next to the building: board now.
	if g.CanLoad(building, blocker) {
		if g.LoadUnit(building, blocker) {
			t.storedAt[blocker] = t.turnNow()
			dbg.Logf(dbg.AI, "team %d stores %s in %s at (%d,%d)", t.team, blocker.Type.Tag(), building.Type.Tag(), building.GridX, building.GridY)
		}

		return
	}

	loadX, loadY, ok := g.LoadCell(building, blocker)

	// The building is boxed in: no cell to board from.
	if !ok {
		return
	}

	// Only a unit that really got the order is held and awaited.
	if g.OrderMove(blocker, loadX, loadY) {
		t.claim(blocker)
		t.storing[blocker] = building
		dbg.Logf(dbg.AI, "team %d sends %s to be stored in %s at (%d,%d)", t.team, blocker.Type.Tag(), building.Type.Tag(), building.GridX, building.GridY)
	}
}

// nearestStorage returns the team's nearest building of the type that has room for the
// unit, the lower ID on a tie, or nil.
func (t *taskClearZone) nearestStorage(unit *game.UnitInfo, shopType assets.UnitType) *game.UnitInfo {
	g := t.ctx.Game
	var best *game.UnitInfo
	bestDistance := 0

	for _, b := range t.storageBuildings() {
		if b.Type != shopType || !g.CanContain(b, unit) {
			continue
		}

		// The buildings come in ID order, so a tie keeps the lower ID.
		if d := sim.SquaredDist(unit.GridX, unit.GridY, b.GridX, b.GridY); best == nil || d < bestDistance {
			best, bestDistance = b, d
		}
	}

	return best
}

// storageBuildings returns the team's standing storage buildings in ID order.
func (t *taskClearZone) storageBuildings() []*game.UnitInfo {
	var buildings []*game.UnitInfo

	for _, b := range t.ctx.Game.Units.Units() {
		if b.Team == t.team && b.Hits > 0 && sim.IsStorageBuilding(b.Type) {
			buildings = append(buildings, b)
		}
	}

	// The unit list comes back in random order: sort for a stable pick.
	sort.Slice(buildings, func(i, j int) bool { return buildings[i].ID < buildings[j].ID })

	return buildings
}

// finishStores boards the units that were sent to a storage building once they stand next
// to it, and drops a plan whose unit or building is gone or whose unit stopped short.
func (t *taskClearZone) finishStores() {
	g := t.ctx.Game
	pending := make([]*game.UnitInfo, 0, len(t.storing))

	for u := range t.storing {
		pending = append(pending, u)
	}

	// Map order is random: go through the units by ID.
	sort.Slice(pending, func(i, j int) bool { return pending[i].ID < pending[j].ID })

	for _, u := range pending {
		building := t.storing[u]

		// The unit or the building is gone: the plan is off.
		if u.Hits <= 0 || u.Parent != nil || !unitOnMap(g, u) || building.Hits <= 0 || !unitOnMap(g, building) {
			delete(t.storing, u)
			continue
		}

		// Next to the building at last: board, and the plan is done either way.
		if g.CanLoad(building, u) {
			if g.LoadUnit(building, u) {
				t.storedAt[u] = t.turnNow()
				dbg.Logf(dbg.AI, "team %d stores %s in %s at (%d,%d)", t.team, u.Type.Tag(), building.Type.Tag(), building.GridX, building.GridY)
			}

			delete(t.storing, u)
			continue
		}

		// No order left and still not next to it: the unit did not make it.
		if !g.HasOrder(u) {
			delete(t.storing, u)
		}
	}
}

// wantedStorage returns the storage building type the zone task needs, Depot first, then
// Barracks, as it recorded in its last pass. The building manager reads it
// before the zone task runs, so it sees the wants of the previous turn.
func (t *taskClearZone) wantedStorage() (assets.UnitType, bool) {
	for _, storageType := range storageWantOrder {
		if t.wantStorage[storageType] {
			return storageType, true
		}
	}

	return 0, false
}

// unloadStored drops the first stored unit that match accepts and returns it, or nil. A
// routine unload takes only a unit that rested storeRestTurns turns, an urgent one takes
// any. Buildings and their units are walked in a stable order.
func (t *taskClearZone) unloadStored(match func(u *game.UnitInfo) bool, urgent bool) *game.UnitInfo {
	now := t.turnNow()

	for _, building := range t.storageBuildings() {
		for _, u := range building.Carried {
			if !match(u) {
				continue
			}

			// A routine unload leaves the unit its rest: a unit never seen stored counts as rested.
			if at, stored := t.storedAt[u]; !urgent && stored && now-at < storeRestTurns {
				continue
			}

			if t.unloadOne(building, u, urgent) {
				return u
			}
		}
	}

	return nil
}

// unloadOne drops u from the building onto a free cell around it, outside every zone. An
// urgent unload falls back on a zone cell when no other cell is free. It reports success.
func (t *taskClearZone) unloadOne(building, u *game.UnitInfo, urgent bool) bool {
	g := t.ctx.Game
	inZone := t.zoneCells()
	passes := 1

	// An urgent unload may take a zone cell in a second pass.
	if urgent {
		passes = 2
	}

	for pass := 0; pass < passes; pass++ {
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				if dx == 0 && dy == 0 {
					continue
				}

				cell := [2]int{building.GridX + dx, building.GridY + dy}

				// The first pass takes cells outside every zone, the second one only zone cells.
				if inZone[cell] != (pass == 1) {
					continue
				}

				if g.UnloadUnit(building, u, cell[0], cell[1]) {
					delete(t.storedAt, u)
					dbg.Logf(dbg.AI, "team %d unloads %s from %s (%s) to (%d,%d)", t.team, u.Type.Tag(), building.Type.Tag(), unloadKind(urgent), cell[0], cell[1])

					return true
				}
			}
		}
	}

	return false
}

// unloadKind names the kind of an unload for the log.
func unloadKind(urgent bool) string {
	if urgent {
		return "urgent"
	}

	return "routine"
}

// unloadForThreat sends out every armed stored unit of a storage building that a spotted
// enemy presses, at once and whatever their rest.
func (t *taskClearZone) unloadForThreat() {
	g := t.ctx.Game

	for _, building := range t.storageBuildings() {
		// Nothing stored, or no enemy near: leave the building alone.
		if len(building.Carried) == 0 || spottedEnemyNear(g, t.team, building.GridX, building.GridY, guardEngageRadius) == nil {
			continue
		}

		var armed []*game.UnitInfo

		for _, u := range building.Carried {
			if armedMobileUnit(u, t.team) {
				armed = append(armed, u)
			}
		}

		// The list is taken first: an unload edits the building's own list.
		for _, u := range armed {
			t.unloadOne(building, u, true)
		}
	}
}

// unloadRoutine lets one rested stored unit out when no zone is open, so the room it was
// kept for is back.
func (t *taskClearZone) unloadRoutine() {
	// An open zone still needs the room.
	if len(t.zones) > 0 {
		return
	}

	t.unloadStored(func(*game.UnitInfo) bool { return true }, false)
}
