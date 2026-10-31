// buildexit.go : moves constructors and engineers out of completed building footprints.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : unitinfo.cpp - builder exit after BuildingReady
package sim

// buildexit.go: the build ready exit flow (faithful ORDER_STATE_UNIT_READY).
// When a constructor/engineer finishes a building it parks on the site and waits
// in a "build ready" state.
// - A human player is shown animated exit markers (BLDMRK) on the accessible cells
//   around the finished footprint and clicks one to drive the builder off.
// - A computer team's builder picks an exit on its own.
//
// C++ ported : drawmap.cpp - DrawMap_RedrawUnit (the exit marker ring) +
// unitinfo.cpp - the constructor leaving on ORDER_STATE_UNIT_READY.

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/build"
	"github.com/suglasp/max-go/internal/game"
)

// BuildReady records a builder that has just finished a structure and is parked
// on the site waiting to drive off. The in game screen reads it to show the
// "Finished building X" prompt, the green "New X available" flash over the new
// building, and the animated exit markers.
type BuildReady struct {
	Product     assets.UnitType // the finished unit's type (for the prompt text)
	Building    *game.UnitInfo  // a constructor: the placed structure (hidden, then unfolds); a FACTORY: the produced unit parked on the plant, awaiting its drive out
	SiteX       int             // finished footprint origin
	SiteY       int
	Footprint   int  // 1 (engineer / 1x1) or 2 (constructor / 2x2 / factory)
	Factory     bool // true = a building's output: Building is the MOBILE unit that drives out (no unfold), keyed by the building itself (factory production OR depot unload)
	FromStorage bool // true = the unit was UNLOADED from a Depot/Hangar/Dock bay (an existing unit), not freshly produced - so no "New X available" flash / "Finished building" prompt
}

// cardinalRing lists the perimeter walk, for each direction, that traces the exit
// marker cells around a finished footprint: step the four cardinal directions
// (N, E, S, W) `limit` cells each, starting below and left of the footprint. It
// matches DrawMap_RedrawUnit's DIRECTION_OFFSETS[0,2,4,6] loop.
var cardinalRing = [4][2]int{{0, -1}, {1, 0}, {0, 1}, {-1, 0}}

// exitRingCells returns the perimeter ring around a build ready site, in the
// same order DrawMap_RedrawUnit walks it (so markers line up with vanilla). The
// cells are not filtered for accessibility here; ExitMarkers does that.
//
// C++ ported : drawmap.cpp - DrawMap_RedrawUnit
func exitRingCells(br *BuildReady) [][2]int {
	limit := 2
	startY := br.SiteY + 1
	if br.Footprint >= 2 { // a 2x2 building has a wider ring
		limit = 3
		startY++
	}
	px, py := br.SiteX-1, startY
	cells := make([][2]int, 0, 4*limit)
	for _, d := range cardinalRing {
		for j := 0; j < limit; j++ {
			px += d[0]
			py += d[1]
			cells = append(cells, [2]int{px, py})
		}
	}
	return cells
}

// ExitMarkers returns the cells a build ready builder may drive off to: the
// accessible cells of the perimeter ring around its finished footprint. The
// in game screen draws an animated BLDMRK marker on each, and a click on one
// calls ExitBuildSite. Returns nil when the builder is not build ready.
func (g *Game) ExitMarkers(builder *game.UnitInfo) [][2]int {
	br := g.buildReady[builder]
	if br == nil {
		return nil
	}
	// The unit that drives off is the builder (a constructor) or, for a factory, the
	// produced unit; mark only the cells THAT unit can enter (a factory can't move,
	// so checking the factory would yield no markers).
	mover := builder
	if br.Factory && br.Building != nil {
		mover = br.Building
	}
	var out [][2]int
	for _, c := range exitRingCells(br) {
		if g.CanEnter(mover, c[0], c[1]) {
			out = append(out, c)
		}
	}
	return out
}

// IsBuildReady reports whether a builder has finished a structure and is parked
// awaiting its exit. C++: ORDER_BUILD && ORDER_STATE_UNIT_READY (unitinfo.cpp).
func (g *Game) IsBuildReady(builder *game.UnitInfo) bool {
	return g.buildReady[builder] != nil
}

// BuildReadyInfo returns the build ready record for a builder, or nil. The
// in game screen uses it for the finished building prompt and the green
// "New X available" flash over the new structure.
func (g *Game) BuildReadyInfo(builder *game.UnitInfo) *BuildReady {
	return g.buildReady[builder]
}

// BuildReadyUnits returns the team's builders currently parked build ready (the
// in game screen flashes the green "New X available" text over each one's new
// structure). The order is unspecified.
func (g *Game) BuildReadyUnits(team int) []*game.UnitInfo {
	var out []*game.UnitInfo
	for b := range g.buildReady {
		if b.Team == team {
			out = append(out, b)
		}
	}
	return out
}

// ExitBuildSite drives a build ready builder off its finished footprint to
// (cx, cy), which must be one of its exit marker cells. It issues an animated
// move order and clears the build ready state. It reports whether the cell was a
// valid exit. C++: clicking a build marker issues the UNIT_READY exit move
// (game_manager.cpp / unitinfo.cpp).
func (g *Game) ExitBuildSite(builder *game.UnitInfo, cx, cy int) bool {
	if g.buildReady[builder] == nil {
		return false
	}
	valid := false
	for _, c := range g.ExitMarkers(builder) {
		if c[0] == cx && c[1] == cy {
			valid = true
			break
		}
	}
	if !valid {
		return false
	}
	br := g.buildReady[builder]
	// A FACTORY's output OR a depot/hangar/dock UNLOAD (vanilla M.A.X.): the produced/
	// stored unit MATERIALISES on the clicked marker and grows in from 0 -> full scale
	// (Unfold 4 -> 0, the same expand animation a finished building plays) - it does NOT
	// drive out. Place it on the marker, reveal it, and queue the step where it grows in;
	// the in game screen animates the scaling up and plays the expand sound. sweepFactoryReady frees the
	// building next Tick (the unit is now off the footprint) and starts the next queued unit.
	if br.Factory {
		product := br.Building
		if product == nil {
			g.clearBuildReady(builder)
			return true
		}
		g.Units.Move(product, cx, cy) // materialise on the marker tile
		product.Unfold = 4            // start collapsed; the screen grows it to full size
		product.HiddenUntilExpand = false
		g.expandReleased = append(g.expandReleased, product) // queue the grow in animation + expand sound
		return true
	}
	// A CONSTRUCTOR leaving its just finished structure: seat it again on the footprint cell
	// NEAREST the marker first (the hidden structure blocks a path the long way around),
	// then drive off - the structure unfolds once the builder clears the footprint.
	nx, ny := nearestFootprintCell(br, cx, cy)
	g.armExpand(builder) // the building expands once the builder clears its footprint
	g.clearBuildReady(builder)
	g.Units.Move(builder, nx, ny) // reseat on the near side before the drive off
	if !g.OrderMove(builder, cx, cy) {
		g.Units.Move(builder, cx, cy) // no drivable path: reposition without animating
	}
	return true
}

// nearestFootprintCell returns the build site footprint cell closest to (cx, cy):
// an exiting unit seats again here so it leaves from the side facing the clicked
// marker, instead of always from the top left corner (which made it drive the long
// way around the footprint to reach a marker on the far side).
func nearestFootprintCell(br *BuildReady, cx, cy int) (int, int) {
	clamp := func(v, lo, hi int) int {
		if v < lo {
			return lo
		}
		if v > hi {
			return hi
		}
		return v
	}
	return clamp(cx, br.SiteX, br.SiteX+br.Footprint-1), clamp(cy, br.SiteY, br.SiteY+br.Footprint-1)
}

/* // note: unused code in Go port - the storage bay Activate now uses klei's
// adjacent cell unload (UnloadSpot + UnloadUnit, repairshopmenu.cpp Activate),
// so the BLDMRK drive out for STORED units went dead. Factories keep their own
// output exit flow; kept for reference per the DEADCODE convention.
// EjectStoredUnit pulls a unit out of a storage building's bay (Depot/Hangar/Dock)
// and parks it build ready ON the building footprint, so the BLDMRK exit markers
// ring the building and the player (or AI) drives it out - the same output exit
// flow a factory uses (no unfold; the next ejection waits until this one leaves,
// via sweepFactoryReady). Returns false if the unit is not stored, or the building
// is already ejecting one. C++: the depot/hangar/dock "Activate" that deploys a
// stored unit back out (RepairShopMenu / units_manager.cpp).
func (g *Game) EjectStoredUnit(building, unit *game.UnitInfo) bool {
	if building == nil || unit == nil || g.buildReady[building] != nil {
		return false // one ejection at a time
	}
	idx := -1
	for i, c := range building.Carried {
		if c == unit {
			idx = i
			break
		}
	}
	if idx < 0 {
		return false // not stored here
	}
	building.Carried = append(building.Carried[:idx], building.Carried[idx+1:]...)
	unit.Parent = nil
	unit.HiddenUntilExpand = true // not drawn while inside: it materialises on the marker when it exits
	// The unit keeps its remaining move budget: stored units refresh at turn start
	// like any other (BeginTeamTurn reaches Carried), and the eject itself neither
	// grants nor costs speed. C++ ported : units_manager.cpp - ActivateUnit leaves
	// the client at its turn-refreshed speed.
	g.Units.Deploy(unit, building.GridX, building.GridY) // park on the footprint until it exits
	if g.buildReady == nil {
		g.buildReady = make(map[*game.UnitInfo]*BuildReady)
	}
	g.buildReady[building] = &BuildReady{
		Product:     unit.Type,
		Building:    unit,
		SiteX:       building.GridX,
		SiteY:       building.GridY,
		Footprint:   build.Footprint(building.Flags),
		Factory:     true, // reuse the factory output exit path (markers + drive out, no unfold)
		FromStorage: true, // an EXISTING stored unit, not new: suppress the "New X available" flash
	}
	// A computer team automatically drives the unit off at the start of its next turn.
	if g.State.Teams[building.Team].Type != game.TeamPlayer {
		if bx, by, ok := g.FactoryDeployCell(building, unit.Flags); ok {
			if g.buildExits == nil {
				g.buildExits = make(map[*game.UnitInfo][2]int)
			}
			g.buildExits[building] = [2]int{bx, by}
		}
	}
	g.RefreshVisibility()
	return true
}
*/

// sweepFactoryReady frees a factory once its parked output unit has left the plant
// footprint (driven out via a marker, auto exited by the AI, moved by the player,
// or destroyed): it drops the build ready record and starts the next queued unit.
// Called each frame from Tick. C++: the factory resumes production once the
// ready unit vacates (units_manager.cpp).
func (g *Game) sweepFactoryReady() {
	for factory, br := range g.buildReady {
		if !br.Factory {
			continue
		}
		p := br.Building
		fp := br.Footprint
		stillParked := p != nil && p.Hits > 0 &&
			p.GridX >= br.SiteX && p.GridX < br.SiteX+fp &&
			p.GridY >= br.SiteY && p.GridY < br.SiteY+fp
		if stillParked {
			continue
		}
		delete(g.buildReady, factory)
		delete(g.buildExits, factory)
		g.startFactoryHead(factory) // the plant is clear: build the next queued unit
	}
}

// armExpand records that builder's just finished building should grow in
// (unfold) once the builder has driven off the footprint. C++:
// UnitsManager_BuildingReady scales the building up (ORDER_STATE_EXPAND) only
// after the constructor leaves its bounds (units_manager.cpp).
func (g *Game) armExpand(builder *game.UnitInfo) {
	br := g.buildReady[builder]
	if br == nil || br.Building == nil {
		return
	}
	if g.pendingExpand == nil {
		g.pendingExpand = make(map[*game.UnitInfo]*game.UnitInfo)
	}
	g.pendingExpand[br.Building] = builder
}

// releaseParkedBuilding frees the finished building of a builder that is leaving the map
// while still parked on it (loaded into a transport): the building is revealed and
// unfolds at once, and the builder leaves its build pose. It must run while the builder
// is still in the cell hash, so the pose ends with a clean rebucket.
// C++ ported : units_manager.cpp - UnitsManager_BuildingReady (klei 23/08/26: a stored
// builder no longer blocks the site)
func (g *Game) releaseParkedBuilding(builder *game.UnitInfo) {
	br := g.buildReady[builder]
	if br == nil || br.Factory {
		return
	}
	if bld := br.Building; bld != nil && bld.Hits > 0 && bld.HiddenUntilExpand {
		bld.HiddenUntilExpand = false
		g.expandReleased = append(g.expandReleased, bld)
	}
	g.clearBuildReady(builder)
}

// updateExpansions releases a building to unfold once its builder has cleared the
// building's footprint (or the builder is gone). Called each frame from Tick.
func (g *Game) updateExpansions() {
	for bld, builder := range g.pendingExpand {
		fp := build.Footprint(bld.Flags)
		onSite := builder != nil && builder.Hits > 0 &&
			builder.GridX >= bld.GridX && builder.GridX < bld.GridX+fp &&
			builder.GridY >= bld.GridY && builder.GridY < bld.GridY+fp
		if !onSite {
			delete(g.pendingExpand, bld)
			bld.HiddenUntilExpand = false // the builder has cleared the site: reveal it and start the unfold
			g.expandReleased = append(g.expandReleased, bld)
		}
	}
}

// ReleasedExpansions returns (and clears) the buildings whose builder has just
// cleared the footprint since the last call; the in game screen plays each
// one's expand sound and animates its unfold (Unfold 4 -> 0).
func (g *Game) ReleasedExpansions() []*game.UnitInfo {
	if len(g.expandReleased) == 0 {
		return nil
	}
	out := g.expandReleased
	g.expandReleased = nil
	return out
}

// clearBuildReady drops a builder's build ready state and returns it to a
// resting idle order (the structure is finished; the builder is a free mobile
// unit again).
func (g *Game) clearBuildReady(builder *game.UnitInfo) {
	br := g.buildReady[builder]
	delete(g.buildReady, builder)
	delete(g.buildExits, builder)
	if br != nil && br.Factory {
		return // a factory was never in a build pose; nothing to reset on it
	}
	// Key on the pose, not the order: a popup order (Sentry) on the parked builder must not strand it in the build pose.
	if builder.Building {
		// Rebucket the builder around EndBuildPose: remove its complete construction footprint, clear the pose to restore its 1x1 footprint, then add it again.
		// Reversing this order leaves stale entries in the completed site's other cells. C++ releases every reserved footprint cell when a UNIT_READY builder exits.
		rebucket := g.Units != nil && g.Units.Cells != nil
		if rebucket {
			g.Units.Cells.Remove(builder) // drop the whole build site footprint
		}
		builder.EndBuildPose()
		builder.State = game.OrderStateExecutingOrder
		// The parked sprite is centered on the construction footprint while the hash uses its corner cell. Reset the pixel position before exit movement to avoid a snap across the site.
		builder.SetPosition(builder.GridX, builder.GridY)
		if rebucket {
			g.Units.Cells.Add(builder) // add again at the single resting cell
		}
	}
}
