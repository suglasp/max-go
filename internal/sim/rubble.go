// rubble.go : places ground-cover rubble when land units and buildings are destroyed.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// C++ ported : unitinfo.cpp - rubble placement on unit destruction
package sim

import (
	"github.com/suglasp/max-go/internal/assets"
	"github.com/suglasp/max-go/internal/build"
	"github.com/suglasp/max-go/internal/economy"
	"github.com/suglasp/max-go/internal/game"
	"github.com/suglasp/max-go/internal/world"
)

// Rubble records debris left by destroyed units: LRGRUBLE for a building and SMLRUBLE otherwise. A Bulldozer clears a pile over one turn for a
// small pile or four turns for a building pile, reclaiming the ground and its salvaged material.
//
// C++ ported : units_manager.cpp - UnitsManager_ProcessOrderClear / StartClearing
// + the StopRemove popup (clearing_time 1/4).

// clearSmallTurns / clearLargeTurns are the original's clearing_time: 1 turn for a small
// (mobile unit) pile, 4 for a large (building) pile.
const (
	clearSmallTurns = 1
	clearLargeTurns = 4
)

// rubbleSpot is the debris on one cell. large = a building's 2x2 pile (LRGRUBLE),
// else a 1x1 mobile unit pile (SMLRUBLE). material is the raw a bulldozer reclaims
// when it clears the pile (the original rubble_unit->storage).
type rubbleSpot struct {
	large    bool
	material int
}

// rubbleSalvage is the raw material a destroyed unit leaves in its debris pile:
// half its build cost, plus any RAW cargo it was still carrying (a storage unit /
// mining station / loaded constructor). A bulldozer reclaims this on clear.
// C++ ported : units_manager.cpp UnitsManager_BuildClearing - cargo_amount =
// GetNormalRateBuildCost()/2 (+ unit->storage when the cargo type is RAW).
func rubbleSalvage(u *game.UnitInfo) int {
	amount := build.Cost(u) / 2
	if economy.CargoTypeOf(u.Type) == economy.CargoRaw {
		amount += u.Storage
	}
	return amount
}

// clearJob tracks a Bulldozer clearing a pile: the cell being cleared, the turns
// of work still left, and the hazard cones+tape markers laid over the site (real
// stationary units, so they render and block movement; removed when the job ends).
type clearJob struct {
	cellX, cellY int
	footprint    int // 1 (small pile) or 2 (building pile) - the exit ring size
	turnsLeft    int
	markers      []*game.UnitInfo
}

// RubbleCell is one debris pile for the renderer and the save snapshot: its anchor
// cell, whether it is a large (building) pile, and the raw a bulldozer reclaims on
// clear (Material).
type RubbleCell struct {
	X, Y     int
	Large    bool
	Material int
}

// recordRubble leaves debris where a unit was just destroyed (large for a
// building, small otherwise). Called from DestroyUnit after the slab has been
// stripped. Vanilla leaves a pile only on solid LAND and only for "wreckable"
// units, so several cases leave nothing:
//   - ground cover (slabs / road / tape / cones) and mines;
//   - soldiers (commando / infantry) - they leave no wreck;
//   - a flying air unit (HOVERING) and a connector;
//   - a unit destroyed on WATER or COAST (a sea unit or a building touching water), which sinks without rubble.
//
// Both the original and MaxR require land terrain for rubble.
//
// C++ ported : units_manager.cpp - UnitsManager_BuildClearing (the rubble_type
// gate: COMMANDO/INFANTRY/CONNECTOR/HOVERING excluded + Access_IsFullyLandCovered).
//
// site is the 2x2 build site a Constructor was working on when it died (nil
// otherwise): its pile is the site's large one at the site corner, or none when
// the site is not all land.
// C++ ported : units_manager.cpp - UnitsManager_BuildClearing (CONSTRCT + ORDER_BUILD
// moves SMLRUBLE onto the construction utility as LRGRUBLE).
func (g *Game) recordRubble(u *game.UnitInfo, site *game.Point) {
	if u.Flags.Has(game.FlagGroundCover) {
		return
	}
	if u.Type == assets.LandMine || u.Type == assets.SeaMine {
		return
	}
	if u.Type == assets.Commando || u.Type == assets.Infantry {
		return
	}
	if u.Flags.Has(game.FlagHovering) || u.Flags.Has(game.FlagConnectorUnit) {
		return
	}
	if !g.isFullyLandCovered(u) {
		return // destroyed on water/coast: it sinks, no debris
	}

	x, y, large := u.GridX, u.GridY, u.Flags.Has(game.FlagBuilding)

	// A Constructor at work leaves the site's large pile instead of its own small one.
	if site != nil && !large {
		x, y, large = site.X, site.Y, true
		if !g.isLandAt(x, y, 2) {
			return
		}
	}

	if g.rubble == nil {
		g.rubble = make(map[[2]int]rubbleSpot)
	}
	g.rubble[[2]int{x, y}] = rubbleSpot{
		large:    large,
		material: rubbleSalvage(u),
	}
}

// constructionSite returns the 2x2 site a Constructor is raising a building on: it
// stands in the build pose on the site of its order. A builder parked on its
// finished building has no site any more (the building is up, the tape is gone),
// and Engineers build 1x1 on their own cell, so neither has one.
// C++ ported : access.cpp - Access_GetConstructionUtility (the site's tape / cones).
func (g *Game) constructionSite(u *game.UnitInfo) *game.Point {
	if u.Type != assets.Constrct || !u.Building || g.buildReady[u] != nil {
		return nil
	}
	for _, o := range g.builds {
		if o.Builder == u && o.Product != nil && o.Product.Flags.Has(game.FlagBuilding) {
			return &game.Point{X: o.GridX, Y: o.GridY}
		}
	}
	return nil
}

// isFullyLandCovered reports whether every cell of u's footprint is land terrain:
// the whole 2x2 for a building, the single cell otherwise. A building straddling
// water (or any sea unit on water) is not fully land covered, so it leaves no
// rubble - the original is all or nothing here (it does not split a pile onto the land
// cells the way MaxR does). C++ ported : access.cpp - Access_IsFullyLandCovered.
func (g *Game) isFullyLandCovered(u *game.UnitInfo) bool {
	n := 1
	if u.Flags.Has(game.FlagBuilding) {
		n = 2
	}
	return g.isLandAt(u.GridX, u.GridY, n)
}

// isLandAt reports whether the n x n block with its corner at (cornerX, cornerY) is
// all land terrain inside the map.
func (g *Game) isLandAt(cornerX, cornerY, n int) bool {
	for dy := 0; dy < n; dy++ {
		for dx := 0; dx < n; dx++ {
			x, y := cornerX+dx, cornerY+dy
			if x < 0 || y < 0 || x >= g.W || y >= g.H || g.Grid.Surf == nil {
				return false
			}
			if g.Grid.Surf.SurfaceAt(x, y) != world.SurfaceLand {
				return false
			}
		}
	}
	return true
}

// RubbleAt reports the debris on (x, y): large is true for a building pile, ok is
// false when the cell has no rubble.
func (g *Game) RubbleAt(x, y int) (large, ok bool) {
	r, ok := g.rubble[[2]int{x, y}]
	return r.large, ok
}

// RubbleCells returns every debris pile in the game, for the in game renderer.
func (g *Game) RubbleCells() []RubbleCell {
	if len(g.rubble) == 0 {
		return nil
	}
	out := make([]RubbleCell, 0, len(g.rubble))
	for c, r := range g.rubble {
		out = append(out, RubbleCell{X: c[0], Y: c[1], Large: r.large, Material: r.material})
	}
	return out
}

// IsBulldozer reports whether u is a Bulldozer (the rubble clearing unit).
func IsBulldozer(u *game.UnitInfo) bool {
	return u != nil && u.Type == assets.Bulldozr
}

// rubbleCoverAnchor finds the pile that covers cell (x, y): the pile's anchor
// cell, whether it is large (a 2x2 building pile), and whether one was found. A
// small pile covers only its own cell; a large pile at anchor (ax, ay) covers the
// 2x2 (ax..ax+1, ay..ay+1), so a bulldozer on ANY of those four cells can clear
// it. The exact cell is preferred, then the three other 2x2 anchors.
func (g *Game) rubbleCoverAnchor(x, y int) (ax, ay int, large, ok bool) {
	if lg, found := g.RubbleAt(x, y); found {
		return x, y, lg, true
	}
	// A large pile whose top left is up/left of (x,y) still covers it.
	for _, a := range [3][2]int{{x - 1, y}, {x, y - 1}, {x - 1, y - 1}} {
		if lg, found := g.RubbleAt(a[0], a[1]); found && lg {
			return a[0], a[1], true, true
		}
	}
	return 0, 0, false, false
}

// clearFootprintRubble removes any debris pile overlapping the just built product's
// footprint (1x1 for a small structure, the 2x2 for a FlagBuilding), matching the original's
// build completion auto clear of underlying rubble. A CONNECTOR (Cnct4W) is the one
// exception - the original sets skip_rubble_removal for an engineer laying a connector - so a
// laid connector leaves the pile beneath it. No salvage is paid (the bulldozer's job);
// this is a silent clear. A large pile is removed whole even if only one cell overlaps
// (it is one ground cover record), via rubbleCoverAnchor.
//
// C++ ported : units_manager.cpp - UnitsManager_BuildClearing -> Access_DestroyUtilities.
func (g *Game) clearFootprintRubble(p *game.UnitInfo, gx, gy int) {
	if p == nil || p.Type == assets.Cnct4W || len(g.rubble) == 0 {
		return
	}
	n := 1
	if p.Flags.Has(game.FlagBuilding) {
		n = 2
	}
	for dy := 0; dy < n; dy++ {
		for dx := 0; dx < n; dx++ {
			if ax, ay, _, ok := g.rubbleCoverAnchor(gx+dx, gy+dy); ok {
				delete(g.rubble, [2]int{ax, ay})
			}
		}
	}
}

// CanClearRubble reports whether the bulldozer can start clearing right now: it is
// a Bulldozer standing on (or on the footprint of) a rubble pile and is not
// already clearing.
func (g *Game) CanClearRubble(u *game.UnitInfo) bool {
	if !IsBulldozer(u) || g.IsClearingRubble(u) {
		return false
	}
	_, _, _, ok := g.rubbleCoverAnchor(u.GridX, u.GridY)
	return ok
}

// StartClearRubble puts the bulldozer to work clearing the pile it stands on. It
// snaps the bulldozer onto the pile's anchor (the constructor style move onto the
// site), lays the hazard cones+tape over the footprint (LRG for a building pile,
// SML otherwise - NO slab; a slab is build only), and clears over clearLargeTurns
// turns for a building pile, clearSmallTurns for a small one. The markers are real
// stationary ground cover units, so they draw the site border and block other
// movers (units route around the site) until the job ends. C++:
// UnitsManager_Popup_OnClick_StopRemove -> ORDER_CLEAR -> UnitsManager_StartClearing
// (deploys SML/LRG CONES + TAPE, no slab).
func (g *Game) StartClearRubble(u *game.UnitInfo) bool {
	ax, ay, large, ok := g.rubbleCoverAnchor(u.GridX, u.GridY)
	if !IsBulldozer(u) || g.IsClearingRubble(u) || !ok {
		return false
	}
	// The dozer keeps its current cell (one of the footprint cells it drove to) and
	// glides from there onto the site centre - it must NOT snap to the anchor first,
	// or the drive in would always start from the top left corner.
	tape, cones := assets.SmlTape, assets.SmlCones
	flags := game.FlagStationary | game.FlagGroundCover
	turns, footprint := clearSmallTurns, 1
	if large {
		tape, cones = assets.LrgTape, assets.LrgCones
		flags |= game.FlagBuilding // 2x2 footprint centre anchor (like the large slab)
		turns, footprint = clearLargeTurns, 2
	}
	markers := []*game.UnitInfo{
		g.deployMarker(cones, u.Team, flags, ax, ay),
		g.deployMarker(tape, u.Team, flags, ax, ay),
	}
	// Rotate to face inward and glide onto the site centre (the gamescreen slide
	// loop advances X/Y); a 1x1 dozer can't sit on the middle CELL of a 2x2, so it
	// is drawn centred. C++: PositionInTape. The order is set below (OrderClear).
	u.StartClearPose(ax, ay, footprint)

	if g.clearing == nil {
		g.clearing = make(map[*game.UnitInfo]*clearJob)
	}
	g.clearing[u] = &clearJob{cellX: ax, cellY: ay, footprint: footprint, turnsLeft: turns, markers: markers}
	u.Order = game.OrderClear
	u.State = game.OrderStateExecutingOrder
	return true
}

// deployMarker spawns one clear site ground cover marker (a cones or tape unit)
// over the site. They render under the bulldozer and block other movers via the
// access default block for an unlisted stationary type.
func (g *Game) deployMarker(t assets.UnitType, team int, flags game.Flags, ax, ay int) *game.UnitInfo {
	g.nextBuilt++
	m := game.NewUnitInfo(t, team, 30000+g.nextBuilt, 0)
	m.Flags = flags
	g.Units.Deploy(m, ax, ay)
	return m
}

// removeClearMarkers takes a job's cones+tape units back off the map.
func (g *Game) removeClearMarkers(j *clearJob) {
	for _, m := range j.markers {
		g.Units.Remove(m)
	}
}

// IsClearingRubble reports whether u is busy clearing a pile.
func (g *Game) IsClearingRubble(u *game.UnitInfo) bool {
	_, ok := g.clearing[u]
	return ok
}

// ClearSite returns the rubble site a bulldozer is clearing (anchor cell +
// footprint) and whether it is clearing. The in game screen uses it for site aware
// selection: the dozer is drawn centred on the site, off its logical cell, so a
// click anywhere on the footprint must still resolve to it.
func (g *Game) ClearSite(u *game.UnitInfo) (x, y, footprint int, ok bool) {
	if j, found := g.clearing[u]; found {
		return j.cellX, j.cellY, j.footprint, true
	}
	return 0, 0, 0, false
}

// ClearTurnsLeft is how many more turns the bulldozer needs to finish the pile
// (0 if it is not clearing), for the status display.
func (g *Game) ClearTurnsLeft(u *game.UnitInfo) int {
	if j, ok := g.clearing[u]; ok {
		return j.turnsLeft
	}
	return 0
}

// StopClearRubble cancels a clear in progress and frees the bulldozer (the
// vanilla StopRemove toggle); the cones+tape come off and the rubble is left.
func (g *Game) StopClearRubble(u *game.UnitInfo) {
	j, ok := g.clearing[u]
	if !ok {
		return
	}
	g.removeClearMarkers(j)
	fp := j.footprint
	delete(g.clearing, u)
	u.BeginClearLeave(fp) // slide back out of the site instead of teleporting, then free
	u.Order = game.OrderAwait
	u.State = game.OrderStateExecutingOrder
}

// abortClear drops a clearing bulldozer's job and its markers without restoring an
// order (the unit is being destroyed). Called from DestroyUnit.
func (g *Game) abortClear(u *game.UnitInfo) {
	if j, ok := g.clearing[u]; ok {
		g.removeClearMarkers(j)
		delete(g.clearing, u)
		u.EndClearPose()
	}
}

// reclaimRubble adds salvaged raw to the bulldozer up to its ATTRIB_STORAGE capacity; excess material is lost.
func reclaimRubble(u *game.UnitInfo, material int) {
	if material <= 0 {
		return
	}
	capacity := 0
	if u.Values != nil {
		capacity = u.Values.Get(game.AttribStorage)
	}
	u.Storage += material
	if u.Storage > capacity {
		u.Storage = capacity
	}
}

// tickClearing advances every clear job for team by one turn: a busy bulldozer
// gets no movement budget this turn, and when its work reaches zero the pile (and
// its cones+tape) are removed and the bulldozer is freed. Called from BeginTeamTurn
// after tickDisabled so the bulldozer's budget has already been refilled. C++: the
// ORDER_CLEAR per turn progress + the rubble removal on completion.
func (g *Game) tickClearing(team int) {
	for u, j := range g.clearing {
		if u.Team != team {
			continue
		}
		j.turnsLeft--
		if j.turnsLeft > 0 {
			u.Speed = 0 // still occupied clearing this turn: no movement budget
			continue
		}
		// Finished this turn: salvage, remove the pile + markers, and free the dozer.
		// KEEP its turn start refilled movement budget (do NOT zero Speed) so it can
		// drive off the cleared site the same turn it is freed, like any finished
		// builder - zeroing it here left the dozer stranded with empty speed.
		key := [2]int{j.cellX, j.cellY}
		reclaimRubble(u, g.rubble[key].material) // salvage the pile's raw into the dozer's cargo
		delete(g.rubble, key)
		g.removeClearMarkers(j)
		fp := j.footprint
		delete(g.clearing, u)
		// Faithful exit (the original SpawnNewUnit for BULLDOZR): no exit markers - the
		// dozer reverts to its driving frame, slides back out of a 2x2 site, and
		// becomes a free AWAIT unit you can drive off normally.
		u.BeginClearLeave(fp)
		u.Order = game.OrderAwait
		u.State = game.OrderStateExecutingOrder
	}
}
