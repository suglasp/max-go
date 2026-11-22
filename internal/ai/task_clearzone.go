// task_clearzone.go moves the team's parked units off cells a builder needs.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
// C++ reference: taskclearzone.cpp, TaskClearZone::ExamineZones, EvaluateSite, and AddZone.
package ai

import (
	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/dbg"
	"github.com/suglasp/max-go/internal/game"
)

// clearZoneTurns is how many turns a zone stays open while its units shift out of it.
// A unit moves one link of a chain per turn. If the zone remains blocked when the timer
// expires, the task drops it and the requester can ask again.
const clearZoneTurns = 4

// clearZone is the set of cells a requester (a boxed in builder, a build site) needs
// free of the team's own parked units. C++: class Zone (zone.hpp).
type clearZone struct {
	requester *game.UnitInfo // the unit the zone is cleared for; it never counts as a blocker
	cells     [][2]int       // the cells to clear
	turnsLeft int            // turns the zone stays open
}

// taskClearZone moves the team's own idle mobile units off the zones the building
// manager asks for, so a parked army never walls in a builder or sits on a build site.
// It runs at the emergency tier and can take a unit from another task for one move.
// C++ reference: taskclearzone.cpp, TaskClearZone(team, MOBILE_LAND_UNIT | MOBILE_SEA_UNIT).
//
// Differences from the original: one ground zone task per team (no air zones, since an
// aircraft never blocks a builder); each turn moves the last unit of each blocking chain
// to the nearest free cell outside every zone, through the direct move order, instead of
// stepping each unit in the chain through path requests; destination cells are not
// checked for enemy threat, as they are in the original.
//
// Port additions, asked for by the user and not in the reference task: a way out ladder
// for a trapped base (clearzone_ladder.go). An amphibious blocker goes onto open water
// first. A blocker with no free cell (never a builder) is stored in an own Depot, Barracks
// or Dock; with no Depot or Barracks room the task asks the building manager for one.
// Stored units come out again, at once when a task asks for the unit or a spotted enemy
// presses the storage (urgent), and one at a time after a rest once no zone is open
// (routine).
type taskClearZone struct {
	TaskBase
	ctx         *aiContext
	zones       []clearZone
	storing     map[*game.UnitInfo]*game.UnitInfo // units on their way to be stored, and the building each is bound for
	storedAt    map[*game.UnitInfo]int            // the turn a unit was stored, for the rest before a routine unload
	wantStorage map[assets.UnitType]bool          // the storage types with no room, recorded in this turn's pass
}

// newTaskClearZone builds the team's zone clearing task at the emergency tier.
// C++: TaskClearZone::TaskClearZone (TASK_PRIORITY_EMERGENCY).
func newTaskClearZone(team int, ctx *aiContext) *taskClearZone {
	return &taskClearZone{
		TaskBase:    newTaskBase(team, nil, taskPriorityEmergency),
		ctx:         ctx,
		storing:     map[*game.UnitInfo]*game.UnitInfo{},
		storedAt:    map[*game.UnitInfo]int{},
		wantStorage: map[assets.UnitType]bool{},
	}
}

// addZone opens a zone for requester, replacing an older zone of the same requester so
// one builder never holds two. C++ reference: taskclearzone.cpp, TaskClearZone::AddZone.
func (t *taskClearZone) addZone(requester *game.UnitInfo, cells [][2]int) {
	zone := clearZone{requester: requester, cells: cells, turnsLeft: clearZoneTurns}

	for i := range t.zones {
		// The same requester asks again: the new cells replace the old ones.
		if t.zones[i].requester == requester {
			t.zones[i] = zone
			return
		}
	}

	t.zones = append(t.zones, zone)
}

// BeginTurn finishes the pending stores, examines each zone and moves a parked unit off
// it: a zone with no blocker left is done, one with an enemy on it or past its turns is
// dropped. It then lets stored units out when the storage is pressed or the room is back.
// C++ reference: taskclearzone.cpp, TaskClearZone::EndTurn, ExamineZones, and SearchMap.
func (t *taskClearZone) BeginTurn() {
	g := t.ctx.Game

	if g == nil {
		return
	}

	// The building manager runs before this task and has read last turn's wants already:
	// this pass starts a fresh record, which the manager reads next turn.
	t.wantStorage = map[assets.UnitType]bool{}

	// Units sent to a storage building board once they stand next to it.
	t.finishStores()

	kept := t.zones[:0]

	for _, zone := range t.zones {
		blockers, enemy := t.examineZone(zone)

		switch {
		case enemy:
			// An enemy stands on the zone: the requester must plan again.
			dbg.Logf(dbg.AI, "team %d drops the clear zone of %s: an enemy stands on it", t.team, zone.requester.Type.Tag())
			continue
		case len(blockers) == 0:
			// Nobody of ours is in the way any more.
			dbg.Logf(dbg.AI, "team %d cleared the zone of %s", t.team, zone.requester.Type.Tag())
			continue
		case zone.turnsLeft <= 0:
			// The units could not shift out in time; the requester asks again if it still needs it.
			dbg.Logf(dbg.AI, "team %d gives up the clear zone of %s: %d units still in the way", t.team, zone.requester.Type.Tag(), len(blockers))
			continue
		}

		for _, u := range blockers {
			t.moveAside(u)
		}

		zone.turnsLeft--
		kept = append(kept, zone)
	}

	t.zones = kept

	// A spotted enemy near a storage building sends its armed units out at once.
	t.unloadForThreat()

	// With no zone open the room is back: one rested unit comes out.
	t.unloadRoutine()
}

// examineZone returns the team's own clearable units standing on the zone, and whether a
// unit of another team stands on it. Busy units stay in place and leave under their own
// orders. C++ reference: taskclearzone.cpp, TaskClearZone::ExamineZones.
func (t *taskClearZone) examineZone(zone clearZone) ([]*game.UnitInfo, bool) {
	g := t.ctx.Game
	var blockers []*game.UnitInfo

	for _, c := range zone.cells {
		for _, o := range g.Units.Cells.At(c[0], c[1]) {
			if o == zone.requester || !isGroundMobile(o) {
				continue
			}

			// Another team's unit on the zone ends it.
			if o.Team != t.team {
				return nil, true
			}

			if isClearableUnit(t.ctx, t.team, o) {
				blockers = append(blockers, o)
			}
		}
	}

	return blockers, false
}

// moveAside finds the nearest free cell outside every zone that the blocker can reach,
// walking through the team's other parked units, and moves the last parked unit on that
// way into it: its way to the cell is open, and the next unit of the chain follows on
// the next turn. An amphibious blocker goes onto open water first, and a blocker with no
// free cell to go to is stored. C++ reference: taskclearzone.cpp, TaskClearZone::SearchMap
// and EvaluateSite.
func (t *taskClearZone) moveAside(blocker *game.UnitInfo) {
	g := t.ctx.Game
	start := [2]int{blocker.GridX, blocker.GridY}
	inZone := t.zoneCells()
	desc := access.UnitDesc{Type: blocker.Type, Flags: blocker.Flags, LandType: access.LandTypeForUnit(blocker.Type, blocker.Flags)}

	// An amphibious blocker goes onto open water first, which leaves the land to the rest.
	if t.moveToWater(blocker, desc) {
		return
	}

	parent := map[[2]int][2]int{start: start}
	queue := [][2]int{start}

	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]

		for _, step := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			n := [2]int{c[0] + step[0], c[1] + step[1]}

			if _, seen := parent[n]; seen || n[0] < 0 || n[1] < 0 || n[0] >= g.W || n[1] >= g.H {
				continue
			}

			// Terrain and buildings only: a cell no mobile unit could stand on is a wall.
			if !g.Grid.IsAccessible(desc, t.team, n[0], n[1], access.IgnoreMobileUnits).Passable() {
				continue
			}

			mover, occupied := t.groundOccupant(n)

			// Another unit that cannot be moved blocks the way.
			if occupied && mover == nil {
				continue
			}

			parent[n] = c

			// A free cell outside every zone: the chain ends here.
			if !occupied && !inZone[n] {
				t.moveChainEnd(n, parent)
				return
			}

			queue = append(queue, n)
		}
	}

	dbg.Logf(dbg.AI, "team %d finds no free cell to move %s at (%d,%d) aside", t.team, blocker.Type.Tag(), blocker.GridX, blocker.GridY)

	// The way out ladder: no free cell, so store the blocker or ask for storage.
	t.storeBlocker(blocker)
}

// moveChainEnd walks the way back from the free cell dest and moves the parked unit
// nearest to it there. C++: the zone square chain EvaluateSite moves along.
func (t *taskClearZone) moveChainEnd(dest [2]int, parent map[[2]int][2]int) {
	g := t.ctx.Game

	for c := parent[dest]; ; c = parent[c] {
		if mover, _ := t.groundOccupant(c); mover != nil {
			// The ledger decides: a unit held by a task that will not let it go stays.
			if !t.mayClaim(mover) {
				return
			}

			// Only a unit that really got the order is held: a failed order leaves it free.
			if g.OrderMove(mover, dest[0], dest[1]) {
				t.claim(mover)
				dbg.Logf(dbg.AI, "team %d moves %s from (%d,%d) to (%d,%d) to clear a zone", t.team, mover.Type.Tag(), mover.GridX, mover.GridY, dest[0], dest[1])
			}

			return
		}

		// The way back reached its start without a unit on it.
		if parent[c] == c {
			return
		}
	}
}

// groundOccupant reports whether a ground mobile unit stands on the cell, and returns it
// when it is one of the team's clearable units (nil for any other occupant).
func (t *taskClearZone) groundOccupant(c [2]int) (*game.UnitInfo, bool) {
	for _, o := range t.ctx.Game.Units.Cells.At(c[0], c[1]) {
		if !isGroundMobile(o) {
			continue
		}

		// A requester or any busy unit makes the cell a wall.
		if !isClearableUnit(t.ctx, t.team, o) || t.isRequester(o) {
			return nil, true
		}

		return o, true
	}

	return nil, false
}

// isRequester reports whether u is the requester of an open zone.
func (t *taskClearZone) isRequester(u *game.UnitInfo) bool {
	for _, zone := range t.zones {
		if zone.requester == u {
			return true
		}
	}

	return false
}

// zoneCells returns the cells of every open zone, so no unit is moved into another zone.
func (t *taskClearZone) zoneCells() map[[2]int]bool {
	cells := map[[2]int]bool{}

	for _, zone := range t.zones {
		for _, c := range zone.cells {
			cells[c] = true
		}
	}

	return cells
}

// isGroundMobile reports whether u is a land or sea mobile unit (the movement classes that
// block a builder). An aircraft never blocks one.
func isGroundMobile(u *game.UnitInfo) bool {
	return u.Flags.Has(game.FlagMobileLandUnit) || u.Flags.Has(game.FlagMobileSeaUnit)
}

// isClearableUnit reports whether u is one of the team's ground units parked with nothing
// to do, so it may be moved out of a zone: alive, not building, no move or build order.
// C++: the ORDER_AWAIT / ORDER_SENTRY test of TaskClearZone::ExamineZones.
func isClearableUnit(ctx *aiContext, team int, u *game.UnitInfo) bool {
	g := ctx.Game

	if u.Team != team || !isGroundMobile(u) || u.Hits <= 0 || u.Building {
		return false
	}

	return !g.HasOrder(u) && g.BuildOrderFor(u) == nil
}
