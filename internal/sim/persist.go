// persist.go : serializes and restores live simulation state in MAXGOSV format.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
package sim

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/game"
)

// Save/restore seams. The saveload package uses these read only accessors +
// restore mutators to capture and restore sim private in flight state that
// the core Snapshot does not cover (engineer line builds, the AI spotted memory).
// The live game logic never calls them; they exist only for the save image.

// BuildPathState is a snapshot of one engineer's running line (Path) build: the
// builder, the template structure being laid (only Type/Flags/Values are needed -
// see buildPath), the build rate, the run of cells and the next cell index.
type BuildPathState struct {
	Builder *game.UnitInfo
	Product assets.UnitType
	Flags   game.Flags
	Values  *game.UnitValues
	Rate    int
	Cells   [][2]int
	Idx     int
}

// BuildPathStates returns every running line build, for the save image. The order
// is unspecified.
func (g *Game) BuildPathStates() []BuildPathState {
	if len(g.buildPaths) == 0 {
		return nil
	}
	out := make([]BuildPathState, 0, len(g.buildPaths))
	for b, bp := range g.buildPaths {
		out = append(out, BuildPathState{
			Builder: b,
			Product: bp.Type.Type,
			Flags:   bp.Type.Flags,
			Values:  bp.Type.Values,
			Rate:    bp.Rate,
			Cells:   bp.Cells,
			Idx:     bp.Idx,
		})
	}
	return out
}

// RestoreBuildPath restores builder's running line build from a save: the
// template structure, the run of cells and the next index. The current cell's
// build order is restored separately (StartBuild); tickBuildPaths then resumes the
// remainder of the run on the following turns.
func (g *Game) RestoreBuildPath(builder *game.UnitInfo, product assets.UnitType, flags game.Flags, values *game.UnitValues, rate int, cells [][2]int, idx int) {
	if builder == nil || len(cells) == 0 {
		return
	}
	if g.buildPaths == nil {
		g.buildPaths = make(map[*game.UnitInfo]*buildPath)
	}
	bp := &buildPath{Rate: rate, Cells: cells, Idx: idx}
	bp.Type.Type = product
	bp.Type.Flags = flags
	bp.Type.Values = values
	g.buildPaths[builder] = bp
}

// SpottedState is one team's memory of one spotted enemy, for the save image.
type SpottedState struct {
	Team    int
	Enemy   *game.UnitInfo
	LastX   int
	LastY   int
	Visible bool
}

// SpottedStates returns every team's spotted enemy memory, for the save image.
func (g *Game) SpottedStates() []SpottedState {
	var out []SpottedState
	for t := range g.spotted.teams {
		for e, su := range g.spotted.teams[t] {
			out = append(out, SpottedState{Team: t, Enemy: e, LastX: su.LastX, LastY: su.LastY, Visible: su.Visible})
		}
	}
	return out
}

// AddSpotted restores one team's memory of an enemy from a save: the (frozen)
// last seen cell + visibility. Called after Restore so the AI keeps pursuing an
// enemy it had lost sight of, instead of forgetting every fogged contact on load
// (Restore's RefreshVisibility derives again only the CURRENTLY visible enemies).
func (g *Game) AddSpotted(team int, enemy *game.UnitInfo, lastX, lastY int, visible bool) {
	if enemy == nil || team < 0 || team >= game.TeamCount {
		return
	}
	g.spotted.ensure()
	g.spotted.teams[team][enemy] = &SpottedUnit{Unit: enemy, Team: team, LastX: lastX, LastY: lastY, Visible: visible}
}

// NextBuilt / SetNextBuilt expose the built unit ID counter so a save preserves it
// and a load keeps issuing IDs that never collide: built units use 10000+, slabs 10000+,
// mines 20000+ and clear markers 30000+, all drawn from this single counter. Without
// it Restore would reset to 0 and the next built unit could collide with a saved one.
func (g *Game) NextBuilt() int {
	return g.nextBuilt
}
func (g *Game) SetNextBuilt(n int) {
	g.nextBuilt = n
}

// RestoreRubble restores the debris piles from a save. Restore otherwise
// starts on a clean field, so every pile (and the ability to clear it) would vanish.
func (g *Game) RestoreRubble(cells []RubbleCell) {
	if len(cells) == 0 {
		return
	}
	if g.rubble == nil {
		g.rubble = make(map[[2]int]rubbleSpot)
	}
	for _, c := range cells {
		g.rubble[[2]int{c.X, c.Y}] = rubbleSpot{large: c.Large, material: c.Material}
	}
}

// ClearJobState is a snapshot of a bulldozer's running clear job: the dozer, the
// site (anchor cell + footprint), the turns of work left, and the hazard cone/tape
// markers laid over it (real on map units, linked again by ID on load).
type ClearJobState struct {
	Bulldozer *game.UnitInfo
	CellX     int
	CellY     int
	Footprint int
	TurnsLeft int
	Markers   []*game.UnitInfo
}

// ClearJobStates returns every running bulldozer clear job, for the save image.
func (g *Game) ClearJobStates() []ClearJobState {
	if len(g.clearing) == 0 {
		return nil
	}
	out := make([]ClearJobState, 0, len(g.clearing))
	for u, j := range g.clearing {
		out = append(out, ClearJobState{
			Bulldozer: u, CellX: j.cellX, CellY: j.cellY,
			Footprint: j.footprint, TurnsLeft: j.turnsLeft, Markers: j.markers,
		})
	}
	return out
}

// ClearJobTurnsLeft reports the turns of work remaining for a bulldozer's running
// clear job, and whether one exists. Used for the "Number of turns to clear site: N"
// status message when a bulldozer is selected. C++: unit->build_time for ORDER_CLEAR.
func (g *Game) ClearJobTurnsLeft(bulldozer *game.UnitInfo) (int, bool) {
	if j, ok := g.clearing[bulldozer]; ok {
		return j.turnsLeft, true
	}
	return 0, false
}

// RestoreClearJob restores a bulldozer's clear job from a save (the dozer +
// its markers stay on the map and are restored as normal units; this links them again).
func (g *Game) RestoreClearJob(bulldozer *game.UnitInfo, cellX, cellY, footprint, turnsLeft int, markers []*game.UnitInfo) {
	if bulldozer == nil {
		return
	}
	if g.clearing == nil {
		g.clearing = make(map[*game.UnitInfo]*clearJob)
	}
	g.clearing[bulldozer] = &clearJob{cellX: cellX, cellY: cellY, footprint: footprint, turnsLeft: turnsLeft, markers: markers}
	// Re enter the clearing POSE, the way StartClearRubble does. The job itself always
	// finished correctly, but a reloaded dozer sat there in its plain driving sprite,
	// with no working animation and no sound, until the last turn of work ran out.
	bulldozer.StartClearPose(cellX, cellY, footprint)
}

// BuildReadyState is a snapshot of a builder parked after finishing a structure,
// awaiting its drive off exit. The builder is OFF the unit registry (finishBuild
// called g.Units.Remove on it), so g.Units.Units() misses it - the save image must
// carry the builder itself, not just the record, or the parked constructor is lost.
type BuildReadyState struct {
	Builder   *game.UnitInfo
	Product   assets.UnitType
	Building  *game.UnitInfo
	SiteX     int
	SiteY     int
	Footprint int
}

// BuildReadyStates returns every parked build ready builder, for the save image.
func (g *Game) BuildReadyStates() []BuildReadyState {
	if len(g.buildReady) == 0 {
		return nil
	}
	out := make([]BuildReadyState, 0, len(g.buildReady))
	for b, br := range g.buildReady {
		out = append(out, BuildReadyState{
			Builder: b, Product: br.Product, Building: br.Building,
			SiteX: br.SiteX, SiteY: br.SiteY, Footprint: br.Footprint,
		})
	}
	return out
}

// RestoreBuildReady restores a parked build ready builder from a save and
// records the finished structure it is waiting to vacate. The builder MUST be the
// instance already rebuilt in the unit registry (the live parked builder stays in
// Cells+ByID, so the save carries it in the main unit list too) - keying off that
// same pointer is what lets a click read it as build ready. A builder that was
// saved off the registry (no matching unit) is added here instead.
//
// A mobile constructor/engineer is itself parked in the build pose on its finished
// footprint and is put back into the FINISHED ORDER_STATE_UNIT_READY state (not the
// BUILD_IN_PROGRESS that StartBuildPose leaves) so its BLDMRK exit flow engages. A
// FACTORY/depot output (a stationary building as the key) never enters a build pose;
// only its build ready record is restored, with Factory set so sweepFactoryReady
// frees the plant once the produced unit drives off.
func (g *Game) RestoreBuildReady(builder, building *game.UnitInfo, product assets.UnitType, siteX, siteY, footprint int) {
	if builder == nil {
		return
	}
	if g.buildReady == nil {
		g.buildReady = make(map[*game.UnitInfo]*BuildReady)
	}
	factory := builder.Flags.Has(game.FlagStationary)
	if !factory {
		// Enter the parked build pose again. The builder rebuilt on the map was deployed at
		// its 1x1 corner (the unit snapshot carries no build footprint), so bucket the
		// cell hash again: drop it, set the pose, then add across the whole reserved site,
		// mirroring finishBuild's parked builder indexing.
		g.Units.Cells.Remove(builder)
		builder.StartBuildPose(siteX, siteY, footprint)
		// A finished builder awaits its drive-off as UNIT_READY, enabling clearBuildReady and the structure's unfold.
		builder.Order = game.OrderBuild
		builder.State = game.OrderStateUnitReady
		// It must be in BOTH hashes so the player can select it and see its BLDMRK exit
		// markers (a builder saved off the registry would be invisible to a click). Add
		// is idempotent for one already registered.
		g.Units.Cells.Add(builder)
		g.Units.ByID.Add(builder)
	}
	g.buildReady[builder] = &BuildReady{
		Product: product, Building: building,
		SiteX: siteX, SiteY: siteY, Footprint: footprint,
		Factory: factory,
	}
}
