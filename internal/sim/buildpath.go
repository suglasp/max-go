// buildpath.go : lets engineers build a line of structures along one route.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : paths.cpp - path build chain along a connector line
package sim

import (
	"github.com/suglasp/max-go/internal/access"
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/build"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
)

// Engineer PATH build - lay a straight run of the same structure (a line of roads,
// a span of bridge, a row of turrets) along one axis, one cell per turn. The player
// arms it from the Construction Menu's Path button, then clicks a destination; the
// engineer builds its own cell first, then drives one cell along the line and builds
// the next, until the run is done, the material runs out, or the player cancels.
//
// The continuation rides on the existing build machinery: a finished FLAT special
// (road / bridge / platform) frees the engineer in place at once (see finishBuild's
// flatSpecial branch), so the chain flows with no exit; a finished real BUILDING
// parks it for the drive off exit and the chain resumes once it has left. Either
// way tickBuildPaths only steps a builder that is FREE.
//
// C++ ported : game_manager.cpp - GameManager_PathBuild (the linear GroundPath from
// the engineer to the picked cell) + units_manager.cpp - UnitsManager_BuildNext (the
// build_list[0] building again at each step, stopping when storage runs out or the path ends).

// buildPath is one engineer's queued line build: the structure type/flags/values to
// lay, the build rate, the straight run of cells (anchor first), and Idx, the next
// cell to start. Cells before Idx are built or skipped.
type buildPath struct {
	Type  game.UnitInfo // a template product (Type/Flags/Values) cloned per cell
	Rate  int
	Cells [][2]int
	Idx   int
}

// PathCells returns the straight, single axis run of cells from (sx,sy) to (dx,dy),
// both ends inclusive. The longer delta picks the axis, so the run is always a true
// horizontal or vertical line (never diagonal), matching the engineer Path build.
// C++ ported : GroundPath::AppendLinearSteps clamped to one axis.
func PathCells(sx, sy, dx, dy int) [][2]int {
	adx, ady := abs(dx-sx), abs(dy-sy)
	// Vertical run when the up/down delta dominates, else horizontal.
	if ady > adx {
		cells := make([][2]int, 0, ady+1)
		step := sign(dy - sy)
		for y := sy; ; y += step {
			cells = append(cells, [2]int{sx, y})
			if y == dy {
				break
			}
		}
		return cells
	}
	cells := make([][2]int, 0, adx+1)
	step := sign(dx - sx)
	for x := sx; ; x += step {
		cells = append(cells, [2]int{x, sy})
		if x == dx {
			break
		}
	}
	return cells
}

// abs / sign are the small integer helpers PathCells needs.
func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func sign(v int) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	default:
		return 0
	}
}

// StartBuildPath arms a Path build: builder lays template (its Type/Flags/Values), at
// the given rate, on each valid cell of the straight run from its own cell to (dx,dy),
// building them in order. The anchor (the engineer's own cell) starts at once. It
// reports false - so the caller falls back to an ordinary single build - when the run
// collapses to a single cell or has fewer than two site valid cells.
//
// C++ ported : GameManager_PathBuild (builds the engineer's cell, lays the linear path).
func (g *Game) StartBuildPath(builder, template *game.UnitInfo, rate, dx, dy int) bool {
	if builder == nil || template == nil {
		return false
	}
	line := PathCells(builder.GridX, builder.GridY, dx, dy)
	// Keep only the cells the structure actually fits on (the builder ignored, since
	// it stands inside its own footprint). A single valid cell is not a "path".
	desc := access.UnitDesc{
		Type:     template.Type,
		Flags:    template.Flags,
		LandType: access.LandTypeForUnit(template.Type, template.Flags),
	}
	g.Units.Cells.Remove(builder)
	valid := line[:0:0]
	for _, c := range line {
		// Ignore mobile units when filtering: a unit merely parked (or paused mid move,
		// so its Order is OrderAwait, not OrderIdle) on a path cell is a transient
		// obstacle, not a reason to drop the cell for good. Keeping it in the run lets
		// tickBuildPaths wait for the unit to leave, then build the spot it stood on,
		// instead of skipping the tile a unit once sat on. Terrain and permanent
		// structures still decide validity.
		if build.SiteValidMod(g.Grid, desc, builder.Team, c[0], c[1], access.SameClassBlocks|access.IgnoreMobileUnits) {
			valid = append(valid, c)
		}
	}
	g.Units.Cells.Add(builder)
	if len(valid) < 2 {
		return false
	}
	if rate < 1 {
		rate = 1
	}
	if g.buildPaths == nil {
		g.buildPaths = make(map[*game.UnitInfo]*buildPath)
	}
	bp := &buildPath{
		Type:  *template, // shallow copy: Type/Flags/Values are all the per cell clone needs
		Rate:  rate,
		Cells: valid,
		Idx:   0,
	}
	g.buildPaths[builder] = bp
	// Start the first cell straight away ONLY when the builder actually stands on it (a
	// road/turret/platform run that begins at the builder's own cell). When the first
	// buildable cell is elsewhere - a BRIDGE run that must begin at the shoreline while
	// the engineer is inland (its own LAND cells are not valid for a bridge and were filtered
	// out above) - drive the engineer there first; tickBuildPaths starts the build once
	// it arrives. Building it in place anchored the order on a cell the engineer never
	// reached: builderAtSite never latched Started, so the order built "forever" while
	// the engineer's build pose jumped to the remote site (the inland teleport bug).
	// C++ ported : GameManager_PathBuild walks the engineer to the first buildable cell.
	if builder.GridX == bp.Cells[0][0] && builder.GridY == bp.Cells[0][1] {
		g.startPathCell(builder, bp)
	} else {
		g.orderMoveTo(builder, bp.Cells[0][0], bp.Cells[0][1])
	}
	return true
}

// startPathCell starts the build order for the path's current cell (bp.Cells[bp.Idx]),
// cloning a fresh product from the template. It returns false when the cell is no longer
// site valid, or already holds the structure being laid (then the caller skips it).
func (g *Game) startPathCell(builder *game.UnitInfo, bp *buildPath) bool {
	cx, cy := bp.Cells[bp.Idx][0], bp.Cells[bp.Idx][1]
	// The structure already stands on this cell - the crossing of an existing road or
	// connector run. Never lay a duplicate: report the cell not buildable so the caller
	// skips it. A road is ground cover, so IsAccessible does not treat an existing one as
	// a same type obstacle, which let a crossing path build a second road on the tile.
	if g.cellHasStructureType(cx, cy, bp.Type.Type) {
		return false
	}
	desc := access.UnitDesc{
		Type:     bp.Type.Type,
		Flags:    bp.Type.Flags,
		LandType: access.LandTypeForUnit(bp.Type.Type, bp.Type.Flags),
	}
	g.Units.Cells.Remove(builder)
	ok := build.SiteValid(g.Grid, desc, builder.Team, cx, cy)
	g.Units.Cells.Add(builder)
	if !ok {
		return false
	}
	p := game.NewUnitInfo(bp.Type.Type, builder.Team, 0, 0)
	p.Flags = bp.Type.Flags
	p.Values = bp.Type.Values
	p.InitState()
	g.StartBuild(builder, p, cx, cy, bp.Rate)
	return true
}

// tickBuildPaths advances each of team's Path builds by one cell when its engineer is
// free: it starts the next cell when the engineer stands on it, else drives the engineer
// there. A builder still building, parked awaiting its exit, or in transit is left alone.
// A run whose remaining cells are all gone (or whose next cell is unreachable) is dropped.
// Called once per AdvanceBuilds, after the turn's finished cells have advanced Idx.
//
// C++ ported : UnitsManager_BuildNext (move one GroundPath step, build the next cell).
func (g *Game) tickBuildPaths(team int) {
	for b, bp := range g.buildPaths {
		if b.Team != team {
			continue
		}
		if b.Hits <= 0 {
			delete(g.buildPaths, b) // destroyed mid run
			continue
		}
		if g.BuildOrderFor(b) != nil {
			continue // still building the current cell
		}
		if g.isMoving(b) {
			continue // still driving to the next cell
		}
		if g.IsBuildReady(b) {
			// A finished real BUILDING (not flat) parked the builder for its drive off
			// exit. In a PATH run there is no manual exit between cells: if more cells
			// remain, release it automatically (the building unfolds as the engineer leaves) and
			// drive on; only the LAST cell's building keeps the normal parked exit.
			//
			// C++ ported : units_manager.cpp - ORDER_STATE_UNIT_READY runs
			// UnitsManager_BuildNext when unit->path is set (continues automatically), and only
			// Animates (waits for the player to move it off) when there is no path.
			if bp.Idx >= len(bp.Cells) {
				delete(g.buildPaths, b) // last cell laid: the normal single build exit takes over
				continue
			}
			// The storage cannot pay for the next cell: end the run here. The builder stays
			// parked on the finished building with its exit markers, as a single build does.
			if !pathCellAffordable(b, bp) {
				g.stopShortPath(b)
				continue
			}
			g.armExpand(b)       // the building unfolds once the engineer clears its footprint
			g.clearBuildReady(b) // free the engineer from the parked build ready state
		}
		// A free builder (after a road, connector, bridge or platform cell) that cannot pay
		// for the next cell ends the run where it stands.
		if bp.Idx < len(bp.Cells) && !pathCellAffordable(b, bp) {
			g.stopShortPath(b)
			continue
		}
		// Advance to the next cell to build, skipping leading cells that are already
		// laid or genuinely unbuildable, but WAITING on one a unit is standing on.
		for bp.Idx < len(bp.Cells) {
			cx, cy := bp.Cells[bp.Idx][0], bp.Cells[bp.Idx][1]
			// The structure already stands here (the crossing of an existing run): skip
			// it, never lay a duplicate.
			if g.cellHasStructureType(cx, cy, bp.Type.Type) {
				bp.Idx++
				continue
			}
			if b.GridX == cx && b.GridY == cy {
				if g.startPathCell(b, bp) {
					break // building this cell now
				}
				bp.Idx++ // cell no longer buildable (terrain): skip to the next
				continue
			}
			// A unit is standing on the next path cell. Stop and WAIT in place instead of
			// skipping it: the engineer holds here, and once the unit drives away it moves
			// on and builds the spot the unit was standing on (rather than leaving a gap).
			if g.cellBlockedByMobileUnit(b, cx, cy) {
				break
			}
			// Not on the cell yet: drive to it (one cell per turn). When the builder
			// just finished a real BUILDING it stands on that now impassable cell, so a
			// normal path off it can fail; fall back to a direct reposition onto the next
			// cell, exactly as ExitBuildSite does for the manual drive off.
			if !g.orderMoveTo(b, cx, cy) { // orderMoveTo, NOT OrderMove: keep the path alive
				if g.CanEnter(b, cx, cy) {
					g.Units.Move(b, cx, cy) // step off the blocked site onto the next cell
					break
				}
				bp.Idx++ // genuinely unreachable terrain; skip it
				continue
			}
			break
		}
		if bp.Idx >= len(bp.Cells) {
			delete(g.buildPaths, b) // the whole run is laid (or skipped)
		}
	}
}

// resumePathBuildOnArrival starts the current path cell's build the moment a path build
// engineer settles ON that cell mid turn, so the lay happens the same turn it arrives
// (matching vanilla's moving and building in one turn) instead of waiting for the next
// turn's tickBuildPaths. Does nothing when the builder is dead, already building, still in transit,
// the run is finished, or it settled on some other cell.
//
// C++ ported : units_manager.cpp - UnitsManager_BuildNext builds the next cell on the
// GroundPath step the engineer reaches it.
func (g *Game) resumePathBuildOnArrival(b *game.UnitInfo) {
	bp := g.buildPaths[b]
	if bp == nil || bp.Idx >= len(bp.Cells) {
		return
	}
	if b.Hits <= 0 || g.BuildOrderFor(b) != nil || g.isMoving(b) {
		return
	}
	cx, cy := bp.Cells[bp.Idx][0], bp.Cells[bp.Idx][1]
	if b.GridX != cx || b.GridY != cy {
		return // settled elsewhere; tickBuildPaths drives again to the cell next turn
	}
	if !g.startPathCell(b, bp) {
		bp.Idx++ // the cell is no longer site valid: skip it (next turn drives to the following one)
	}
}

// abandonStalledPaths drops the rest of the Path run of any builder that stalled this
// turn for lack of material. The started cell is kept and waits for material like a
// single build (the caller's "needs material" message tells the player). The check
// before each cell (pathCellAffordable) makes this a safety net only.
// C++ ported : unitinfo.cpp - UnitInfo::UpdateProduction (a started build is never cancelled).
func (g *Game) abandonStalledPaths(stalled []*game.UnitInfo) {
	for _, b := range stalled {
		delete(g.buildPaths, b)
	}
}

// pathCellAffordable reports whether builder's storage pays for one whole cell of
// the run at rate 1, the same rule the Construction Menu applies to a first build.
// C++ ported : units_manager.cpp - UnitsManager_BuildNext
func pathCellAffordable(builder *game.UnitInfo, bp *buildPath) bool {
	turns := 1
	if bp.Type.Values != nil {
		turns = max(bp.Type.Values.Get(game.AttribTurns), 1)
	}
	return builder.Storage >= economy.RawConsumptionRate(builder.Type, 1)*turns
}

// stopShortPath ends builder's run because it cannot pay for the next cell, and
// records it so the in game screen shows the insufficient material warning.
// C++ ported : unitinfo.cpp - UnitInfo::ClearBuildListAndPath
func (g *Game) stopShortPath(builder *game.UnitInfo) {
	delete(g.buildPaths, builder)
	g.pathShort = append(g.pathShort, builder)
}

// CancelBuildPath drops builder's remaining Path run (the player pressed ESC or
// right clicked). A cell already under construction is left to finish; only the cells
// not yet started are cleared. C++: the ESC or right click cancel of ORDER_STATE_SELECT_SITE.
func (g *Game) CancelBuildPath(builder *game.UnitInfo) {
	delete(g.buildPaths, builder)
}

// BuildPathActive reports whether builder is running a Path build (the in game screen
// keeps it selected and draws its remaining run).
func (g *Game) BuildPathActive(builder *game.UnitInfo) bool {
	return g.buildPaths[builder] != nil
}

// cellHasStructureType reports whether (cx,cy) already holds a unit of type t. A Path
// build reads it to skip a cell whose structure is already there - the crossing of an
// existing road or connector run - so it never lays a duplicate on that tile.
func (g *Game) cellHasStructureType(cx, cy int, t assets.UnitType) bool {
	for _, u := range g.Units.Cells.At(cx, cy) {
		if u.Type == t {
			return true
		}
	}
	return false
}

// cellBlockedByMobileUnit reports whether a movable unit other than builder is standing
// on (cx,cy) right now. A Path build treats such a cell as only temporarily blocked: the
// engineer waits in place for the unit to leave, then builds the spot, rather than
// skipping the tile for good (which left a permanent gap where a unit once stood).
func (g *Game) cellBlockedByMobileUnit(builder *game.UnitInfo, cx, cy int) bool {
	for _, u := range g.Units.Cells.At(cx, cy) {
		if u == builder {
			continue
		}
		if u.Flags.Has(game.FlagMobileLandUnit) || u.Flags.Has(game.FlagMobileSeaUnit) || u.Flags.Has(game.FlagMobileAirUnit) {
			return true
		}
	}
	return false
}

// isMoving reports whether u has an active or queued (over budget) move order.
func (g *Game) isMoving(u *game.UnitInfo) bool {
	if _, queued := g.pending[u]; queued {
		return true
	}
	for _, o := range g.orders {
		if o.Unit == u {
			return true
		}
	}
	return false
}
